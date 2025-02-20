// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package preppers contains preppers for configured containers.
package preppers

import (
	"context"
	"os"
	"path"

	"google.golang.org/api/option"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

// AdbBase implements the prepper for adb-base container.
func AdbBase(ctx context.Context, dir string) error {
	downloadOptions := []option.ClientOption{}
	credentialFile := common.VMLabDockerKeyFileLocation
	if _, err := os.Stat(credentialFile); err == nil {
		downloadOptions = append(downloadOptions, option.WithCredentialsFile(credentialFile))
	}

	fileNames := map[string]string{
		"aapt":  "gs://chromeos-arc-images/builds/git_udc_release-static_sdk_tools/9594652/aapt",
		"aapt2": "gs://chromeos-arc-images/builds/git_udc_release-static_sdk_tools/9594652/aapt2",
		"adb":   "gs://chromeos-arc-images/builds/git_udc_release-static_sdk_tools/9594652/adb",
	}
	for lp, gp := range fileNames {
		localFilePath := path.Join(dir, lp)
		logging.Infof(ctx, "prepper: downloading %q", gp)
		if err := common.DownloadGcsFileAsLocalFile(ctx, gp, localFilePath, downloadOptions...); err != nil {
			return errors.Annotate(err, "prepper: download %q", gp).Err()
		}
		logging.Infof(ctx, "prepper: downloaded as %q", localFilePath)
	}
	return nil
}
