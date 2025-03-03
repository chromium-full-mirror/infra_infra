// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package cpu_temperature

import "go.chromium.org/infra/cros/satlab/satlabrpcserver/utils/sized_queue"

// CPUTemperatureOrchestrator keep the history of cpu temperature queue and
// then use the queue to calculate the average temperature.
type CPUTemperatureOrchestrator struct {
	queue          sized_queue.SizedQueue[float32]
	cpuTemperature ICPUTemperature
}

// NewOrchestrator create a new CPUTemperatureOrchestrator
func NewOrchestrator(temperature ICPUTemperature, capacity int) *CPUTemperatureOrchestrator {
	return &CPUTemperatureOrchestrator{
		queue:          sized_queue.New[float32](capacity),
		cpuTemperature: temperature,
	}
}

// GetAverageCPUTemperature get the average temperature according the temperature queue.
func (c *CPUTemperatureOrchestrator) GetAverageCPUTemperature() float32 {
	var avg float32 = 0
	size := c.queue.Size()
	var rawData = c.queue.Data()

	for i := range size {
		avg += rawData[i]
	}

	if size == 0 {
		return 0
	}

	return avg / float32(size)
}

// Observe implement the `Observable` interface. Let the `Monitor` to
// observe the cpu temperature.
func (c *CPUTemperatureOrchestrator) Observe() {
	temp, err := c.cpuTemperature.GetCurrentCPUTemperature()
	if err != nil {
		return
	}
	c.queue.Push(temp)
}
