// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/vm_leaser/client"
)

func TestAbandonVMsWithName(t *testing.T) {
	vms := []*api.VM{
		{
			Id: "vm-test",
		},
		{
			Id: "vm-test2",
		},
	}
	expected := []*api.VM{
		{
			Id: "vm-test",
		},
	}

	vmLeaser := client.Client{
		VMLeaserClient: mockVMLeaserClient{
			releaseVMFunc: func(rv *api.ReleaseVMRequest) (*api.ReleaseVMResponse, error) {
				return nil, nil
			},
		},
		Email: testEmail,
	}

	ctx := context.Background()
	actual, err := abandonVMs(ctx, &vmLeaser, vms, "vm-test")

	if err != nil {
		t.Errorf("Expected nil error, get %v", err)
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Expected result to be %v, but is %v", expected, actual)
	}
}

func TestAbandonVMsNoName(t *testing.T) {
	vms := []*api.VM{
		{
			Id: "vm-test",
		},
		{
			Id: "vm-test2",
		},
	}

	vmLeaser := client.Client{
		VMLeaserClient: mockVMLeaserClient{
			releaseVMFunc: func(rv *api.ReleaseVMRequest) (*api.ReleaseVMResponse, error) {
				return nil, nil
			},
		},
		Email: testEmail,
	}

	ctx := context.Background()
	actual, err := abandonVMs(ctx, &vmLeaser, vms, "")

	if err != nil {
		t.Errorf("Expected nil error, get %v", err)
	}

	if !reflect.DeepEqual(actual, vms) {
		t.Errorf("Expected result to be %v, but is %v", vms, actual)
	}
}

func TestAbandonVMsError(t *testing.T) {
	vms := []*api.VM{
		{
			Id: "vm-test",
		},
		{
			Id: "vm-test2",
		},
	}

	vmLeaser := client.Client{
		VMLeaserClient: mockVMLeaserClient{
			releaseVMFunc: func(rv *api.ReleaseVMRequest) (*api.ReleaseVMResponse, error) {
				return nil, errors.New("error")
			},
		},
		Email: testEmail,
	}

	ctx := context.Background()
	actual, err := abandonVMs(ctx, &vmLeaser, vms, "")

	if err == nil {
		t.Error("Expected error, get nil")
	}

	if actual != nil {
		t.Errorf("Expected result to be nil, but is %v", actual)
	}
}
