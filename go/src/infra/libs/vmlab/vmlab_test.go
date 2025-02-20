// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vmlab

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"go.chromium.org/infra/libs/vmlab/api"
	"go.chromium.org/infra/libs/vmlab/internal/instance/gcloud"
	vmleaser "go.chromium.org/infra/libs/vmlab/internal/instance/vm_leaser"
)

func TestNewInstanceApi_unimplemented(t *testing.T) {
	ins, err := NewInstanceApi(api.ProviderId_UNKNOWN)
	if ins != nil {
		t.Errorf("InstanceApi = %v, but want nil", ins)
	}
	if err == nil {
		t.Errorf("error should not be nil")
	}
}

func TestNewInstanceApi_cloudsdk(t *testing.T) {
	ins, err := NewInstanceApi(api.ProviderId_CLOUDSDK)
	if ins != nil {
		t.Errorf("InstanceApi = %v, but want nil", ins)
	}
	if err == nil {
		t.Errorf("error should not be nil")
	}
}

func TestNewInstanceApi_gcloud(t *testing.T) {
	want, _ := gcloud.New()
	ins, err := NewInstanceApi(api.ProviderId_GCLOUD)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cmp.Equal(ins, want) {
		t.Errorf("InstanceApi = %v, but want %v", ins, want)
	}
}

func TestNewInstanceApi_vmLeaser(t *testing.T) {
	want, _ := vmleaser.New()
	ins, err := NewInstanceApi(api.ProviderId_VM_LEASER)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cmp.Equal(ins, want) {
		t.Errorf("InstanceApi = %v, but want %v", ins, want)
	}
}
