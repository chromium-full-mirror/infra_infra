// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"
	"fmt"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/server/auth"
)

// GetServiceAccountName returns the app's service account name.
func GetServiceAccountName(ctx context.Context) (string, error) {
	signer := auth.GetSigner(ctx)
	if signer == nil {
		return "", errors.New("failed to get the Signer instance representing the service")
	}
	info, err := signer.ServiceInfo(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get service info: %w", err)
	}
	return info.ServiceAccountName, nil
}
