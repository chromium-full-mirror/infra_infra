// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package firmwareservice

import (
	"context"
	"testing"
)

func TestGetCandidateURLs(t *testing.T) {
	// smallUrlPrefix := "gs://firmware-image-archive/firmware-brya-14505.B/14505.832.0"

	type TestCase struct {
		inputGsUrl           string
		board                string
		model                string
		corebootName         string // firmware.build-targets.coreboot || firmware.image-name
		legacyECName         string // firmware.build-targets.ec || firmware.build-targets.coreboot
		standaloneECName     string // firmware.build-targets.ec || firmware.build-targets.zephyr-ec
		expectedAPCandidates []ImageCandidate
		expectedECCandidates []ImageCandidate
	}

	testCases := []TestCase{
		{
			inputGsUrl:       "gs://chromeos-image-archive/firmware-brya-14505.B-branch/R100-14505.832.0-1-8730368903603296945/brya/firmware_from_source.tar.bz2",
			board:            "brya",
			model:            "omniknight",
			corebootName:     "omnigul",
			legacyECName:     "omnigul",
			standaloneECName: "omnigul",
			expectedAPCandidates: []ImageCandidate{
				{
					GSURL:     "gs://firmware-image-archive/firmware-brya-14505.B/14505.832.0/omnigul.14505.832.0.tar.bz2",
					Filenames: []string{"image-omnigul.bin"},
				},
				{
					GSURL:     "gs://chromeos-image-archive/firmware-brya-14505.B-branch/R100-14505.832.0-1-8730368903603296945/brya/firmware_from_source.tar.bz2",
					Filenames: []string{"image-omnigul.bin", "image-omniknight.bin", "image-brya.bin", "image.bin", "bios.bin"},
				},
			},
			expectedECCandidates: []ImageCandidate{
				{
					GSURL:     "gs://firmware-image-archive/firmware-brya-14505.B/14505.832.0/omnigul.EC.14505.832.0.tar.bz2",
					Filenames: []string{"ec.bin"},
				},
				{
					GSURL:     "gs://chromeos-image-archive/firmware-brya-14505.B-branch/R100-14505.832.0-1-8730368903603296945/brya/firmware_from_source.tar.bz2",
					Filenames: []string{"omnigul/ec.bin", "omniknight/ec.bin", "brya/ec.bin", "ec.bin"},
				},
			},
		},
		{
			inputGsUrl:       "gs://firmware-image-archive/firmware-ec-R134-16181.3.B/16181.3.14/rex/firmware_from_source.tar.bz2",
			board:            "rex",
			model:            "karis",
			corebootName:     "karis",
			legacyECName:     "karis",
			standaloneECName: "karis",
			expectedAPCandidates: []ImageCandidate{
				{
					GSURL:     "gs://firmware-image-archive/firmware-ec-R134-16181.3.B/16181.3.14/karis.16181.3.14.tar.bz2",
					Filenames: []string{"image-karis.bin"},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-ec-R134-16181.3.B/16181.3.14/rex/firmware_from_source.tar.bz2",
					Filenames: []string{"image-karis.bin", "image-karis.bin", "image-rex.bin", "image.bin", "bios.bin"},
				},
			},
			expectedECCandidates: []ImageCandidate{
				{
					GSURL:     "gs://firmware-image-archive/firmware-ec-R134-16181.3.B/16181.3.14/karis.EC.16181.3.14.tar.bz2",
					Filenames: []string{"ec.bin"},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-ec-R134-16181.3.B/16181.3.14/rex/firmware_from_source.tar.bz2",
					Filenames: []string{"karis/ec.bin", "karis/ec.bin", "rex/ec.bin", "ec.bin"},
				},
			},
		},
		{
			inputGsUrl:       "gs://firmware-image-archive/firmware-ec-R134-16181.3.B/16181.3.14/karis.16181.3.14.tar.bz2",
			board:            "rex",
			model:            "karis",
			corebootName:     "karis",
			legacyECName:     "karis",
			standaloneECName: "karis",
			expectedAPCandidates: []ImageCandidate{
				{
					GSURL:     "gs://firmware-image-archive/firmware-ec-R134-16181.3.B/16181.3.14/karis.16181.3.14.tar.bz2",
					Filenames: []string{"image-karis.bin"},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-ec-R134-16181.3.B/16181.3.14/karis.16181.3.14.tar.bz2",
					Filenames: []string{"image-karis.bin", "image-karis.bin", "image-rex.bin", "image.bin", "bios.bin"},
				},
			},
			expectedECCandidates: []ImageCandidate{
				{
					GSURL:     "gs://firmware-image-archive/firmware-ec-R134-16181.3.B/16181.3.14/karis.EC.16181.3.14.tar.bz2",
					Filenames: []string{"ec.bin"},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-ec-R134-16181.3.B/16181.3.14/karis.16181.3.14.tar.bz2",
					Filenames: []string{"karis/ec.bin", "karis/ec.bin", "rex/ec.bin", "ec.bin"},
				},
			},
		},
		{
			inputGsUrl:       "gs://chromeos-image-archive/firmware-rex-15709.B-branch-firmware/R122-15709.59.0/rex/firmware_from_source.tar.bz2",
			board:            "rex",
			model:            "karis",
			corebootName:     "karis",
			legacyECName:     "karis",
			standaloneECName: "karis",
			expectedAPCandidates: []ImageCandidate{
				{
					GSURL:     "gs://firmware-image-archive/firmware-rex-15709.B/15709.59.0/karis.15709.59.0.tar.bz2",
					Filenames: []string{"image-karis.bin"},
				},
				{
					GSURL:     "gs://chromeos-image-archive/firmware-rex-15709.B-branch-firmware/R122-15709.59.0/rex/firmware_from_source.tar.bz2",
					Filenames: []string{"image-karis.bin", "image-karis.bin", "image-rex.bin", "image.bin", "bios.bin"},
				},
			},
			expectedECCandidates: []ImageCandidate{
				{
					GSURL:     "gs://firmware-image-archive/firmware-rex-15709.B/15709.59.0/karis.EC.15709.59.0.tar.bz2",
					Filenames: []string{"ec.bin"},
				},
				{
					GSURL:     "gs://chromeos-image-archive/firmware-rex-15709.B-branch-firmware/R122-15709.59.0/rex/firmware_from_source.tar.bz2",
					Filenames: []string{"karis/ec.bin", "karis/ec.bin", "rex/ec.bin", "ec.bin"},
				},
			},
		},
	}
	for _, testCase := range testCases {
		t.Logf("Running testCase %+v", testCase)
		actualCandidates, err := GetAPCandidateURLs(context.Background(), testCase.inputGsUrl, &FirmwareService{
			board:            testCase.board,
			model:            testCase.model,
			CorebootName:     testCase.corebootName,
			StandaloneECName: testCase.standaloneECName,
			LegacyECName:     testCase.legacyECName,
		})
		if err != nil {
			t.Errorf("GetAPCandidateURLs failed: %v", err)
			continue
		}
		compareCandiates(t, "AP", actualCandidates, testCase.expectedAPCandidates)
		actualCandidates, err = GetECCandidateURLs(context.Background(), testCase.inputGsUrl, &FirmwareService{
			board:            testCase.board,
			model:            testCase.model,
			CorebootName:     testCase.corebootName,
			StandaloneECName: testCase.standaloneECName,
			LegacyECName:     testCase.legacyECName,
		})
		if err != nil {
			t.Errorf("GetECCandidateURLs failed: %v", err)
			continue
		}
		compareCandiates(t, "EC", actualCandidates, testCase.expectedECCandidates)
	}
}

func compareCandiates(t *testing.T, imageType string, actualCandidates, expectedCandidates []ImageCandidate) {
	for i, expected := range expectedCandidates {
		if len(actualCandidates) <= i {
			t.Errorf("[%s %d]: Missing candidate %v", imageType, i, expected)
			continue
		}
		actual := actualCandidates[i]
		if expected.GSURL != actual.GSURL {
			t.Errorf("[%s %d]: Incorrect GSURL, got %q, want %q", imageType, i, actual.GSURL, expected.GSURL)
		}
		for f, expectedFilename := range expected.Filenames {
			if len(actual.Filenames) <= f {
				t.Errorf("[%s %d.%d]: Missing filename %v", imageType, i, f, expectedFilename)
				continue
			}
			actualFilename := actual.Filenames[f]
			if actualFilename != expectedFilename {
				t.Errorf("[%s %d.%d]: Incorrect filename, got %q, want %q", imageType, i, f, actualFilename, expectedFilename)
			}
		}
		for f := len(expected.Filenames); f < len(actual.Filenames); f += 1 {
			t.Errorf("[%s %d.%d]: Extra filename %v", imageType, i, f, actual.Filenames[f])
		}
	}
	for i := len(expectedCandidates); i < len(actualCandidates); i += 1 {
		t.Errorf("[%s %d]: Extra candidate %v", imageType, i, actualCandidates[i])
	}
}
