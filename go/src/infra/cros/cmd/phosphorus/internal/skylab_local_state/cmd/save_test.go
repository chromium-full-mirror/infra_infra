// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cmd

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"go.chromium.org/chromiumos/infra/proto/go/lab_platform"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_local_state"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestNewDutStateFromHostInfo(t *testing.T) {
	ftt.Run("When a DUT state is updated only provisionable labels and attributes are changed.", t, func(t *ftt.Test) {
		i := &skylab_local_state.AutotestHostInfo{
			Attributes: map[string]string{
				"dummy-attribute": "dummy-value",
				"job_repo_url":    "dummy-url",
				"outlet_changed":  "true",
			},
			Labels: []string{
				"dummy-label:dummy-value",
				"cros-version:dummy-os-version",
			},
			SerializerVersion: 1,
		}

		state := updateDutStateFromHostInfo(&lab_platform.DutState{}, i)

		want := &lab_platform.DutState{
			ProvisionableAttributes: map[string]string{
				"job_repo_url":   "dummy-url",
				"outlet_changed": "true",
			},
		}

		assert.Loosely(t, want, should.Resemble(state))
	})
}

// TestValidateSaveRequest verifies the validation and default val logic
func TestValidateSaveRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     *skylab_local_state.SaveRequest
		wantReq *skylab_local_state.SaveRequest
		wantErr bool
	}{
		{
			name:    "valid",
			req:     validSaveRequest,
			wantReq: validSaveRequest,
			wantErr: false,
		},
		{
			name:    "invalid",
			req:     invalidSaveRequest,
			wantReq: invalidSaveRequest,
			wantErr: true,
		},
		{
			name:    "add default",
			req:     validSaveRequestNoNamespace,
			wantReq: validSaveRequestDefaultNamespace,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateSaveRequest(tt.req); (err != nil) != tt.wantErr {
				t.Errorf("validateSaveRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.req, tt.wantReq, cmpopts.IgnoreUnexported(skylab_local_state.SaveRequest{}, skylab_local_state.Config{})); diff != "" {
				t.Errorf("unexpected diff: %s", diff)
			}
		})
	}
}

var (
	// validSaveRequest is a completely valid request
	validSaveRequest = &skylab_local_state.SaveRequest{
		Config: &skylab_local_state.Config{
			AutotestDir:  "dir",
			UfsNamespace: "namespace",
		},
		ResultsDir: "dir",
		DutName:    "name",
		DutId:      "id",
		DutState:   "state",
	}

	// validSaveRequestNoNamespace is a request that is valid but will get a
	// Config.UfsNamespace added to it
	validSaveRequestNoNamespace = &skylab_local_state.SaveRequest{
		Config: &skylab_local_state.Config{
			AutotestDir: "dir",
		},
		ResultsDir: "dir",
		DutName:    "name",
		DutId:      "id",
		DutState:   "state",
	}

	// validSaveRequestDefaultNamespace is the expected result of the above
	// NoNamespace getting the default added
	validSaveRequestDefaultNamespace = &skylab_local_state.SaveRequest{
		Config: &skylab_local_state.Config{
			AutotestDir:  "dir",
			UfsNamespace: "os",
		},
		ResultsDir: "dir",
		DutName:    "name",
		DutId:      "id",
		DutState:   "state",
	}

	// invalidSaveRequest should be rejected
	invalidSaveRequest = &skylab_local_state.SaveRequest{}
)
