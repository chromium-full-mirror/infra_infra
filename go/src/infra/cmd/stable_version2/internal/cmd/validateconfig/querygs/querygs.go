// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package querygs

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golang/protobuf/jsonpb"

	labPlatform "go.chromium.org/chromiumos/infra/proto/go/lab_platform"
	"go.chromium.org/luci/common/gcloud/gs"

	gslib "go.chromium.org/infra/cmd/stable_version2/internal/gs"
	"go.chromium.org/infra/cros/stableversion"
)

type existenceChecker func(gsPath gs.Path) error

// Reader reads chromiumos_test_image.tar.xz files from google storage and caches the result.
type Reader struct {
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

// ValidateConfig takes a stable version protobuf and attempts to validate every entry.
func (r *Reader) ValidateConfig(ctx context.Context, versions []*labPlatform.StableVersion) error {
	validPathes := map[string]bool{}
	// use the CrOS keys in the sv file to seed the reader.
	for _, item := range versions {
		if !stableversion.DeviceTypeForValidations[item.GetTarget().GetDeviceType()] {
			continue
		}
		key := stableversion.TargetToKey(item)
		if !validPathes[item.GetOsImagePath()] {
			if err := r.verifyCrosImageExists(ctx, key.String(), item.GetOsImagePath()); err != nil {
				return fmt.Errorf("cros version path is not valid: %w", err)
			}
			validPathes[item.GetOsImagePath()] = true
		}
		if imagePath := item.GetFirmwareRoImagePath(); imagePath != "" {
			if !validPathes[item.GetFirmwareRoImagePath()] {
				if _, err := r.validateFirmwarePath(ctx, key.String(), imagePath); err != nil {
					return fmt.Errorf("firmware path is not valid: %w", err)
				}
				validPathes[item.GetFirmwareRoImagePath()] = true
			}
		}
	}
	return nil
}

// Checks whether a CrOS image is able to be found for a given buildTarget (board) and OS version.
func (r *Reader) verifyCrosImageExists(ctx context.Context, key, path string) error {
	if r.cache == nil {
		v := make(map[string]bool)
		r.cache = &v
	}
	if !strings.HasSuffix(path, ".tar.xz") {
		path += "/chromiumos_test_image.tar.xz"
	}

	rawRemotePath := fmt.Sprintf("gs://chromeos-image-archive/%s", path)
	remotePath := gs.Path(rawRemotePath)

	if err := r.RemoteFileExists(remotePath); err != nil {
		return fmt.Errorf("cros image for key=%q, is not found by %q: %w", key, rawRemotePath, err)
	}

	(*r.cache)[rawRemotePath] = true
	return nil
}
