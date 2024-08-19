// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package api

import (
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func checkDuplicatedString() (input chan string, result chan bool) {
	input = make(chan string)
	result = make(chan bool)
	set := map[string]bool{}
	go func() {
		defer close(result)

		for i := range input {
			_, existing := set[i]
			set[i] = true
			result <- existing
		}
	}()
	return
}

// Validate validates getting requests must be non-empty and return error if
// it's not.
func (r *GetCrosDevicesRequest) Validate() error {
	if len(r.GetIds()) == 0 && len(r.GetModels()) == 0 {
		return status.Errorf(codes.InvalidArgument, "must specify device ID(s) to get")
	}
	return nil
}

// Validate validates input requests and return error if it's not.
func (r *UpdateDutsStatusRequest) Validate() error {
	if r.States == nil || len(r.States) == 0 {
		return status.Errorf(codes.InvalidArgument, "no devices to update")
	}
	// There must be no dupicated ID in the request.
	idChecker, duplicatedID := checkDuplicatedString()
	defer close(idChecker)

	idWithStates := make(map[string]bool, len(r.States))
	for _, d := range r.States {
		id := d.GetId().GetValue()
		idWithStates[id] = true
		if idChecker <- id; <-duplicatedID {
			return status.Errorf(codes.InvalidArgument, fmt.Sprintf("Duplicated id found: %s", id))
		}
	}

	idChecker2, duplicatedID2 := checkDuplicatedString()
	defer close(idChecker2)
	for _, d := range r.GetDutMetas() {
		id := d.GetChromeosDeviceId()
		if idChecker2 <- id; <-duplicatedID2 {
			return status.Errorf(codes.InvalidArgument, fmt.Sprintf("Duplicated id found in meta : %s", id))
		}
	}

	for _, d := range r.GetDutMetas() {
		id := d.GetChromeosDeviceId()
		if _, ok := idWithStates[id]; !ok {
			return status.Errorf(codes.InvalidArgument, fmt.Sprintf("Cannot update meta without valid dut states: %s", id))
		}
	}
	return nil
}

// Validate validates input requests of GetManufacturingConfig.
func (r *GetManufacturingConfigRequest) Validate() error {
	if r == nil || r.Name == "" {
		return status.Errorf(codes.InvalidArgument, "Request is empty")
	}
	return nil
}

// Validate validates input requests of GetDeviceConfig.
func (r *GetDeviceConfigRequest) Validate() error {
	if r == nil || r.GetConfigId() == nil {
		return status.Errorf(codes.InvalidArgument, "Request is empty")
	}
	return nil
}

// Validate validates input requests of GetHwidData.
func (r *GetHwidDataRequest) Validate() error {
	if r == nil || r.Name == "" {
		return status.Errorf(codes.InvalidArgument, "Request is empty")
	}
	return nil
}
