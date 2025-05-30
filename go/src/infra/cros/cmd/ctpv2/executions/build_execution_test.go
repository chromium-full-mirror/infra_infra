// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executions

import (
	"context"
	"fmt"
	"testing"

	buildapi "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"
)

func buildTestContainer(name string, digest string, binaryName string) *api.ContainerInfo {
	c := &buildapi.ContainerImageInfo{
		Repository: &buildapi.GcrRepository{
			Hostname: "us-docker.pkg.dev",
			Project:  "cros-registry/test-services",
		},
		Name:   name,
		Digest: fmt.Sprintf("sha256:%s", digest),
		Tags:   []string{"prod"},
	}
	return &api.ContainerInfo{Container: c, BinaryName: binaryName}
}

func TestGetTotalFilters(t *testing.T) {
	ctx := context.Background()
	req := &api.CTPRequest{
		KarbonFilters: []*api.CTPFilter{},
	}
	defKarbon := []*api.CTPFilter{
		{
			ContainerInfo: &api.ContainerInfo{
				Container: &buildapi.ContainerImageInfo{Name: "foo"},
			},
		}, {
			ContainerInfo: &api.ContainerInfo{
				Container: &buildapi.ContainerImageInfo{Name: "bar"},
			},
		}, {
			ContainerInfo: &api.ContainerInfo{
				Container: &buildapi.ContainerImageInfo{Name: "fizz"},
			},
		},
	}
	numFilter := getTotalFilters(ctx, req, defKarbon)
	if numFilter != 3 {
		t.Fatalf("expected 3 default filters got: %v", numFilter)
	}

	req = &api.CTPRequest{
		KarbonFilters: []*api.CTPFilter{
			{ContainerInfo: buildTestContainer("foo", "1234", "foo")},
		}}

	numFilter = getTotalFilters(ctx, req, defKarbon)
	if numFilter != 3 {
		t.Fatalf("expected 3 filters when provided overwrites default. got: %v", numFilter)
	}
}
