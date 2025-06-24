// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
//go:build linux
// +build linux

package atutil

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"go.chromium.org/infra/cros/internal/assert"
)

// Matches the sample host info file at test_data/host_info.json.
var testHostInfo = &HostInfo{
	Attributes: map[string]string{
		"HWID":          "sample-hwid",
		"job_repo_url":  "foo://bar.baz",
		"serial_number": "sample-servo-number",
	},
	Labels: []string{
		"wifi_chip:wireless_cc8d134f",
		"cros-version:foo",
		"pool:quota",
	},
	SerializerVersion: 1,
	StableVersions:    map[string]string{},
}

func TestSetCrosVersion(t *testing.T) {
	wantHostInfo := &HostInfo{
		Labels: []string{
			"wifi_chip:wireless_cc8d134f",
			"pool:quota",
			"cros-version:bar",
		},
	}
	info := &HostInfo{
		Labels: []string{
			"wifi_chip:wireless_cc8d134f",
			"cros-version:foo",
			"pool:quota",
		},
	}
	info.setCrosVersion("bar")
	if diff := cmp.Diff(wantHostInfo, info); diff != "" {

	}
}

func TestReadHostInfoFile(t *testing.T) {
	wantHostInfo := testHostInfo
	gotHostInfo, err := readHostInfoFile("test_data/host_info.json")
	assert.NilError(t, err)
	if diff := cmp.Diff(wantHostInfo, gotHostInfo); diff != "" {
		t.Fatalf("unexpected diff (%s)", diff)
	}
}

func TestWriteHostInfoFile(t *testing.T) {
	tmpDir := "hostinfo_tmp_dir"
	tmpDir, err := os.MkdirTemp("", tmpDir)
	defer os.RemoveAll(tmpDir)
	assert.NilError(t, err)
	tmpPath := filepath.Join(tmpDir, "host_info.json")
	err = os.WriteFile(tmpPath, []byte{}, 0644)
	assert.NilError(t, err)

	wantData, err := os.ReadFile("test_data/host_info.json")
	assert.NilError(t, err)
	assert.NilError(t, writeHostInfoFile(tmpPath, testHostInfo))
	gotData, err := os.ReadFile(tmpPath)
	assert.NilError(t, err)
	if diff := cmp.Diff(wantData, gotData); diff != "" {
		t.Fatalf("unexpected diff (%s)", diff)
	}
}

func TestProvisionURLToPkgStagingURL(t *testing.T) {
	rawProvisionURL := "http://devServerIP:9999/download/chromeos-image-archive/nami-release/R91-13894.0.0"
	wantPkgStagingURL := &url.URL{
		Scheme: "http",
		Host:   "devServerIP:8082",
		Path:   "static/nami-release/R91-13894.0.0/autotest/packages",
	}
	gotPkgStagingURL, err := convertToPkgStagingURL(
		rawProvisionURL, "nami-release/R91-13894.0.0")
	assert.NilError(t, err)
	diff := cmp.Diff(wantPkgStagingURL, gotPkgStagingURL)
	if diff != "" {
		t.Fatalf("unexpected diff (%s)", diff)
	}
}
