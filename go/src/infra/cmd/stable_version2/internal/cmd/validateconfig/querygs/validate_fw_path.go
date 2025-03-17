// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package querygs

import (
	"context"
	"fmt"
	"strings"

	"go.chromium.org/luci/common/gcloud/gs"
)

// validateFirmwarePath validates the remote firmware path to make sure
// that a firmware bundle actually exists there.
//
// See b:241150358 for an example of this issue happening.
// See b:241155320 for more information about this change itself.
// See b:286114085 for detail information
func (r *Reader) validateFirmwarePath(ctx context.Context, key, firmwarePath string) (validatePath string, _ error) {
	if strings.HasPrefix(firmwarePath, "gs://") {
		return "", fmt.Errorf("validate firmware path: path is not expected to have gs:// %q", firmwarePath)
	}
	//check if the fw image path has .tar.bz2 extension
	if !strings.HasSuffix(firmwarePath, ".tar.bz2") {
		firmwarePath += "/firmware_from_source.tar.bz2"
	}
	gsPath := fmt.Sprintf("gs://chromeos-image-archive/%s", firmwarePath)
	if err := r.exst(gs.Path(gsPath)); err != nil {
		return gsPath, fmt.Errorf("validate firmware for key=%q: path: %w", key, err)
	}
	return gsPath, nil
}
