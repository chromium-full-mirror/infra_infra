// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/server/secrets"
)

// GetSecret gets the active secret using the LUCI Secrets package.
//
// The secret can be a Google Secret Manager managed secret with proper scheme
// prefix, or a plain text.
// If the secret is prefixed with `devsecret`, it must be a base64 encoded
// string. It should not have padding and be in raw encoded form.
func GetSecret(ctx context.Context, secret string) (string, error) {
	if isPlainTextSecret(secret) {
		return secret, nil
	}
	s, err := secrets.StoredSecret(ctx, secret)
	if err != nil {
		logging.Errorf(ctx, "GetSecret: failed to get secret %s: %s", secret, err)
		return "", err
	}
	return string(s.Active), nil
}

func isPlainTextSecret(secret string) bool {
	if strings.HasPrefix(secret, "sm://") {
		return false
	}
	if strings.HasPrefix(secret, "devsecret://") {
		return false
	}
	if strings.HasPrefix(secret, "devsecret-text://") {
		return false
	}
	return true
}

// GetEnvVar tries to get the corresponding environment variable for a string.
func GetEnvVar(ctx context.Context, k string) (string, error) {
	v := os.Getenv(k)
	if v == "" {
		return "", fmt.Errorf("GetEnvVar: %s environment variable not set", k)
	}
	return v, nil
}
