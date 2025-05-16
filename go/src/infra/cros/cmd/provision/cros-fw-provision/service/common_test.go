// Copyright 2024 The Chromium Authors
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
		// A recent brya build for a model that doesn't match the image name.
		{
			inputGsUrl:       "gs://chromeos-image-archive/firmware-brya-14505.B-branch/R100-14505.832.0-1-8730368903603296945/brya/firmware_from_source.tar.bz2",
			board:            "brya",
			model:            "omniknight",
			corebootName:     "omnigul",
			legacyECName:     "omnigul",
			standaloneECName: "omnigul",
			expectedAPCandidates: []ImageCandidate{
				{
					GSURL:     "gs://firmware-image-archive/firmware-brya-14505.B/14505.832.0/omnigul.14505.832.0.tar.bz2", // exists
					Filenames: []string{"image-omnigul.bin" /* exists */, "image.bin"},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-brya-14505.B/14505.832.0/Omnigul.14505.832.0.tbz2",
					Filenames: []string{"image-omnigul.bin", "image.bin"},
				},
				{
					GSURL:     "gs://chromeos-image-archive/firmware-brya-14505.B-branch/R100-14505.832.0-1-8730368903603296945/brya/firmware_from_source.tar.bz2", // exists
					Filenames: []string{"image-omnigul.bin" /* exists */, "image-omniknight.bin", "image-brya.bin", "image.bin", "bios.bin"},
				},
			},
			expectedECCandidates: []ImageCandidate{
				{
					GSURL:     "gs://firmware-image-archive/firmware-brya-14505.B/14505.832.0/omnigul.EC.14505.832.0.tar.bz2", // exists
					Filenames: []string{"ec.bin" /* exists */},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-brya-14505.B/14505.832.0/Omnigul_EC.14505.832.0.tbz2",
					Filenames: []string{"ec.bin"},
				},
				{
					GSURL:     "gs://chromeos-image-archive/firmware-brya-14505.B-branch/R100-14505.832.0-1-8730368903603296945/brya/firmware_from_source.tar.bz2", // exists
					Filenames: []string{"omnigul/ec.bin" /* exists */, "omniknight/ec.bin", "brya/ec.bin" /* exists */, "ec.bin"},
				},
			},
		},
		// aviko is somewhat complicated, in that model != coreboot != ec.
		{
			inputGsUrl:       "gs://chromeos-image-archive/firmware-brya-14505.B-branch/R100-14505.832.0-1-8730368903603296945/brya/firmware_from_source.tar.bz2",
			board:            "brya",
			model:            "aviko",
			corebootName:     "skolas",
			legacyECName:     "brya",
			standaloneECName: "brya",
			expectedAPCandidates: []ImageCandidate{
				{
					GSURL:     "gs://firmware-image-archive/firmware-brya-14505.B/14505.832.0/skolas.14505.832.0.tar.bz2", // exists
					Filenames: []string{"image-skolas.bin" /* exists */, "image.bin"},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-brya-14505.B/14505.832.0/Skolas.14505.832.0.tbz2",
					Filenames: []string{"image-skolas.bin", "image.bin"},
				},
				{
					GSURL:     "gs://chromeos-image-archive/firmware-brya-14505.B-branch/R100-14505.832.0-1-8730368903603296945/brya/firmware_from_source.tar.bz2", // exists
					Filenames: []string{"image-skolas.bin" /* exists */, "image-aviko.bin", "image-brya.bin", "image.bin", "bios.bin"},
				},
			},
			expectedECCandidates: []ImageCandidate{
				{
					GSURL:     "gs://firmware-image-archive/firmware-brya-14505.B/14505.832.0/brya.EC.14505.832.0.tar.bz2", // exists
					Filenames: []string{"ec.bin" /* exists */},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-brya-14505.B/14505.832.0/Brya_EC.14505.832.0.tbz2",
					Filenames: []string{"ec.bin"},
				},
				{
					GSURL:     "gs://chromeos-image-archive/firmware-brya-14505.B-branch/R100-14505.832.0-1-8730368903603296945/brya/firmware_from_source.tar.bz2", // exists
					Filenames: []string{"brya/ec.bin" /* exists */, "aviko/ec.bin", "brya/ec.bin" /* exists */, "ec.bin"},
				},
			},
		},
		// There is a sku of brox that has model=brox coreboot=brox_ec_ish zephyrEC=brox-ish-ec, verify that legacyECName is used in the images.
		{
			inputGsUrl:       "gs://chromeos-image-archive/firmware-brox-16080.B-branch/R132-16080.159.0-1-8718833705983315281/brox/firmware_from_source.tar.bz2",
			board:            "brox",
			model:            "brox",
			corebootName:     "brox_ec_ish",
			legacyECName:     "brox_ec_ish",
			standaloneECName: "brox-ish-ec",
			expectedAPCandidates: []ImageCandidate{
				{
					GSURL:     "gs://firmware-image-archive/firmware-brox-16080.B/16080.159.0/brox_ec_ish.16080.159.0.tar.bz2", // exists
					Filenames: []string{"image-brox_ec_ish.bin" /* exists */, "image.bin"},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-brox-16080.B/16080.159.0/Brox_Ec_Ish.16080.159.0.tbz2",
					Filenames: []string{"image-brox_ec_ish.bin", "image.bin"},
				},
				{
					GSURL:     "gs://chromeos-image-archive/firmware-brox-16080.B-branch/R132-16080.159.0-1-8718833705983315281/brox/firmware_from_source.tar.bz2", // exists
					Filenames: []string{"image-brox_ec_ish.bin" /* exists */, "image-brox.bin" /* exists */, "image-brox.bin", "image.bin", "bios.bin"},
				},
			},
			expectedECCandidates: []ImageCandidate{
				{
					GSURL:     "gs://firmware-image-archive/firmware-brox-16080.B/16080.159.0/brox_ec_ish.EC.16080.159.0.tar.bz2", // exists
					Filenames: []string{"ec.bin" /* exists */},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-brox-16080.B/16080.159.0/Brox_Ec_Ish_EC.16080.159.0.tbz2",
					Filenames: []string{"ec.bin"},
				},
				{
					GSURL:     "gs://chromeos-image-archive/firmware-brox-16080.B-branch/R132-16080.159.0-1-8718833705983315281/brox/firmware_from_source.tar.bz2", // exists
					Filenames: []string{"brox_ec_ish/ec.bin" /* exists */, "brox/ec.bin" /* exists */, "brox/ec.bin" /* exists */, "ec.bin"},
				},
			},
		},
		// There is a sku of brox that has model=brox coreboot=brox_ec_ish zephyrEC=brox-ish-ec, verify that standaloneECName is used in the images from the firmware-ec branch.
		{
			inputGsUrl:       "gs://chromeos-image-archive/firmware-ec-R135-16209.5.B-branch/R135-16209.5.9-1-8720478693255597665/brox/firmware_from_source.tar.bz2",
			board:            "brox",
			model:            "brox",
			corebootName:     "brox_ec_ish",
			legacyECName:     "brox_ec_ish",
			standaloneECName: "brox-ish-ec",
			expectedAPCandidates: []ImageCandidate{
				{
					GSURL:     "gs://firmware-image-archive/firmware-ec-R135-16209.5.B/16209.5.9/brox_ec_ish.16209.5.9.tar.bz2",
					Filenames: []string{"image-brox_ec_ish.bin", "image.bin"},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-ec-R135-16209.5.B/16209.5.9/Brox_Ec_Ish.16209.5.9.tbz2",
					Filenames: []string{"image-brox_ec_ish.bin", "image.bin"},
				},
				{
					GSURL:     "gs://chromeos-image-archive/firmware-ec-R135-16209.5.B-branch/R135-16209.5.9-1-8720478693255597665/brox/firmware_from_source.tar.bz2", // exists
					Filenames: []string{"image-brox_ec_ish.bin", "image-brox.bin", "image-brox.bin", "image.bin", "bios.bin"},
				},
			},
			expectedECCandidates: []ImageCandidate{
				{
					GSURL:     "gs://firmware-image-archive/firmware-ec-R135-16209.5.B/16209.5.9/brox-ish-ec.EC.16209.5.9.tar.bz2", // exists
					Filenames: []string{"ec.bin" /* exists */},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-ec-R135-16209.5.B/16209.5.9/Brox-Ish-Ec_EC.16209.5.9.tbz2",
					Filenames: []string{"ec.bin"},
				},
				{
					GSURL:     "gs://chromeos-image-archive/firmware-ec-R135-16209.5.B-branch/R135-16209.5.9-1-8720478693255597665/brox/firmware_from_source.tar.bz2", // exists
					Filenames: []string{"brox-ish-ec/ec.bin" /* exists */, "brox/ec.bin" /* exists */, "brox/ec.bin" /* exists */, "ec.bin"},
				},
			},
		},
		// A firmware-ec firmware_from_source.tar.bz2 artifact. There are no AP images, so those are nonsense.
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
					Filenames: []string{"image-karis.bin", "image.bin"},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-ec-R134-16181.3.B/16181.3.14/Karis.16181.3.14.tbz2",
					Filenames: []string{"image-karis.bin", "image.bin"},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-ec-R134-16181.3.B/16181.3.14/rex/firmware_from_source.tar.bz2", // exists
					Filenames: []string{"image-karis.bin", "image-karis.bin", "image-rex.bin", "image.bin", "bios.bin"},
				},
			},
			expectedECCandidates: []ImageCandidate{
				{
					GSURL:     "gs://firmware-image-archive/firmware-ec-R134-16181.3.B/16181.3.14/karis.EC.16181.3.14.tar.bz2", // exists
					Filenames: []string{"ec.bin" /* exists */},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-ec-R134-16181.3.B/16181.3.14/Karis_EC.16181.3.14.tbz2",
					Filenames: []string{"ec.bin"},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-ec-R134-16181.3.B/16181.3.14/rex/firmware_from_source.tar.bz2", // exists
					Filenames: []string{"karis/ec.bin" /* exists */, "karis/ec.bin" /* exists */, "rex/ec.bin" /* exists */, "ec.bin"},
				},
			},
		},
		// A firmware-ec single model artifact. There are no AP images, so those are nonsense.
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
					Filenames: []string{"image-karis.bin", "image.bin"},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-ec-R134-16181.3.B/16181.3.14/Karis.16181.3.14.tbz2",
					Filenames: []string{"image-karis.bin", "image.bin"},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-ec-R134-16181.3.B/16181.3.14/karis.16181.3.14.tar.bz2",
					Filenames: []string{"image-karis.bin", "image-karis.bin", "image-rex.bin", "image.bin", "bios.bin"},
				},
			},
			expectedECCandidates: []ImageCandidate{
				{
					GSURL:     "gs://firmware-image-archive/firmware-ec-R134-16181.3.B/16181.3.14/karis.EC.16181.3.14.tar.bz2", // exists
					Filenames: []string{"ec.bin" /* exists*/},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-ec-R134-16181.3.B/16181.3.14/Karis_EC.16181.3.14.tbz2",
					Filenames: []string{"ec.bin"},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-ec-R134-16181.3.B/16181.3.14/karis.16181.3.14.tar.bz2",
					Filenames: []string{"karis/ec.bin", "karis/ec.bin", "rex/ec.bin", "ec.bin"},
				},
			},
		},
		// A firmware-rex firmware_from_source.tar.bz2 artifact in chromeos-image-archive. Too old to have firmware-image-archive artifacts.
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
					Filenames: []string{"image-karis.bin", "image.bin"},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-rex-15709.B/15709.59.0/Karis.15709.59.0.tbz2",
					Filenames: []string{"image-karis.bin", "image.bin"},
				},
				{
					GSURL:     "gs://chromeos-image-archive/firmware-rex-15709.B-branch-firmware/R122-15709.59.0/rex/firmware_from_source.tar.bz2", // exists
					Filenames: []string{"image-karis.bin" /* exists */, "image-karis.bin" /* exists */, "image-rex.bin", "image.bin", "bios.bin"},
				},
			},
			expectedECCandidates: []ImageCandidate{
				{
					GSURL:     "gs://firmware-image-archive/firmware-rex-15709.B/15709.59.0/karis.EC.15709.59.0.tar.bz2",
					Filenames: []string{"ec.bin"},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-rex-15709.B/15709.59.0/Karis_EC.15709.59.0.tbz2",
					Filenames: []string{"ec.bin"},
				},
				{
					GSURL:     "gs://chromeos-image-archive/firmware-rex-15709.B-branch-firmware/R122-15709.59.0/rex/firmware_from_source.tar.bz2", // exists
					Filenames: []string{"karis/ec.bin" /* exists */, "karis/ec.bin" /* exists */, "rex/ec.bin", "ec.bin"},
				},
			},
		},
		// A firmware-image-archive firmware-brya directory with no artifact listed.
		{
			inputGsUrl:       "gs://firmware-image-archive/firmware-brya-14505.B/14505.832.0/",
			board:            "brya",
			model:            "omniknight",
			corebootName:     "omnigul",
			legacyECName:     "omnigul",
			standaloneECName: "omnigul",
			expectedAPCandidates: []ImageCandidate{
				{
					GSURL:     "gs://firmware-image-archive/firmware-brya-14505.B/14505.832.0/omnigul.14505.832.0.tar.bz2", // exists
					Filenames: []string{"image-omnigul.bin" /* exists */, "image.bin"},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-brya-14505.B/14505.832.0/Omnigul.14505.832.0.tbz2",
					Filenames: []string{"image-omnigul.bin", "image.bin"},
				},
			},
			expectedECCandidates: []ImageCandidate{
				{
					GSURL:     "gs://firmware-image-archive/firmware-brya-14505.B/14505.832.0/omnigul.EC.14505.832.0.tar.bz2", // exists
					Filenames: []string{"ec.bin" /* exists */},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-brya-14505.B/14505.832.0/Omnigul_EC.14505.832.0.tbz2",
					Filenames: []string{"ec.bin"},
				},
			},
		},
		// A firmware-image-archive firmware-ec directory with no artifact listed. No AP artifacts exist.
		{
			inputGsUrl:       "gs://firmware-image-archive/firmware-ec-R134-16181.3.B/16181.3.14/",
			board:            "rex",
			model:            "karis",
			corebootName:     "karis",
			legacyECName:     "karis",
			standaloneECName: "karis",
			expectedAPCandidates: []ImageCandidate{
				{
					GSURL:     "gs://firmware-image-archive/firmware-ec-R134-16181.3.B/16181.3.14/karis.16181.3.14.tar.bz2",
					Filenames: []string{"image-karis.bin", "image.bin"},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-ec-R134-16181.3.B/16181.3.14/Karis.16181.3.14.tbz2",
					Filenames: []string{"image-karis.bin", "image.bin"},
				},
			},
			expectedECCandidates: []ImageCandidate{
				{
					GSURL:     "gs://firmware-image-archive/firmware-ec-R134-16181.3.B/16181.3.14/karis.EC.16181.3.14.tar.bz2", // exists
					Filenames: []string{"ec.bin" /* exists */},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-ec-R134-16181.3.B/16181.3.14/Karis_EC.16181.3.14.tbz2",
					Filenames: []string{"ec.bin"},
				},
			},
		},
		// A really old artifact that isn't named firmware_from_source.tar.bz2
		{
			inputGsUrl:       "gs://chromeos-releases/canary-channel/zork/13433.0.0/ChromeOS-firmware-R87-13433.0.0-zork.tar.bz2",
			board:            "zork",
			model:            "vilboz",
			corebootName:     "vilboz",
			legacyECName:     "vilboz",
			standaloneECName: "vilboz",
			expectedAPCandidates: []ImageCandidate{
				{
					GSURL:     "gs://chromeos-releases/canary-channel/zork/13433.0.0/ChromeOS-firmware-R87-13433.0.0-zork.tar.bz2", // exists
					Filenames: []string{"image-vilboz.bin" /* exists */, "image-vilboz.bin" /* exists */, "image-zork.bin", "image.bin", "bios.bin"},
				},
			},
			expectedECCandidates: []ImageCandidate{
				{
					GSURL:     "gs://chromeos-releases/canary-channel/zork/13433.0.0/ChromeOS-firmware-R87-13433.0.0-zork.tar.bz2", // exists
					Filenames: []string{"vilboz/ec.bin" /* exists */, "vilboz/ec.bin" /* exists */, "zork/ec.bin", "ec.bin"},
				},
			},
		},
		// A fairly complicated name, which has artifacts, but doesn't appear to be ever used in boxter.
		{
			inputGsUrl:       "gs://chromeos-image-archive/firmware-nissa-15217.B-branch/R109-15217.916.0-1-8714961020606258097/nissa/firmware_from_source.tar.bz2",
			board:            "nissa",
			model:            "glassway",
			corebootName:     "glassway_hdmi_ufs",
			legacyECName:     "glassway_hdmi_ufs",
			standaloneECName: "glassway",
			expectedAPCandidates: []ImageCandidate{
				{
					GSURL:     "gs://firmware-image-archive/firmware-nissa-15217.B/15217.916.0/glassway_hdmi_ufs.15217.916.0.tar.bz2",
					Filenames: []string{"image-glassway_hdmi_ufs.bin", "image.bin"},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-nissa-15217.B/15217.916.0/Glassway_Hdmi_Ufs.15217.916.0.tbz2", // exists
					Filenames: []string{"image-glassway_hdmi_ufs.bin" /* exists */, "image.bin"},
				},
				{
					"gs://chromeos-image-archive/firmware-nissa-15217.B-branch/R109-15217.916.0-1-8714961020606258097/nissa/firmware_from_source.tar.bz2",
					[]string{"image-glassway_hdmi_ufs.bin", "image-glassway.bin", "image-nissa.bin", "image.bin", "bios.bin"},
				},
			},
			expectedECCandidates: []ImageCandidate{
				{
					GSURL:     "gs://firmware-image-archive/firmware-nissa-15217.B/15217.916.0/glassway_hdmi_ufs.EC.15217.916.0.tar.bz2",
					Filenames: []string{"ec.bin"},
				},
				{
					GSURL:     "gs://firmware-image-archive/firmware-nissa-15217.B/15217.916.0/Glassway_Hdmi_Ufs_EC.15217.916.0.tbz2", // exists
					Filenames: []string{"ec.bin" /* exists */},
				},
				{
					"gs://chromeos-image-archive/firmware-nissa-15217.B-branch/R109-15217.916.0-1-8714961020606258097/nissa/firmware_from_source.tar.bz2",
					[]string{"glassway_hdmi_ufs/ec.bin" /* exists */, "glassway/ec.bin" /* exists */, "nissa/ec.bin", "ec.bin"},
				},
			},
		},
	}
	for _, testCase := range testCases {
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
		compareCandiates(t, testCase.inputGsUrl, "AP", actualCandidates, testCase.expectedAPCandidates)
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
		compareCandiates(t, testCase.inputGsUrl, "EC", actualCandidates, testCase.expectedECCandidates)
	}
}

func compareCandiates(t *testing.T, inputUrl, imageType string, actualCandidates, expectedCandidates []ImageCandidate) {
	for i, expected := range expectedCandidates {
		if len(actualCandidates) <= i {
			t.Errorf("[%s %s %d]: Missing candidate %v", inputUrl, imageType, i, expected)
			continue
		}
		actual := actualCandidates[i]
		if expected.GSURL != actual.GSURL {
			t.Errorf("[%s %s %d]: Incorrect GSURL, got %q, want %q", inputUrl, imageType, i, actual.GSURL, expected.GSURL)
		}
		for f, expectedFilename := range expected.Filenames {
			if len(actual.Filenames) <= f {
				t.Errorf("[%s %s %d.%d]: Missing filename %v", inputUrl, imageType, i, f, expectedFilename)
				continue
			}
			actualFilename := actual.Filenames[f]
			if actualFilename != expectedFilename {
				t.Errorf("[%s %s %d.%d]: Incorrect filename, got %q, want %q", inputUrl, imageType, i, f, actualFilename, expectedFilename)
			}
		}
		for f := len(expected.Filenames); f < len(actual.Filenames); f += 1 {
			t.Errorf("[%s %s %d.%d]: Extra filename %v", inputUrl, imageType, i, f, actual.Filenames[f])
		}
	}
	for i := len(expectedCandidates); i < len(actualCandidates); i += 1 {
		t.Errorf("[%s %s %d]: Extra candidate %v", inputUrl, imageType, i, actualCandidates[i])
	}
}

func TestTitleCase(t *testing.T) {
	for _, ts := range []struct {
		input, expected string
	}{
		{"word", "Word"},
		{"one_two_three", "One_Two_Three"},
		{"hello_there_2a_world", "Hello_There_2A_World"},
	} {
		titled := TitleCase(ts.input)
		if titled != ts.expected {
			t.Errorf("Incorrect title: got %q, want %q", titled, ts.expected)
		}
	}
}
