// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package querygs

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	labPlatform "go.chromium.org/chromiumos/infra/proto/go/lab_platform"
	"go.chromium.org/luci/common/gcloud/gs"

	"go.chromium.org/infra/cros/stableversion/validateconfig"
)

const DONTCARE = "f7e8bdf6-f67c-4d63-aea3-46fa5e980403"

// NOERROR is used when inspecting error messages. It only matches a nil error value.
const NOERROR = "NO-ERROR--ca5fc27a-4353-478c-bda2-c20519a2e0ff"

// ANYERROR is used to match any non-nil error.
const ANYERROR = "ANY-ERROR--4430e445-67c1-46a7-90b9-fad144490b5d"

var testVerifyCrosImageExistsData = []struct {
	name      string
	key       string
	osVersion string
	out       map[string]bool
}{
	{
		"happy path",
		"board=board;model=model",
		"test-target-release/R81-12835.0.0",
		map[string]bool{
			"gs://chromeos-image-archive/test-target-release/R81-12835.0.0/chromiumos_test_image.tar.xz": true,
		},
	},
	{
		"happy path 2",
		"board=board;model=model",
		"test-target-release/R81-12835.0.0/my_image.tar.xz",
		map[string]bool{
			"gs://chromeos-image-archive/test-target-release/R81-12835.0.0/my_image.tar.xz": true,
		},
	},
}

func TestVerifyCrosImageExists(t *testing.T) {
	t.Parallel()
	for _, tt := range testVerifyCrosImageExistsData {
		t.Run(tt.name, func(t *testing.T) {
			var r Reader
			r.exst = func(gsPath gs.Path) error {
				if tt.out[string(gsPath)] {
					return nil
				}
				return fmt.Errorf("Unexpected path")
			}
			err := r.verifyCrosImageExists(context.Background(), tt.key, tt.osVersion)
			if err != nil {
				msg := fmt.Sprintf("TestVerifyCrosImageExists (%s): unexpected error (%s)", tt.name, err.Error())
				t.Error(msg)
			}
			diff := cmp.Diff(&tt.out, r.cache)
			if diff != "" {
				msg := fmt.Sprintf("TestVerifyCrosImageExists (%s): unexpected diff (%s)", tt.name, diff)
				t.Error(msg)
			}
		})
	}
}

var testValidateConfigData = []struct {
	name          string
	in            string
	errorFragment string
}{
	{
		"empty",
		`{}`,
		NOERROR,
	},
	{
		"two present boards",
		`{
			"versions": [
				{
					"target": {
						"deviceType": "cros",
						"board": "gale",
						"model": "gale"
					},
					"osVersion": "R92-13982.81.0",
					"osImagePath": "gale-test-ap-tryjob/R92-13982.81"
				},
				{
					"target": {
						"deviceType": "cros",
						"board": "nami",
						"model": "akali360"
					},
					"osVersion": "R81-12835.0.0",
					"osImagePath": "nami-release/R81-12835.0.0",
					"firmwareRoVersion":"Google_Nami.42.43.44"
				}
			]
		}`,
		NOERROR,
	},
	{
		"two present boards with specific CrOS entries",
		`{
			"versions": [
				{
					"target": {
						"deviceType": "cros",
						"board": "nami",
						"model": "sona"
					},
					"osVersion": "R81-12835.0.0",
					"osImagePath": "nami-release/R81-12835.0.0",
					"firmwareRoVersion":"Google_Nami.42.43.44"
				},
				{
					"target": {
						"deviceType": "cros",
						"board": "nami",
						"model": "akali360"
					},
					"osVersion": "R81-12835.0.0",
					"osImagePath": "nami-release/R81-12835.0.0",
					"firmwareRoVersion":"Google_Nami.52.53.54"
				}
			]
		}`,
		NOERROR,
	},
	{
		"one nonexistent chrome os version",
		`{
			"versions": [{
				"target": {
					"deviceType": "cros",
					"board": "nami",
					"model": "nonexistent-model"
				},
				"osVersion": "R81-12835.0.0",
				"osImagePath": "nami-release/R81-12835.0.0"
			}]
		}`,
		NOERROR,
	},
}

func TestValidateConfig(t *testing.T) {
	t.Parallel()
	for _, tt := range testValidateConfigData {
		t.Run(tt.name, func(t *testing.T) {
			var r Reader
			bg := context.Background()
			// All paths are valid.
			r.exst = func(gsPath gs.Path) error {
				return nil
			}
			sv := parseStableVersionsOrPanic(tt.in)
			e := r.ValidateConfig(bg, sv.GetVersions())
			if err := validateErrorContainsSubstring(e, tt.errorFragment); err != nil {
				t.Error(err.Error())
			}
		})
	}
}

// parseStableVersionsOrPanic is a helper function that's used in tests to feed
// a stable version file contained in a string literal to a test.
func parseStableVersionsOrPanic(content string) *labPlatform.StableVersions {
	out, err := validateconfig.ParseStableVersions([]byte(content))
	if err != nil {
		panic(err.Error())
	}
	return out
}

// validateErrorContainsSubstring checks whether an error matches a string provided in a table-driven test
func validateErrorContainsSubstring(e error, msg string) error {
	if msg == "" {
		panic("unexpected empty string in validateError function")
	}
	if e == nil {
		switch msg {
		case NOERROR:
			return nil
		case ANYERROR:
			return fmt.Errorf("expected error to be non-nil, but it wasn't")
		default:
			return fmt.Errorf("expected error to contain (%s), but it was nil", msg)
		}
	}
	switch msg {
	case NOERROR:
		return fmt.Errorf("expected error to be nil, but it was (%s)", e.Error())
	case ANYERROR:
		return nil
	default:
		if strings.Contains(e.Error(), msg) {
			return nil
		}
		return fmt.Errorf("expected error (%s) to contain (%s), but it did not", e.Error(), msg)
	}
}

func unmarshalOrPanic(content string, dest interface{}) {
	if err := json.Unmarshal([]byte(content), dest); err != nil {
		panic(err.Error())
	}
}

func validateMatches(pattern string, s string) error {
	b, err := regexp.MatchString(pattern, s)
	if err != nil {
		return err
	}
	if b {
		return nil
	}
	return fmt.Errorf("no part of string %q matches pattern %q", s, pattern)
}

func errorToString(e error) string {
	if e == nil {
		return ""
	}
	return e.Error()
}
