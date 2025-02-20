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

	"infra/cros/stableversion/validateconfig"
)

const DONTCARE = "f7e8bdf6-f67c-4d63-aea3-46fa5e980403"

// NOERROR is used when inspecting error messages. It only matches a nil error value.
const NOERROR = "NO-ERROR--ca5fc27a-4353-478c-bda2-c20519a2e0ff"

// ANYERROR is used to match any non-nil error.
const ANYERROR = "ANY-ERROR--4430e445-67c1-46a7-90b9-fad144490b5d"

var testVerifyCrosImageExistsData = []struct {
	uuid        string
	buildTarget string
	crosVersion string
	out         *map[string]bool
}{
	{
		"f959c762-214e-4293-b655-032cd791a85f",
		"test-target",
		"R81-12835.0.0",
		&map[string]bool{
			"gs://chromeos-image-archive/test-target-release/R81-12835.0.0/chromiumos_test_image.tar.xz": true,
		},
	},
}

func TestVerifyCrosImageExists(t *testing.T) {
	t.Parallel()
	for _, tt := range testVerifyCrosImageExistsData {
		t.Run(tt.uuid, func(t *testing.T) {
			var r Reader
			r.exst = makeConstantExistenceChecker()
			e := r.verifyCrosImageExists(context.Background(), tt.buildTarget, DONTCARE, tt.crosVersion)
			if e != nil {
				msg := fmt.Sprintf("uuid (%s): unexpected error (%s)", tt.uuid, e.Error())
				t.Error(msg)
			}
			diff := cmp.Diff(tt.out, r.cache)
			if diff != "" {
				msg := fmt.Sprintf("uuid (%s): unexpected diff (%s)", tt.uuid, diff)
				t.Error(msg)
			}
		})
	}
}

var testValidateConfigData = []struct {
	name          string
	uuid          string
	in            string
	out           string
	errorFragment string
}{
	{
		"empty",
		"b2b7aa51-3c2b-4c3f-93bf-0b26e6483489",
		`{}`,
		`{
			"missing_boards": null,
			"failed_to_lookup": null,
			"invalid_versions": null
		}`,
		NOERROR,
	},
	{
		"two present boards",
		"f63476d1-0382-4098-8a15-18fcb1a2e61a",
		`{
			"cros": [
				{
					"key": {
						"buildTarget": {"name": "nami"},
						"modelId": {"value": "akali360"}
					},
					"version": "R81-12835.0.0"
				},
				{
					"key": {
						"buildTarget": {"name": "nami"},
						"modelId": {"value": "sona"}
					},
					"version": "R81-12835.0.0"
				}
			],
			"firmware": [
				{
					"key": {
						"modelId": {"value": "sona"},
						"buildTarget": {"name": "nami"}
					},
					"version": "Google_Nami.42.43.44"
				},
				{
					"key": {
						"modelId": {"value": "akali360"},
						"buildTarget": {"name": "nami"}
					},
					"version": "Google_Nami.52.53.54"
				}
			]
		}`,
		`{
			"missing_boards": null,
			"failed_to_lookup": null,
			"invalid_versions": null
		}`,
		NOERROR,
	},
	{
		"two present boards with specific CrOS entries",
		"f63476d1-0382-4098-8a15-18fcb1a2e61a",
		`{
			"cros": [
				{
					"key": {
						"buildTarget": {"name": "nami"},
						"modelId": {"value": "sona"}
					},
					"version": "R81-12835.0.0"
				},
				{
					"key": {
						"buildTarget": {"name": "nami"},
						"modelId": {"value": "akali360"}
					},
					"version": "R81-12835.0.0"
				}
			],
			"firmware": [
				{
					"key": {
						"modelId": {"value": "sona"},
						"buildTarget": {"name": "nami"}
					},
					"version": "Google_Nami.42.43.44"
				},
				{
					"key": {
						"modelId": {"value": "akali360"},
						"buildTarget": {"name": "nami"}
					},
					"version": "Google_Nami.52.53.54"
				}
			]
		}`,
		`{
			"missing_boards": null,
			"failed_to_lookup": null,
			"invalid_versions": null
		}`,
		NOERROR,
	},
	{
		"one nonexistent chrome os version",
		"fab84b16-288d-44f0-b489-3712f8c14ad3",
		`{
			"cros": [
				{
					"key": {
						"buildTarget": {"name": "nonexistent-build-target"},
						"modelId": {}
					},
					"version": "R81-12835.0.0"
				}
			]
		}`,
		`{
			"missing_boards": ["nonexistent-build-target"],
			"failed_to_lookup": null,
			"invalid_versions": null
		}`,
		NOERROR,
	},
	{
		"invalid Chrome OS version in config file SHOULD PASS",
		"e188b8d4-6c2a-4fc1-b525-70144e0d8148",
		`{
			"cros": [
				{
					"key": {
						"buildTarget": {"name": "nami"},
						"modelId": {}
					},
					"version": "xxx-fake-version"
				}
			]
		}`,
		`null`,
		NOERROR,
	},
}

func TestValidateConfig(t *testing.T) {
	t.Parallel()
	for _, tt := range testValidateConfigData {
		t.Run(tt.name, func(t *testing.T) {
			var r Reader
			bg := context.Background()
			r.dld = makeConstantDownloader(DONTCARE)
			r.exst = makeConstantExistenceChecker()
			sv := parseStableVersionsOrPanic(tt.in)
			expected := parseResultsOrPanic(tt.out)
			result, e := r.ValidateConfig(bg, sv)
			if err := validateErrorContainsSubstring(e, tt.errorFragment); err != nil {
				t.Error(err.Error())
			}
			diff := cmp.Diff(expected, result)
			if diff != "" {
				msg := fmt.Sprintf("name (%s): uuid (%s): unexpected diff (%s)", tt.name, tt.uuid, diff)
				t.Error(msg)
			}
		})
	}
}

func TestNonLowercaseIsMalformed(t *testing.T) {
	cases := []struct {
		name         string
		fileContents string
		in           string
		out          string
	}{
		{
			"uppercase buildTarget in cros version",
			"",
			`{
				"cros": [
					{
						"key": {
							"buildTarget": {"name": "naMi"},
							"modelId": {}
						},
						"version": "xxx-fake-version"
					}
				]
			}`,
			`{
				"non_lowercase_entries": ["naMi"]
			}`,
		},
	}

	t.Parallel()

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			bg := context.Background()
			var out ValidationResult
			unmarshalOrPanic(tt.out, &out)

			var r Reader
			r.dld = makeConstantDownloader(tt.fileContents)

			in := parseStableVersionsOrPanic(tt.in)
			res, err := r.ValidateConfig(bg, in)
			if err != nil {
				t.Errorf("unexpected error %s", err)
			}
			if diff := cmp.Diff(&out, res); diff != "" {
				t.Errorf("comparison failure: %s", diff)
			}
		})
	}
}

func TestIsLowercase(t *testing.T) {
	cases := []struct {
		in  string
		out bool
	}{
		{
			"",
			true,
		},
		{
			"a",
			true,
		},
		{
			"A",
			false,
		},
		{
			"aA",
			false,
		},
	}

	t.Parallel()

	for _, tt := range cases {
		tt := tt
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			if isLowercase(tt.in) != tt.out {
				t.Errorf("isLowercase(%s) is unexpectedly %v", tt.in, tt.out)
			}
		})
	}
}

func testCombinedKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		board string
		model string
		out   string
	}{
		{
			"",
			"",
			"",
		},
		{
			"a",
			"",
			"a",
		},
		{
			"",
			"b",
			";b",
		},
		{
			"a",
			"b",
			"a;b",
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.out, func(t *testing.T) {
			t.Parallel()
			want := tt.out
			got := combinedKey(tt.board, tt.model)
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("diff (-want +got):\n%s", diff)
			}
		})
	}
}

func makeConstantDownloader(content string) downloader {
	return func(gsPath gs.Path) ([]byte, error) {
		return []byte(content), nil
	}
}

func makeConstantExistenceChecker() existenceChecker {
	return func(gsPath gs.Path) error {
		return nil
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

// parseStableVersionsOrPanic is a helper function that's used in tests to feed
// a result file contained in a string literal to a test.
func parseResultsOrPanic(content string) *ValidationResult {
	var out ValidationResult
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		panic(err.Error())
	}
	return &out
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
