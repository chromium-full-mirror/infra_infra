// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package testutils

import (
	"infra/fleetconsole/api/fleetconsolerpc"
)

type DeviceBuilder struct {
	id         string
	dutID      string
	hostname   string
	port       int
	state      fleetconsolerpc.DeviceState
	deviceType fleetconsolerpc.DeviceType
	labels     map[string]*fleetconsolerpc.LabelValues
}

func NewDeviceBuilder() *DeviceBuilder {
	return &DeviceBuilder{
		id:         "id",
		dutID:      "dut_id",
		hostname:   "hostname",
		port:       1234,
		state:      fleetconsolerpc.DeviceState_DEVICE_STATE_AVAILABLE,
		deviceType: fleetconsolerpc.DeviceType_DEVICE_TYPE_PHYSICAL,
		labels:     map[string]*fleetconsolerpc.LabelValues{},
	}
}

func (b *DeviceBuilder) WithID(id string) *DeviceBuilder {
	b.id = id
	return b
}

func (b *DeviceBuilder) WithDutID(dutID string) *DeviceBuilder {
	b.dutID = dutID
	return b
}

func (b *DeviceBuilder) WithHostname(hostname string) *DeviceBuilder {
	b.hostname = hostname
	return b
}

func (b *DeviceBuilder) WithPort(port int) *DeviceBuilder {
	b.port = port
	return b
}

func (b *DeviceBuilder) WithState(state fleetconsolerpc.DeviceState) *DeviceBuilder {
	b.state = state
	return b
}

func (b *DeviceBuilder) WithType(deviceType fleetconsolerpc.DeviceType) *DeviceBuilder {
	b.deviceType = deviceType
	return b
}

func (b *DeviceBuilder) WithLabel(key string, values []string) *DeviceBuilder {
	b.labels[key] = &fleetconsolerpc.LabelValues{Values: values}
	return b
}

func (b *DeviceBuilder) Build() *fleetconsolerpc.Device {
	return &fleetconsolerpc.Device{
		Id:         b.id,
		DutId:      b.dutID,
		Address:    &fleetconsolerpc.DeviceAddress{Host: b.hostname, Port: int32(b.port)},
		Type:       b.deviceType,
		State:      b.state,
		DeviceSpec: &fleetconsolerpc.DeviceSpec{Labels: b.labels}}
}
