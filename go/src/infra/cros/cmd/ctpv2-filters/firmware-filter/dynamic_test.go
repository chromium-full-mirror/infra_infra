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
	for tcIndex, tc := range []struct {
		inputFile          string
		expectedOutputFile string
	}{
		{"testdata/input1.textpb", "testdata/output1.textpb"},
		{"testdata/input2.textpb", "testdata/output2.textpb"},
	} {
		// Read the input file
		data, err := os.ReadFile(tc.inputFile)
		if err != nil {
			t.Errorf("[%d]Error reading %s: %+v", tcIndex, tc.inputFile, err)
			continue
		}
		req := &api.InternalTestplan{}
		err = prototext.Unmarshal(data, req)
		if err != nil {
			t.Errorf("[%d]Error unmarshaling %s: %+v", tcIndex, tc.inputFile, err)
			continue
		}

		// Read the expected file
		data, err = os.ReadFile(tc.expectedOutputFile)
		if err != nil {
			t.Errorf("[%d]Error reading %s: %+v", tcIndex, tc.expectedOutputFile, err)
			continue
		}
		expected := &api.InternalTestplan{}
		err = prototext.Unmarshal(data, expected)
		if err != nil {
			t.Errorf("[%d]Error unmarshaling %s: %+v", tcIndex, tc.expectedOutputFile, err)
			continue
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
		firmwareSpecs.FirmwareBuilds["corsola"] = FirmwareBranchBuild{
			Builder:         "firmware-corsola-15194.B-branch",
			FirmwareByBoard: "corsola/firmware_from_source.tar.bz2",
			ArtifactLink:    "gs://chromeos-image-archive/firmware-corsola-15194.B-branch/R109-15194.291.0-1-8724140311024464177",
		}

		err = GenerateDynamicInfo(req, firmwareSpecs, log.New(&TWriter{t: t}, "", log.Lshortfile))
		if err != nil {
			t.Errorf("[%d]GenerateDynamicInfo failed: %+v", tcIndex, err)
			continue
		}

		// Diff just the dynamicUpdates section first, for a better diff.
		if diff := cmp.Diff(expected.GetSuiteInfo().GetSuiteMetadata().GetDynamicUpdates(), req.GetSuiteInfo().GetSuiteMetadata().GetDynamicUpdates(), protocmp.Transform(),
			protocmp.SortRepeated(func(a, b *api.DynamicDep) bool {
				return a.GetKey() < b.GetKey()
			}),
		); diff != "" {
			t.Errorf("[%d]messages are not equal: %v", tcIndex, diff)
		}
		if diff := cmp.Diff(expected, req, protocmp.Transform(),
			protocmp.SortRepeated(func(a, b *api.DynamicDep) bool {
				return a.GetKey() < b.GetKey()
			}),
		); diff != "" {
			t.Errorf("[%d]messages are not equal: %v", tcIndex, diff)
		}
	}
}
