/****************************************************************************
 * Copyright 2019, Optimizely, Inc. and contributors                        *
 *                                                                          *
 * Licensed under the Apache License, Version 2.0 (the "License");          *
 * you may not use this file except in compliance with the License.         *
 * You may obtain a copy of the License at                                  *
 *                                                                          *
 *    http://www.apache.org/licenses/LICENSE-2.0                            *
 *                                                                          *
 * Unless required by applicable law or agreed to in writing, software      *
 * distributed under the License is distributed on an "AS IS" BASIS,        *
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. *
 * See the License for the specific language governing permissions and      *
 * limitations under the License.                                           *
 ***************************************************************************/

package optlyplugins

import (
	"sync"
	"time"

	"github.com/optimizely/go-sdk/pkg/client"
	"github.com/optimizely/go-sdk/pkg/config"
	"github.com/optimizely/go-sdk/pkg/decision"
	"github.com/optimizely/go-sdk/pkg/entities"
	"github.com/optimizely/go-sdk/pkg/event"
	"github.com/optimizely/go-sdk/pkg/notification"
	"github.com/optimizely/go-sdk/pkg/registry"
	"github.com/optimizely/go-sdk/tests/integration/models"
)

// DefaultInitializationTimeout defines default timeout for datafile sync
const DefaultInitializationTimeout = time.Duration(3000) * time.Millisecond

// NotificationManager manager class for notification listeners
type NotificationManager struct {
	listenersCalled                    []interface{}
	projectConfigUpdateListenersCalled []notification.ProjectConfigUpdateNotification
	configManager                      config.ProjectConfigManager
}

// SubscribeNotifications subscribes to the provided notification listeners
func (n *NotificationManager) SubscribeNotifications(listeners map[string]int, client *client.OptimizelyClient) {

	addNotificationCallback := func(notificationType string) {
		switch notificationType {
		case models.KeyDecision:
			client.DecisionService.OnDecision(n.decisionCallback)
			break
		case models.KeyTrack:
			client.OnTrack(n.trackCallback)
			break
		}
	}

	for key, count := range listeners {
		for i := 0; i < count; i++ {
			addNotificationCallback(key)
		}
	}
}

// SubscribeProjectConfigUpdateNotifications subscribes to the project config notification listeners
func (n *NotificationManager) SubscribeProjectConfigUpdateNotifications(sdkKey string, listeners map[string]int) {
	if count, ok := listeners[models.KeyConfigUpdate]; ok {
		for i := 0; i < count; i++ {
			registry.GetNotificationCenter(sdkKey).AddHandler(notification.ProjectConfigUpdate, n.projectConfigUpdateCallback)
		}
	}
}

// GetListenersCalled - Returns listeners called
func (n *NotificationManager) GetListenersCalled() []interface{} {
	listenerCalled := n.listenersCalled
	// Since for every scenario, a new sdk instance is created, emptying listenersCalled is required for scenario's
	// where multiple requests are executed but no session is to be maintained among them.
	// @TODO: Make it optional once event-batching(sessioned) tests are implemented.
	n.listenersCalled = nil
	return listenerCalled
}

// GetProjectConfigUpdateListenersCalled - Returns ProjectConfigUpdate listeners called
func (n *NotificationManager) GetProjectConfigUpdateListenersCalled() []notification.ProjectConfigUpdateNotification {
	projectConfigUpdateListenersCalled := n.projectConfigUpdateListenersCalled
	// Since for every scenario, a new sdk instance is created, emptying listenersCalled is required for scenario's
	// where multiple requests are executed but no session is to be maintained among them.
	// @TODO: Make it optional once event-batching(sessioned) tests are implemented.
	n.projectConfigUpdateListenersCalled = nil
	return projectConfigUpdateListenersCalled
}

// TestDFMConfiguration - Executes DFM configuration tests
func (n *NotificationManager) TestDFMConfiguration(configuration models.DataFileManagerConfiguration) {
	timeout := DefaultInitializationTimeout
	if configuration.Timeout != nil {
		timeout = time.Duration(*(configuration.Timeout)) * time.Millisecond
	}

	verify := func(wg *sync.WaitGroup) {
		start := time.Now()
		switch configuration.Mode {
		case models.KeyWaitForOnReady:
			for {
				t := time.Now()
				elapsed := t.Sub(start)
				if elapsed >= timeout {
					break
				}
				// Check if projectconfig is ready
				_, err := n.configManager.GetConfig()
				if err == nil {
					break
				}
			}
			break
		case models.KeyWaitForConfigUpdate:
			revision := 0
			if configuration.Revision != nil {
				revision = *(configuration.Revision)
			}
			for {
				t := time.Now()
				elapsed := t.Sub(start)
				if elapsed >= timeout {
					break
				}
				if revision > 0 {
					// This means we want the manager to poll until we get to a specific revision
					if revision == len(n.projectConfigUpdateListenersCalled) {
						break
					}
				} else if len(n.projectConfigUpdateListenersCalled) == 1 {
					// For cases where we are just waiting for config listener
					break
				}
			}
			break
		default:
			break
		}
		wg.Done()
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go verify(&wg)
	wg.Wait()
}

func (n *NotificationManager) decisionCallback(notification notification.DecisionNotification) {

	model := models.DecisionListener{}
	model.Type = notification.Type
	model.UserID = notification.UserContext.ID
	if notification.UserContext.Attributes == nil {
		model.Attributes = make(map[string]interface{})
	} else {
		model.Attributes = notification.UserContext.Attributes
	}

	decisionInfoDict := getDecisionInfoForNotification(notification)
	model.DecisionInfo = decisionInfoDict
	n.listenersCalled = append(n.listenersCalled, model)
}

func (n *NotificationManager) trackCallback(eventKey string, userContext entities.UserContext, eventTags map[string]interface{}, conversionEvent event.ConversionEvent) {
	listener := models.TrackListener{
		EventKey:   eventKey,
		UserID:     userContext.ID,
		Attributes: userContext.Attributes,
		EventTags:  eventTags,
	}
	n.listenersCalled = append(n.listenersCalled, listener)
}

func (n *NotificationManager) projectConfigUpdateCallback(payload interface{}) {
	if notification, ok := payload.(notification.ProjectConfigUpdateNotification); ok {
		n.projectConfigUpdateListenersCalled = append(n.projectConfigUpdateListenersCalled, notification)
	}
}

func getDecisionInfoForNotification(decisionNotification notification.DecisionNotification) map[string]interface{} {
	decisionInfoDict := make(map[string]interface{})

	updateSourceInfo := func(source string) {
		decisionInfoDict["source_info"] = make(map[string]interface{})
		if source == string(decision.FeatureTest) {
			featureInfoDict := decisionNotification.DecisionInfo["feature"].(map[string]interface{})
			if sourceInfo, ok := featureInfoDict["sourceInfo"].(interface{}); ok {
				sourceInfoDict := sourceInfo.((map[string]string))
				if experimentKey, ok := sourceInfoDict["experimentKey"]; ok {
					if variationKey, ok := sourceInfoDict["variationKey"]; ok {
						dict := make(map[string]interface{})
						dict["experiment_key"] = experimentKey
						dict["variation_key"] = variationKey
						decisionInfoDict["source_info"] = dict
					}
				}
			}
		}
	}

	switch notificationType := decisionNotification.Type; notificationType {
	case notification.ABTest, notification.FeatureTest:
		decisionInfoDict["experiment_key"] = decisionNotification.DecisionInfo["experimentKey"]
		decisionInfoDict["variation_key"] = decisionNotification.DecisionInfo["variationKey"]
		break
	case notification.Feature:
		featureInfoDict := decisionNotification.DecisionInfo["feature"].(map[string]interface{})
		source := ""
		if decisionSource, ok := featureInfoDict["source"].(decision.Source); ok {
			source = string(decisionSource)
		} else {
			source = featureInfoDict["source"].(string)
		}
		decisionInfoDict["source"] = source
		decisionInfoDict["feature_enabled"] = featureInfoDict["featureEnabled"]
		decisionInfoDict["feature_key"] = featureInfoDict["featureKey"]
		updateSourceInfo(source)
	case notification.FeatureVariable:
		featureInfoDict := decisionNotification.DecisionInfo["feature"].(map[string]interface{})
		source := ""
		if decisionSource, ok := featureInfoDict["source"].(decision.Source); ok {
			source = string(decisionSource)
		} else {
			source = featureInfoDict["source"].(string)
		}
		decisionInfoDict["source"] = source
		decisionInfoDict["variable_key"] = featureInfoDict["variableKey"]
		if variableType, ok := featureInfoDict["variableType"].(entities.VariableType); ok {
			decisionInfoDict["variable_type"] = string(variableType)
		} else {
			decisionInfoDict["variable_type"] = featureInfoDict["variableType"].(string)
		}
		decisionInfoDict["variable_value"] = featureInfoDict["variableValue"]
		decisionInfoDict["feature_enabled"] = featureInfoDict["featureEnabled"]
		decisionInfoDict["feature_key"] = featureInfoDict["featureKey"]
		updateSourceInfo(source)
	default:
	}
	return decisionInfoDict
}
