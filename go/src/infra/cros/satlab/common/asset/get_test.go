// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package asset

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"go.chromium.org/infra/cros/satlab/common/utils/executor"
	ufsModels "go.chromium.org/infra/unifiedfleet/api/v1/models"
)

func TestGetDUTShouldWork(t *testing.T) {
	t.Parallel()

	// Create a request
	p := GetAsset{}
	ctx := context.Background()

	// Create a fake data
	expected := []*ufsModels.Asset{
		{
			Name:  "name",
			Type:  ufsModels.AssetType_DUT,
			Model: "atlas",
			Info: &ufsModels.AssetInfo{
				Model:       "atlas",
				BuildTarget: "atlas",
			},
		},
		{
			Name:  "name 2",
			Type:  ufsModels.AssetType_DUT,
			Model: "atlas 2",
			Info: &ufsModels.AssetInfo{
				Model:       "atlas 2",
				BuildTarget: "atlas 2",
			},
		},
	}
	m := jsonpb.Marshaler{}
	s := []string{}
	for _, elem := range expected {
		js, _ := m.MarshalToString(elem)
		s = append(s, js)
	}
	in := fmt.Sprintf("[%v]", strings.Join(s, ","))

	executor := executor.FakeCommander{CmdOutput: in}

	// Act
	res, err := p.TriggerRun(ctx, &executor)

	// Asset
	if err != nil {
		t.Errorf("Should be success, but got an error: %v", err)
		return
	}

	// ignore pb fields in `Asset`, and `AssetInfo`
	ignorePBFieldOpts := cmpopts.IgnoreUnexported(ufsModels.Asset{}, ufsModels.AssetInfo{})

	if diff := cmp.Diff(res, expected, ignorePBFieldOpts); diff != "" {
		t.Errorf("Expected: %v\n, got: %v\n, diff: %v\n", expected, res, diff)
		return
	}
}

func TestGetDUTShouldFail(t *testing.T) {
	t.Parallel()

	// Create a request
	p := GetAsset{}
	ctx := context.Background()

	// Create a fake data
	executor := executor.FakeCommander{Err: errors.New("cmd error")}

	// Act
	res, err := p.TriggerRun(ctx, &executor)

	// Asset
	if err == nil {
		t.Errorf("should be failed")
		return
	}

	if res != nil {
		t.Errorf("result should be empty, but got: %v", res)
		return
	}
}
