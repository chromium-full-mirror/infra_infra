// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/testing/protocmp"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

type TWriter struct {
	t *testing.T
}

func (tw *TWriter) Write(p []byte) (n int, err error) {
	tw.t.Logf("%s", string(p))
	return len(p), nil
}

func TestGoldenFile(t *testing.T) {
	firmwareBuilds := make(map[string]FirmwareBranchBuild)
	firmwareBuilds["zork"] = FirmwareBranchBuild{
		Builder:         "firmware-zork-13434.B-branch",
		FirmwareByBoard: "zork/firmware_from_source.tar.bz2",
		ArtifactLink:    "gs://chromeos-image-archive/firmware-zork-13434.B-branch/R87-13434.899.0-1-8724259196952154561",
	}
	firmwareBuilds["corsola"] = FirmwareBranchBuild{
		Builder:         "firmware-corsola-15194.B-branch",
		FirmwareByBoard: "corsola/firmware_from_source.tar.bz2",
		ArtifactLink:    "gs://chromeos-image-archive/firmware-corsola-15194.B-branch/R109-15194.291.0-1-8724140311024464177",
	}
	firmwareBuilds["rex"] = FirmwareBranchBuild{
		Builder:         "firmware-rex-15709.B-branch",
		FirmwareByBoard: "rex/firmware_from_source.tar.bz2",
		ArtifactLink:    "gs://chromeos-image-archive/firmware-rex-15709.B-branch/R122-15709.223.0-1-8722295718732023121",
	}
	firmwareBuilds["nissa"] = FirmwareBranchBuild{
		Builder:         "firmware-nissa-15217.B-branch",
		FirmwareByBoard: "nissa/firmware_from_source.tar.bz2",
		ArtifactLink:    "gs://chromeos-image-archive/firmware-nissa-15217.B-branch/R109-15217.894.0-1-8717312278391894769",
	}
	firmwareBuilds["brya"] = FirmwareBranchBuild{
		Builder:         "firmware-brya-14505.B-branch",
		FirmwareByBoard: "brya/firmware_from_source.tar.bz2",
		ArtifactLink:    "gs://chromeos-image-archive/firmware-brya-14505.B-branch-firmware/R100-14505.682.0",
	}
	ecMilestoneBuilds := make(map[string]map[int]FirmwareBranchBuild)
	ecMilestoneBuilds["rex"] = make(map[int]FirmwareBranchBuild)
	ecMilestoneBuilds["rex"][134] = FirmwareBranchBuild{
		Builder:         "firmware-ec-R134-16181.3.B-branch",
		FirmwareByBoard: "rex/firmware_from_source.tar.bz2",
		ArtifactLink:    "gs://chromeos-image-archive/firmware-ec-R134-16181.3.B-branch/R134-16181.3.19-1-8721865933425388673",
	}
	// bucket is the storage bucket, i.e. chromeos-image-archive
	// branchPrefix is the branch up through the major version and dot, i.e. firmware-brya-14505.
	// release is optional, but if provided should be a milestone with trailing hyphen like R100-
	// version is the desired version, i.e. 14505.102.0
	// suffix is the path to the file, i.e. brya/firmware_from_source.tar.bz2
	fakeMatcher := func(ctx context.Context, bucket, branchPrefix, release, version, suffix, board string) (string, error) {
		if bucket == "chromeos-image-archive" && branchPrefix == "firmware-zork-13434." && release == "R87-" && version == "13434.283.0" && suffix == "zork/firmware_from_source.tar.bz2" && board == "zork" {
			return "gs://chromeos-image-archive/zork-firmware/R87-13434.283.0/firmware_from_source.tar.bz2", nil
		}
		if bucket == "chromeos-image-archive" && branchPrefix == "firmware-nissa-15217." && release == "R109-" && version == "15217.439.0" && suffix == "nissa/firmware_from_source.tar.bz2" && board == "nissa" {
			return "gs://chromeos-image-archive/firmware-nissa-15217.B-branch-firmware/R109-15217.439.0/nissa/firmware_from_source.tar.bz2", nil
		}
		if bucket == "chromeos-image-archive" && branchPrefix == "firmware-nissa-15217." && release == "R109-" && version == "15217.608.0" && suffix == "nissa/firmware_from_source.tar.bz2" && board == "nissa" {
			return "gs://chromeos-image-archive/firmware-nissa-15217.B-branch-firmware/R109-15217.608.0/nissa/firmware_from_source.tar.bz2", nil
		}
		if bucket == "chromeos-image-archive" && branchPrefix == "firmware-brya-14505." && release == "R100-" && version == "14505.682.0" && suffix == "brya/firmware_from_source.tar.bz2" && board == "brya" {
			return "gs://chromeos-image-archive/firmware-brya-14505.B-branch-firmware/R100-14505.682.0/brya/firmware_from_source.tar.bz2", nil
		}
		return "", fmt.Errorf("Unexpected args bucket=%q branchPrefix=%q release=%q, version=%q suffix=%q board=%q", bucket, branchPrefix, release, version, suffix, board)
	}
	for tcIndex, tc := range []struct {
		inputFile          string
		expectedOutputFile string
		specs              *FirmwareSpecs
	}{
		{"testdata/input1.textpb", "testdata/output1.textpb", &FirmwareSpecs{
			Ro:             LatestFirmwareBranch + "," + OSSource,
			FirmwareBuilds: firmwareBuilds,
		}},
		{"testdata/input2.textpb", "testdata/output2.textpb", &FirmwareSpecs{
			Ro:             LatestFirmwareBranch + "," + OSSource,
			FirmwareBuilds: firmwareBuilds,
		}},
		{"testdata/faft_bios_rex_input1.textpb", "testdata/faft_bios_rex_output1.textpb", &FirmwareSpecs{
			Ro:                LatestFirmwareBranch + "," + OSSource,
			ECRO:              "M-1," + LatestFirmwareBranch + "," + OSSource,
			ECRW:              "M-1," + LatestFirmwareBranch + "," + OSSource,
			FirmwareBuilds:    firmwareBuilds,
			ECMilestoneBuilds: ecMilestoneBuilds,
			LatestMilestone:   135,
		}},
		// [3] Explicit version from fw branch
		{"testdata/input1.textpb", "testdata/output_13434.283.0.textpb", &FirmwareSpecs{
			Ro:             "13434.283.0",
			FirmwareBuilds: firmwareBuilds,
		}},
		// [4] Explicit versions into test args
		{"testdata/testargs_input.textpb", "testdata/testargs_output.textpb", &FirmwareSpecs{
			TestArgReplacements: map[string]string{
				"old_bios_ro": "15217.439.0",
				"old_bios_rw": "15217.439.0",
				"old_ec":      "15217.439.0",
				"new_bios_ro": "15217.608.0",
				"new_bios_rw": "15217.608.0",
				"new_ec":      "15217.608.0",
			},
			FirmwareBuilds: firmwareBuilds,
		}},
		// [5] AL FAFT FW Qual request
		{"testdata/al_input1.textpb", "testdata/al_output1.textpb", &FirmwareSpecs{
			Ro:             "14505.682.0",
			ECRO:           "14505.682.0",
			Rw:             "gs://chromeos-image-archive/firmware-android-brya-14505.782.B-branch-firmware/R100-14505.782.212/brya/firmware_from_source.tar.bz2",
			ECRW:           "gs://chromeos-image-archive/firmware-android-brya-14505.782.B-branch-firmware/R100-14505.782.212/brya/firmware_from_source.tar.bz2",
			FirmwareBuilds: firmwareBuilds,
		}},
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

		err = GenerateDynamicInfo(context.Background(), &common.CommonFilterParams{}, req, tc.specs, log.New(&TWriter{t: t}, "", log.Lshortfile), fakeMatcher)
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
			t.Errorf("[%d]messages are not equal: %s", tcIndex, diff)
		}
		if diff := cmp.Diff(expected, req, protocmp.Transform(),
			protocmp.SortRepeated(func(a, b *api.DynamicDep) bool {
				return a.GetKey() < b.GetKey()
			}),
			protocmp.SortRepeated(func(a, b *api.Arg) bool {
				return a.GetFlag() < b.GetFlag() || (a.GetFlag() == b.GetFlag() && a.GetValue() < b.GetValue())
			}),
		); diff != "" {
			t.Errorf("[%d]messages are not equal: %s", tcIndex, diff)
		}
	}
}
