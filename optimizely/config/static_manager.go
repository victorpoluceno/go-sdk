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

package config

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"sync"

	"github.com/optimizely/go-sdk/optimizely"
	"github.com/optimizely/go-sdk/optimizely/config/datafileProjectConfig"
)

// StaticProjectConfigManager maintains a static copy of the project config
type StaticProjectConfigManager struct {
	projectConfig optimizely.ProjectConfig
	configLock    sync.Mutex
}

func NewStaticProjectConfigManagerFromUrl(URL string) (*StaticProjectConfigManager, error) {
	downloadFile := func(URL string) ([]byte, error) {
		response, err := http.Get(URL)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return nil, errors.New(response.Status)
		}
		var data bytes.Buffer
		_, err = io.Copy(&data, response.Body)
		if err != nil {
			return nil, err
		}
		return data.Bytes(), nil
	}

	body, err := downloadFile(URL)

	if err != nil {
		return nil, err
	}

	return NewStaticProjectConfigManagerFromPayload(body)
}

func NewStaticProjectConfigManagerFromPayload(payload []byte) (*StaticProjectConfigManager, error) {
	projectConfig, err := datafileProjectConfig.NewDatafileProjectConfig(payload)

	if err != nil {
		return nil, err
	}

	return NewStaticProjectConfigManager(projectConfig), nil
}

// NewStaticProjectConfigManager creates a new instance of the manager with the given project config
func NewStaticProjectConfigManager(config optimizely.ProjectConfig) *StaticProjectConfigManager {
	return &StaticProjectConfigManager{
		projectConfig: config,
	}
}

// GetConfig returns the project config
func (cm *StaticProjectConfigManager) GetConfig() optimizely.ProjectConfig {
	cm.configLock.Lock()
	defer cm.configLock.Unlock()
	return cm.projectConfig
}