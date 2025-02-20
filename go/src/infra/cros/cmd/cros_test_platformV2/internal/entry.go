// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package internal

import (
	"context"
	"fmt"

	"go.chromium.org/chromiumos/config/go/test/api"

	parsers "go.chromium.org/infra/cros/cmd/cros_test_platformV2/tools"
)

func Execute(inputPath string, cloud bool) (*api.CTPv2Response, error) {
	request, err := parsers.ReadInput(inputPath)
	if err != nil {
		fmt.Printf("Unable to parse: %s", err)
		// return nil, fmt.Errorf("unable to parse request: %s", err)
	}
	ctx := context.Background()

	// Build Exectors. Right now do CTRv2 + Both filters.
	// Note: the filter impls are currently nil.
	executors, _ := buildExecutors(ctx, request, cloud)

	InternalStruct := translateRequest(request)

	// Run the same commands for each
	for _, executor := range executors {
		// For CTR, init = start Server async. For services it will be pull/prep container/launch
		newIS, err := executor.Execute(ctx, "init", InternalStruct)
		if err != nil {
			fmt.Printf("filter err: %s\n", err)
		}
		err = validateStruct(InternalStruct, newIS)
		if err == nil {
			InternalStruct = newIS
		}
		if err != nil {
			fmt.Printf("validator err err: %s", err)
		}
	}
	// Run the same commands for each
	for _, executor := range executors {
		// Gcloud auth for CTR (kinda odd...). For services, it will be `call the service.`
		newIS, err := executor.Execute(ctx, "run", InternalStruct)
		if err != nil {
			fmt.Printf("filter err: %s\n", err)
		}
		err = validateStruct(InternalStruct, newIS)
		if err == nil {
			InternalStruct = newIS
		}
		if err != nil {
			fmt.Printf("validator err err: %s", err)
		}
	}

	// After all execs are run, stop them all. TODO, this probably needs to be a bit smarter defered.
	for _, executor := range executors {
		newIS, err := executor.Execute(ctx, "stop", InternalStruct)
		if err != nil {
			fmt.Printf("filter err: %s", err)
		}
		err = validateStruct(InternalStruct, newIS)
		if err == nil {
			InternalStruct = newIS
		}
		if err != nil {
			fmt.Printf("validator err err: %s", err)
		}
	}

	return kompress(InternalStruct)
}

func kompress(resp *api.InternalTestplan) (*api.CTPv2Response, error) {
	return nil, nil
}

func validateStruct(resp *api.InternalTestplan, newResp *api.InternalTestplan) error {
	return fmt.Errorf("no change")
}
