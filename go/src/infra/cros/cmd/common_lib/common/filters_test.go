// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

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

func TestGetDefaultFilters(t *testing.T) {
	ctx := context.Background()
	contMetadataMap := make(map[string]*buildapi.ContainerImageInfo)
	contMetadataMap["container1"] = &buildapi.ContainerImageInfo{
		Repository: nil,
		Name:       "container1",
		Digest:     "foo",
		Tags:       []string{"tag1", "tag2"},
	}
	contMetadataMap["container2"] = &buildapi.ContainerImageInfo{
		Repository: nil,
		Name:       "container2",
		Digest:     "foo",
		Tags:       []string{"tag1", "tag2"},
	}
	contMetadataMap["cros-test-finder"] = &buildapi.ContainerImageInfo{
		Repository: nil,
		Name:       "cros-test-finder",
		Digest:     "foo",
		Tags:       []string{"tag1", "tag2"},
	}
	contMetadataMap[LegacyHWContainerName] = &buildapi.ContainerImageInfo{
		Repository: nil,
		Name:       LegacyHWContainerName,
		Digest:     "foo",
		Tags:       []string{"tag1", "tag2"},
	}
	fmt.Println(contMetadataMap)

	filters, err := GetDefaultFilters(ctx, []string{"container1", TestFinderContainerName, LegacyHWContainerName}, contMetadataMap, 16005)
	if err != nil {
		t.Fatalf("got err: %s", err)
	}
	if len(filters) != 3 {
		t.Fatal("Expected 3 filters, got: ", len(filters))
	}

	// Check the index is correct.
	if filters[2].GetContainerInfo().GetContainer().GetName() != LegacyHWContainerName {
		t.Fatal("No LegacyHWContainerName found in default filters (or is out of order).")
	}
	if filters[2].GetContainerInfo().GetContainer().GetDigest() != fmt.Sprintf("foo") {
		t.Fatal("LegacyHWContainerName default has incorrect sha")
	}

	// Check test-finder is in place with the given digest.
	if filters[1].GetContainerInfo().GetContainer().GetName() != TestFinderContainerName {
		t.Fatal("No LegacyHWContainerName found in default filters (or is out of order).")
	}
	if filters[1].GetContainerInfo().GetContainer().GetDigest() != "foo" {
		fmt.Println(filters[1])
		t.Fatal("TestFinderContainerName has incorrect sha")
	}

	// this hits the backwards compatibility check
	if filters[1].GetContainerInfo().GetBinaryName() != "test_finder_filter" {
		fmt.Println(filters)
		t.Fatal("TestFinderContainerName incorrect binary_name")

	}

	if filters[0].GetContainerInfo().GetContainer().GetName() != "container1" {
		t.Fatal("No container1 found in default filters (or is out of order).")
	}
	if filters[0].GetContainerInfo().GetContainer().GetDigest() != "foo" {
		t.Fatal("container1 has incorrect sha")
	}

	// Test prior to the compatibility.
	filters, err = GetDefaultFilters(ctx, []string{TestFinderContainerName}, contMetadataMap, 15000)
	if err != nil {
		t.Fatalf("got err: %s", err)
	}

	// Check test-finder is in place with the given digest.
	if filters[0].GetContainerInfo().GetContainer().GetName() != TestFinderContainerName {
		t.Fatal("No LegacyHWContainerName found in default filters (or is out of order).")
	}
	if filters[0].GetContainerInfo().GetContainer().GetDigest() != "foo" {
		t.Fatal("TestFinderContainerName has incorrect sha")
	}

	// this hits the backwards compatibility check
	if filters[0].GetContainerInfo().GetBinaryName() != "cros-test-finder" {
		t.Fatal("TestFinderContainerName has wrong binary_name")

	}

	// Test missing filter errs.
	filters, err = GetDefaultFilters(ctx, []string{"somerandomfilter"}, contMetadataMap, 15000)
	if err == nil {
		t.Fatal("An undiscovered filter should have errored but didnt")
	}
}

func TestConstructCtpFilters(t *testing.T) {
	ctx := context.Background()
	defNames := []string{LegacyHWContainerName}
	extraFilter := &api.CTPFilter{
		ContainerInfo: buildTestContainer("container1", "foo", "container1_binaryName"),
	}
	filters := []*api.CTPFilter{extraFilter}
	contMetadataMap := make(map[string]*buildapi.ContainerImageInfo)
	contMetadataMap[LegacyHWContainerName] = &buildapi.ContainerImageInfo{
		Repository: nil,
		Name:       LegacyHWContainerName,
		Digest:     "foo",
		Tags:       []string{"tag1", "tag2"},
	}
	contMetadataMap["container1"] = &buildapi.ContainerImageInfo{
		Repository: nil,
		Name:       "container1",
		Digest:     "foo",
		Tags:       []string{"tag1", "tag2"},
	}

	filters, err := ConstructCtpFilters(ctx, defNames, contMetadataMap, filters, 15000)
	if err != nil {
		t.Fatal("err from ConstructCtpFilters: ", err)
	}

	if filters[0].GetContainerInfo().GetContainer().GetName() != LegacyHWContainerName {
		t.Fatal("Filters not in correct order")
	}
	if filters[1].GetContainerInfo().GetContainer().GetName() != "container1" {
		t.Fatal("Filters not in correct order")
	}

}
