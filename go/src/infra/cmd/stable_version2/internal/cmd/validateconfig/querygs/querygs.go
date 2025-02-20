// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package querygs

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"unicode"

	"github.com/golang/protobuf/jsonpb"

	labPlatform "go.chromium.org/chromiumos/infra/proto/go/lab_platform"
	"go.chromium.org/luci/common/gcloud/gs"
	"go.chromium.org/luci/common/logging"

	gslib "go.chromium.org/infra/cmd/stable_version2/internal/gs"
)

// BoardModel is a combined build target and model. It is used for models that aren't present
// in any chromiumos_test_image.tar.xz file that we read.
type BoardModel struct {
	BuildTarget string `json:"build_target"`
	Model       string `json:"model"`
}

// VersionMismatch is a manifestation of a mismatched firmware version.
// For the purposes of analyzing the stable version file, we always assume
// that the CrOS version is correct and that the firmware version is potentially
// out of sync with it, rather than the other way around.
type VersionMismatch struct {
	BuildTarget string `json:"build_target"`
	Model       string `json:"model"`
	Wanted      string `json:"wanted"`
	Got         string `json:"got"`
}

// ValidationResult is a summary of the result of validating a stable version config file.
type ValidationResult struct {
	// Non-Lowercase entries
	NonLowercaseEntries []string `json:"non_lowercase_entries"`
	// MissingBoardModels are the board+models that don't have a metadata file in Google Storage.
	MissingBoardModels []*BoardModel `json:"missing_board_models"`
	// InvalidFaftVersions is the list of FAFT entries.
	InvalidFaftVersions []*VersionMismatch `json:"invalid_faft_versions"`
}

// AnomalyCount counts the total number of issues found in the results summary.
func (r *ValidationResult) AnomalyCount() int {
	return len(r.MissingBoardModels) + len(r.NonLowercaseEntries) + len(r.InvalidFaftVersions)
}

type downloader func(gsPath gs.Path) ([]byte, error)

type existenceChecker func(gsPath gs.Path) error

// Reader reads chromiumos_test_image.tar.xz files from google storage and caches the result.
type Reader struct {
	dld  downloader
	exst existenceChecker
	// GCS bucket URL > whether it exists or not
	cache *map[string]bool
}

// Init creates a new Google Storage Client.
// TODO(gregorynisbet): make it possible to initialize a test gsClient as well
func (r *Reader) Init(ctx context.Context, t http.RoundTripper, unmarshaler jsonpb.Unmarshaler, tempPrefix string) error {
	var gsc gslib.Client
	if err := gsc.Init(ctx, t, unmarshaler); err != nil {
		return fmt.Errorf("Reader::Init: %w", err)
	}
	r.dld = func(remotePath gs.Path) ([]byte, error) {
		dir, err := ioutil.TempDir("", tempPrefix)
		if err != nil {
			return nil, fmt.Errorf("download adapter: making temporary directory: %w", err)
		}
		defer os.RemoveAll(dir)
		localPath := filepath.Join(dir, "chromiumos_test_image.tar.xz")
		if err := gsc.Download(remotePath, localPath); err != nil {
			return nil, fmt.Errorf("download adapter: fetching file: %w", err)
		}
		contents, err := ioutil.ReadFile(localPath)
		if err != nil {
			return nil, fmt.Errorf("download adapter: reading local file: %w", err)
		}
		return contents, nil
	}
	r.exst = func(remotePath gs.Path) error {
		// Thoroughly check for existence by downloading a few bytes and discarding
		// the result.
		return gsc.DownloadByteRange(remotePath, os.DevNull, 0, 10)
	}
	return nil
}

// RemoteFileExists checks for the existence and nonzero size of a given path in Google Storage.
func (r *Reader) RemoteFileExists(remotePath gs.Path) error {
	return r.exst(remotePath)
}

// combinedKey combines a board and a model into a single key
// and returns just the board name when the model is "".
func combinedKey(board string, model string) string {
	if model == "" {
		return board
	}
	return fmt.Sprintf("%s;%s", board, model)
}

// ValidateConfig takes a stable version protobuf and attempts to validate every entry.
func (r *Reader) ValidateConfig(ctx context.Context, sv *labPlatform.StableVersions) (*ValidationResult, error) {
	var cfgCrosVersions = make(map[string]string, len(sv.GetCros()))
	var out ValidationResult
	if sv == nil {
		return nil, fmt.Errorf("Reader::ValidateConfig: config file cannot be nil")
	}
	// use the CrOS keys in the sv file to seed the reader.
	for _, item := range sv.GetCros() {
		bt := item.GetKey().GetBuildTarget().GetName()
		model := item.GetKey().GetModelId().GetValue()
		combined := combinedKey(bt, model)
		version := item.GetVersion()
		if !isLowercase(bt) {
			out.NonLowercaseEntries = append(out.NonLowercaseEntries, bt)
			continue
		}
		if !isLowercase(model) {
			out.NonLowercaseEntries = append(out.NonLowercaseEntries, model)
			continue
		}
		if err := r.verifyCrosImageExists(ctx, bt, model, version); err != nil {
			out.MissingBoardModels = append(out.MissingBoardModels, &BoardModel{bt, model})
			continue
		}
		cfgCrosVersions[combined] = version
	}
	// Confirm that all faft firmware bundles exist.
	for _, item := range sv.GetFaft() {
		bt := item.GetKey().GetBuildTarget().GetName()
		model := item.GetKey().GetModelId().GetValue()
		if path, err := r.validateFaft(ctx, item.GetVersion()); err != nil {
			out.InvalidFaftVersions = append(out.InvalidFaftVersions, &VersionMismatch{bt, model, path, ""})
		}
	}
	return &out, nil
}

// Checks whether a CrOS image is able to be found for a given buildTarget (board) and OS version.
func (r *Reader) verifyCrosImageExists(ctx context.Context, buildTarget string, model string, crosVersion string) error {
	if r.cache == nil {
		v := make(map[string]bool)
		r.cache = &v
	}

	rawRemotePath := crosImagePath(buildTarget, crosVersion)
	remotePath := gs.Path(rawRemotePath)

	if err := r.RemoteFileExists(remotePath); err != nil {
		logging.Errorf(ctx, "failed to get CrOS image from GCS for board=%q, model=%q, os=%q from path: %q",
			buildTarget, model, crosVersion, rawRemotePath)
		return fmt.Errorf("verifyCrosImageExists: checking file existence: %w", err)
	}

	(*r.cache)[rawRemotePath] = true
	return nil
}

// Generates the GCS path for a ChromeOS image.
func crosImagePath(buildTarget string, crosVersion string) string {
	return fmt.Sprintf("gs://chromeos-image-archive/%s-release/%s/chromiumos_test_image.tar.xz", buildTarget, crosVersion)
}

// check if string has entirely lowercase letters
func isLowercase(s string) bool {
	for _, ch := range s {
		if unicode.IsLetter(ch) {
			if unicode.IsUpper(ch) {
				return false
			}
		}
	}
	return true
}
