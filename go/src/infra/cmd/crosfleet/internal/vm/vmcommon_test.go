// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"google.golang.org/grpc"

	"go.chromium.org/chromiumos/config/go/test/api"

	croscommon "go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/vm_leaser/client"
)

const (
	testEmail = "example-user@google.com"
)

type mockVMLeaserClient struct {
	listLeasesFunc func(*api.ListLeasesRequest) (*api.ListLeasesResponse, error)
	releaseVMFunc  func(*api.ReleaseVMRequest) (*api.ReleaseVMResponse, error)
}

func (m mockVMLeaserClient) ListLeases(ctx context.Context, in *api.ListLeasesRequest, opts ...grpc.CallOption) (*api.ListLeasesResponse, error) {
	return m.listLeasesFunc(in)
}
func (m mockVMLeaserClient) LeaseVM(ctx context.Context, in *api.LeaseVMRequest, opts ...grpc.CallOption) (*api.LeaseVMResponse, error) {
	return nil, errors.New("Not implemented")
}

func (m mockVMLeaserClient) ReleaseVM(ctx context.Context, in *api.ReleaseVMRequest, opts ...grpc.CallOption) (*api.ReleaseVMResponse, error) {
	return m.releaseVMFunc(in)
}

func (m mockVMLeaserClient) ExtendLease(ctx context.Context, in *api.ExtendLeaseRequest, opts ...grpc.CallOption) (*api.ExtendLeaseResponse, error) {
	return nil, errors.New("Not implemented")
}

func (m mockVMLeaserClient) ImportImage(ctx context.Context, in *api.ImportImageRequest, opts ...grpc.CallOption) (*api.ImportImageResponse, error) {
	return nil, errors.New("Not implemented")
}

func TestSanitizeForLabel(t *testing.T) {
	expected := "example-user_google_com"
	actual := sanitizeForLabel(testEmail)

	if expected != actual {
		t.Errorf("Expected label to be %s, but is %s", expected, actual)
	}
}

func TestListLeases(t *testing.T) {
	expected := []*api.VM{
		{
			Id:        "vm-test",
			GceRegion: "us-west1-b",
		},
	}

	vmLeaser := client.Client{
		VMLeaserClient: mockVMLeaserClient{
			listLeasesFunc: func(r *api.ListLeasesRequest) (*api.ListLeasesResponse, error) {
				if r.GetParent() != "projects/"+croscommon.GceProject {
					return nil, fmt.Errorf("Unexpected parent: %s", r.GetParent())
				}
				if r.GetFilter() != fmt.Sprintf("labels.client:crosfleet AND labels.leased-by:%s AND status:RUNNING", sanitizeForLabel(testEmail)) {
					return nil, fmt.Errorf("Unexpected filter: %s", r.GetFilter())
				}
				return &api.ListLeasesResponse{
					Vms: []*api.VM{
						{
							Id:        "vm-test",
							GceRegion: "projects/chromeos-gce-tests/zones/us-west1-b",
						},
					},
				}, nil
			},
		},
		Email: testEmail,
	}

	ctx := context.Background()
	actual, err := listLeases(&vmLeaser, ctx)

	if err != nil {
		t.Errorf("Expected nil error, get %v", err)
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Expected result to be %v, but is %v", expected, actual)
	}
}
