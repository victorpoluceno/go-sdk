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

// Package event //
package event

import (
	"errors"
	"testing"
	"time"

	"github.com/optimizely/go-sdk/pkg/utils"

	"github.com/stretchr/testify/assert"
)

func TestDefaultEventProcessor_ProcessImpression(t *testing.T) {
	exeCtx := utils.NewCancelableExecutionCtx()
	processor := NewEventProcessor(FlushInterval(100))
	processor.Start(exeCtx)

	impression := BuildTestImpressionEvent()

	processor.ProcessEvent(impression)

	assert.Equal(t, 1, processor.EventsCount())

	exeCtx.TerminateAndWait()

	assert.NotNil(t, processor.Ticker)

	assert.Equal(t, 0, processor.EventsCount())
}

func TestCustomEventProcessor_Create(t *testing.T) {
	exeCtx := utils.NewCancelableExecutionCtx()
	processor := NewEventProcessor(QueueSize(10), FlushInterval(100))
	processor.Start(exeCtx)

	impression := BuildTestImpressionEvent()

	processor.ProcessEvent(impression)

	assert.Equal(t, 1, processor.EventsCount())

	exeCtx.TerminateAndWait()

	assert.NotNil(t, processor.Ticker)

	assert.Equal(t, 0, processor.EventsCount())
}

type MockDispatcher struct {
	ShouldFail bool
	Events     Queue
}

func (f *MockDispatcher) DispatchEvent(event LogEvent) (bool, error) {
	if f.ShouldFail {
		return false, errors.New("Failed to dispatch")
	}

	f.Events.Add(event)
	return true, nil
}

func TestDefaultEventProcessor_ProcessBatch(t *testing.T) {
	exeCtx := utils.NewCancelableExecutionCtx()
	processor := NewEventProcessor(FlushInterval(100), QueueSize(100),
		PQ(NewInMemoryQueue(100)), PDispatcher(&MockDispatcher{Events: NewInMemoryQueue(100)}))
	processor.Start(exeCtx)

	impression := BuildTestImpressionEvent()
	conversion := BuildTestConversionEvent()

	processor.ProcessEvent(impression)
	processor.ProcessEvent(impression)
	processor.ProcessEvent(conversion)
	processor.ProcessEvent(conversion)

	assert.Equal(t, 4, processor.EventsCount())

	exeCtx.TerminateAndWait()

	assert.NotNil(t, processor.Ticker)

	assert.Equal(t, 0, processor.EventsCount())

	result, ok := (processor.EventDispatcher).(*MockDispatcher)

	if ok {
		assert.Equal(t, 1, result.Events.Size())
		evs := result.Events.Get(1)
		logEvent, _ := evs[0].(LogEvent)
		assert.Equal(t, 4, len(logEvent.Event.Visitors))
	}
}

func TestDefaultEventProcessor_QSizeMet(t *testing.T) {
	exeCtx := utils.NewCancelableExecutionCtx()
	processor := NewEventProcessor(QueueSize(2), FlushInterval(100),
		PQ(NewInMemoryQueue(2)), PDispatcher(&MockDispatcher{Events: NewInMemoryQueue(100)}))
	processor.Start(exeCtx)

	impression := BuildTestImpressionEvent()
	conversion := BuildTestConversionEvent()

	processor.ProcessEvent(impression)
	processor.ProcessEvent(impression)

	assert.Equal(t, 2, processor.EventsCount())

	time.Sleep(100 * time.Millisecond)

	result, ok := (processor.EventDispatcher).(*MockDispatcher)

	if ok {
		assert.Equal(t, 1, result.Events.Size())
		evs := result.Events.Get(1)
		logEvent, _ := evs[0].(LogEvent)
		assert.Equal(t, 2, len(logEvent.Event.Visitors))

	}

	processor.ProcessEvent(conversion)
	processor.ProcessEvent(conversion)

	assert.Equal(t, 2, processor.EventsCount())

	exeCtx.TerminateAndWait()

	assert.NotNil(t, processor.Ticker)

	assert.Equal(t, 0, processor.EventsCount())

	if ok {
		assert.Equal(t, 2, result.Events.Size())
	}
}

func TestDefaultEventProcessor_FailedDispatch(t *testing.T) {
	exeCtx := utils.NewCancelableExecutionCtx()
	processor := NewEventProcessor(QueueSize(100), FlushInterval(100),
		PQ(NewInMemoryQueue(100)), PDispatcher(&MockDispatcher{ShouldFail: true, Events: NewInMemoryQueue(100)}))
	processor.Start(exeCtx)

	impression := BuildTestImpressionEvent()
	conversion := BuildTestConversionEvent()

	processor.ProcessEvent(impression)
	processor.ProcessEvent(impression)
	processor.ProcessEvent(conversion)
	processor.ProcessEvent(conversion)

	assert.Equal(t, 4, processor.EventsCount())

	exeCtx.TerminateAndWait()

	assert.NotNil(t, processor.Ticker)

	assert.Equal(t, 4, processor.EventsCount())

	result, ok := (processor.EventDispatcher).(*MockDispatcher)

	if ok {
		assert.Equal(t, 0, result.Events.Size())
	}
}

func TestBatchEventProcessor_FlushesOnClose(t *testing.T) {
	exeCtx := utils.NewCancelableExecutionCtx()
	processor := NewEventProcessor(QueueSize(100), PQ(NewInMemoryQueue(100)),
		PDispatcher(&MockDispatcher{Events: NewInMemoryQueue(100)}))
	processor.Start(exeCtx)

	impression := BuildTestImpressionEvent()
	conversion := BuildTestConversionEvent()

	processor.ProcessEvent(impression)
	processor.ProcessEvent(impression)
	processor.ProcessEvent(conversion)
	processor.ProcessEvent(conversion)

	assert.Equal(t, 4, processor.EventsCount())

	// Triggers the flush in the processor
	exeCtx.TerminateAndWait()

	assert.Equal(t, 0, processor.EventsCount())
}

func TestDefaultEventProcessor_ProcessBatchRevisionMismatch(t *testing.T) {
	exeCtx := utils.NewCancelableExecutionCtx()
	processor := NewEventProcessor(QueueSize(100), FlushInterval(100),
		PQ(NewInMemoryQueue(100)), PDispatcher(&MockDispatcher{Events: NewInMemoryQueue(100)}))

	processor.Start(exeCtx)

	impression := BuildTestImpressionEvent()
	conversion := BuildTestConversionEvent()

	processor.ProcessEvent(impression)
	impression.EventContext.Revision = "12112121"
	processor.ProcessEvent(impression)
	processor.ProcessEvent(conversion)
	processor.ProcessEvent(conversion)

	assert.Equal(t, 4, processor.EventsCount())

	exeCtx.TerminateAndWait()

	assert.NotNil(t, processor.Ticker)

	assert.Equal(t, 0, processor.EventsCount())

	result, ok := (processor.EventDispatcher).(*MockDispatcher)

	if ok {
		assert.Equal(t, 3, result.Events.Size())
		evs := result.Events.Get(3)
		logEvent, _ := evs[len(evs)-1].(LogEvent)
		assert.Equal(t, 2, len(logEvent.Event.Visitors))
	}
}

func TestDefaultEventProcessor_ProcessBatchProjectMismatch(t *testing.T) {
	exeCtx := utils.NewCancelableExecutionCtx()
	processor := NewEventProcessor(QueueSize(100), FlushInterval(100),
		PQ(NewInMemoryQueue(100)), PDispatcher(&MockDispatcher{Events: NewInMemoryQueue(100)}))

	processor.Start(exeCtx)

	impression := BuildTestImpressionEvent()
	conversion := BuildTestConversionEvent()

	processor.ProcessEvent(impression)
	impression.EventContext.ProjectID = "121121211111"
	processor.ProcessEvent(impression)
	processor.ProcessEvent(conversion)
	processor.ProcessEvent(conversion)

	assert.Equal(t, 4, processor.EventsCount())

	exeCtx.TerminateAndWait()

	assert.NotNil(t, processor.Ticker)

	assert.Equal(t, 0, processor.EventsCount())

	result, ok := (processor.EventDispatcher).(*MockDispatcher)

	if ok {
		assert.Equal(t, 3, result.Events.Size())
		evs := result.Events.Get(3)
		logEvent, _ := evs[len(evs)-1].(LogEvent)
		assert.Equal(t, 2, len(logEvent.Event.Visitors))
	}
}

func TestChanQueueEventProcessor_ProcessImpression(t *testing.T) {
	exeCtx := utils.NewCancelableExecutionCtx()
	processor := NewEventProcessor(QueueSize(100), FlushInterval(100),
		PQ(NewInMemoryQueue(100)), PDispatcher(&HTTPEventDispatcher{}))

	processor.Start(exeCtx)

	impression := BuildTestImpressionEvent()

	processor.ProcessEvent(impression)
	processor.ProcessEvent(impression)
	processor.ProcessEvent(impression)

	exeCtx.TerminateAndWait()
	assert.NotNil(t, processor.Ticker)

	assert.Equal(t, 0, processor.EventsCount())
}

func TestChanQueueEventProcessor_ProcessBatch(t *testing.T) {
	exeCtx := utils.NewCancelableExecutionCtx()
	processor := NewEventProcessor(QueueSize(100), FlushInterval(100),
		PQ(NewInMemoryQueue(100)), PDispatcher(&MockDispatcher{Events: NewInMemoryQueue(100)}))
	processor.Start(exeCtx)

	impression := BuildTestImpressionEvent()
	conversion := BuildTestConversionEvent()

	processor.ProcessEvent(impression)
	processor.ProcessEvent(impression)
	processor.ProcessEvent(conversion)
	processor.ProcessEvent(conversion)

	exeCtx.TerminateAndWait()

	assert.NotNil(t, processor.Ticker)

	assert.Equal(t, 0, processor.EventsCount())

	result, ok := (processor.EventDispatcher).(*MockDispatcher)

	if ok {
		assert.Equal(t, 1, result.Events.Size())
		evs := result.Events.Get(1)
		logEvent, _ := evs[0].(LogEvent)
		assert.True(t, len(logEvent.Event.Visitors) >= 1)
	}
}