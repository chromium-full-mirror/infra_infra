// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"log"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/testing/protocmp"

	"go.chromium.org/chromiumos/config/go/test/api"
)

type TWriter struct {
	t *testing.T
}

func (tw *TWriter) Write(p []byte) (n int, err error) {
	tw.t.Logf("%s", string(p))
	return len(p), nil
}

func TestGoldenFile(t *testing.T) {
	// Read the input file
	data, err := os.ReadFile("testdata/input1.textpb")
	if err != nil {
		t.Fatal("Error reading input1:", err)
	}
	req := &api.InternalTestplan{}
	err = prototext.Unmarshal(data, req)
	if err != nil {
		t.Fatal("Error unmarshaling input1:", err)
	}

	// Read the expected file
	data, err = os.ReadFile("testdata/output1.textpb")
	if err != nil {
		t.Fatal("Error reading output1:", err)
	}
	expected := &api.InternalTestplan{}
	err = prototext.Unmarshal(data, expected)
	if err != nil {
		t.Fatal("Error unmarshaling output1:", err)
	}

	firmwareSpecs := &FirmwareSpecs{}
	firmwareSpecs.Ro = LatestFirmwareBranch
	firmwareSpecs.FallbackToCros = true
	firmwareSpecs.FirmwareBuilds = make(map[string]FirmwareBranchBuild)
	firmwareSpecs.FirmwareBuilds["zork"] = FirmwareBranchBuild{
		Builder:         "firmware-zork-13434.B-branch",
		FirmwareByBoard: "zork/firmware_from_source.tar.bz2",
		ArtifactLink:    "gs://chromeos-image-archive/firmware-zork-13434.B-branch/R87-13434.899.0-1-8724259196952154561",
	}

	err = GenerateDynamicInfo(req, firmwareSpecs, log.New(&TWriter{t: t}, "", log.Lshortfile))
	if err != nil {
		t.Fatal("GenerateDynamicInfo failed:", err)
	}

	// Diff just the dynamicUpdates section first, for a better diff.
	if diff := cmp.Diff(expected.GetSuiteInfo().GetSuiteMetadata().GetDynamicUpdates(), req.GetSuiteInfo().GetSuiteMetadata().GetDynamicUpdates(), protocmp.Transform(),
		protocmp.SortRepeated(func(a, b *api.DynamicDep) bool {
			return a.GetKey() < b.GetKey()
		}),
	); diff != "" {
		t.Errorf("messages are not equal: %v", diff)
	}
	if diff := cmp.Diff(expected, req, protocmp.Transform(),
		protocmp.SortRepeated(func(a, b *api.DynamicDep) bool {
			return a.GetKey() < b.GetKey()
		}),
	); diff != "" {
		t.Errorf("messages are not equal: %v", diff)
	}
}
