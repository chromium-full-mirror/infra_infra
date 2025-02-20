// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vmlab

import (
	"reflect"
	"testing"

	"go.chromium.org/infra/libs/vmlab/api"
	"go.chromium.org/infra/libs/vmlab/internal/image/cloudsdk"
)

func TestNewImageApi_unimplemented(t *testing.T) {
	imageApi, err := NewImageApi(api.ProviderId_UNKNOWN)
	if imageApi != nil {
		t.Errorf("ImageApi = %v, but want nil", imageApi)
	}
	if err == nil {
		t.Errorf("error should not be nil")
	}
}

func TestNewImageApi_cloudsdk(t *testing.T) {
	want, _ := cloudsdk.New()
	imageApi, err := NewImageApi(api.ProviderId_CLOUDSDK)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(imageApi, want) {
		t.Errorf("ImageApi = %v, but want %v", imageApi, want)
	}
}

func TestNewImageApi_gcloud(t *testing.T) {
	imageApi, err := NewImageApi(api.ProviderId_GCLOUD)
	if imageApi != nil {
		t.Errorf("ImageApi = %v, but want nil", imageApi)
	}
	if err == nil {
		t.Errorf("error should not be nil")
	}
}

func TestNewImageApi_VmLeaser(t *testing.T) {
	imageApi, err := NewImageApi(api.ProviderId_VM_LEASER)
	if imageApi != nil {
		t.Errorf("ImageApi = %v, but want nil", imageApi)
	}
	if err == nil {
		t.Errorf("error should not be nil")
	}
}
