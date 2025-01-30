// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package site provides service account pathing.
package site

import (
	"os"
)

const (
	// ServiceAccountKeyPathEnv defines the Service account key path to be used by staging filter
	ServiceAccountKeyPathEnv = "SERVICE_ACCOUNT_KEY_PATH"
)

// ServiceAccountPath specifies the service account key path to be used by staging filter
func ServiceAccountPath() string {
	saPath := os.Getenv(ServiceAccountKeyPathEnv)
	return saPath
}
