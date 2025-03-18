// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package site

import (
	"fmt"
	"testing"
)

// TestGetFullyQualifiedHostname tests that we produce correct FQ hostnames when passed different satlab ids and hosts
func TestGetFullyQualifiedHostname(t *testing.T) {
	t.Parallel()

	type input struct {
		specifiedSatlabID string
		fetchedSatlabID   string
		prefix            string
		content           string
	}

	type test struct {
		name   string
		input  input
		output string
	}

	tests := []test{
		{"prepend fields", input{"", "abc", "satlab", "host"}, "satlab-abc-host"},
		{"prepend fields manual override", input{"def", "abc", "satlab", "host"}, "satlab-def-host"},
		{"dont prepend", input{"abc", "abc", "satlab", "satlab-abc-host"}, "satlab-abc-host"},
	}

	for i, tc := range tests {
		t.Run(fmt.Sprintf("TestFullyQualifiedHostname%d", i), func(t *testing.T) {
			t.Parallel()
			got := GetFullyQualifiedHostname(tc.input.specifiedSatlabID, tc.input.fetchedSatlabID, tc.input.prefix, tc.input.content)
			if got != tc.output {
				t.Errorf("got: %s, expected: %s for input %+v", got, tc.output, tc.input)
			}
		})
	}
}

// TestEnvFlags_GetNamespace ensures we properly respect the priority in which
// we should fetch namespace:
//  1. set via flag
//  2. set via env
//  3. default to OS
func TestEnvFlags_GetNamespace(t *testing.T) {
	tests := []struct {
		name          string
		flagNamespace string
		envNamespace  string
		wantNamespace string
	}{
		{
			name:          "no env, no flag",
			flagNamespace: "",
			envNamespace:  "",
			wantNamespace: DefaultNamespace,
		},
		{
			name:          "env, no flag",
			flagNamespace: "",
			envNamespace:  "env-ns",
			wantNamespace: "env-ns",
		},
		{
			name:          "no env, flag",
			flagNamespace: "flag-ns",
			envNamespace:  "",
			wantNamespace: "flag-ns",
		},
		{
			name:          "no env, no flag",
			flagNamespace: "flag-ns",
			envNamespace:  "env-ns",
			wantNamespace: "flag-ns",
		},
	}
	for _, tt := range tests {
		// t.Parallel() env var manipulation
		t.Run(tt.name, func(t *testing.T) {
			f := &EnvFlags{
				namespace: tt.flagNamespace,
			}
			t.Setenv(UFSNamespaceEnv, tt.envNamespace)
			if got := f.GetNamespace(); got != tt.wantNamespace {
				t.Errorf("EnvFlags.GetNamespace() = %v, want %v", got, tt.wantNamespace)
			}
		})
	}
}

// TestGetLUCIProject ensures we are properly using env vars/defaults.
func TestGetLUCIProject(t *testing.T) {
	tests := []struct {
		name        string
		envProject  string
		wantProject string
	}{
		{
			name:        "no env",
			envProject:  "",
			wantProject: DefaultLUCIProject,
		},
		{
			name:        "env",
			envProject:  "fake-project",
			wantProject: "fake-project",
		},
	}
	for _, tt := range tests {
		// t.Parallel() env var manipulation
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(LUCIProjectEnv, tt.envProject)
			if got := GetLUCIProject(); got != tt.wantProject {
				t.Errorf("GetLUCIProject() = %v, want %v", got, tt.wantProject)
			}
		})
	}
}

// TestGetDeployBucket ensures we properly use env var/defaults.
func TestGetDeployBucket(t *testing.T) {
	tests := []struct {
		name             string
		envDeployBucket  string
		wantDeployBucket string
	}{
		{
			name:             "no env",
			envDeployBucket:  "",
			wantDeployBucket: DefaultDeployBuilderBucket,
		},
		{
			name:             "env",
			envDeployBucket:  "fake-project",
			wantDeployBucket: "fake-project",
		},
	}
	for _, tt := range tests {
		// t.Parallel() env var manipulation
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(DeployBuilderBucketEnv, tt.envDeployBucket)
			if got := GetDeployBucket(); got != tt.wantDeployBucket {
				t.Errorf("GetDeployBucket() = %v, want %v", got, tt.wantDeployBucket)
			}
		})
	}
}

func TestGetGCSPartnerBucket(t *testing.T) {
	tests := []struct {
		name                 string
		isPartner            bool
		envGCSImageBucket    string
		envGCSPartnerBucket  string
		wantGCSPartnerBucket string
	}{
		{
			name:                 "No env, internal user",
			isPartner:            false,
			envGCSPartnerBucket:  "",
			wantGCSPartnerBucket: DefaultGCSPartnerBucket,
		},
		{
			name:                 "No env, partner user",
			isPartner:            true,
			envGCSImageBucket:    "chromeos-partner-bucket",
			envGCSPartnerBucket:  "",
			wantGCSPartnerBucket: "chromeos-partner-bucket",
		},
		{
			name:                 "env, internal user",
			isPartner:            false,
			envGCSPartnerBucket:  "chromeos-internal-bucket",
			wantGCSPartnerBucket: "chromeos-internal-bucket",
		},
		{
			name:                 "env, partner user",
			isPartner:            true,
			envGCSPartnerBucket:  "chromeos-partner-bucket",
			wantGCSPartnerBucket: "chromeos-partner-bucket",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(GCSPartnerBucketEnv, tt.envGCSPartnerBucket)
			t.Setenv(GCSImageBucketEnv, tt.envGCSImageBucket)
			if tt.isPartner {
				t.Setenv(UFSNamespaceEnv, "os-partner")
			}
			if got := GetGCSPartnerBucket(); got != tt.wantGCSPartnerBucket {
				t.Errorf("GetGCSPartnerBucket() = %v, want %v", got, tt.wantGCSPartnerBucket)
			}
		})
	}
}
