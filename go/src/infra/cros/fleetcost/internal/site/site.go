// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package site

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/hardcoded/chromeinfra"
)

// ShortRPCDuration is the duration for short RPCs.
const ShortRPCDuration = 30 * time.Second

// ErrShortRPCEnded is used when the ShortRPCDuration deadline has ended.
var ErrShortRPCEnded = errors.New("short RPC ended")

// MediumRPCDuration is the duration for medium RPCs that can touch many records.
const MediumRPCDuration = 3 * time.Minute

// ErrMediumRPCEnded is used when the MediumRPCDuration deadline has ended.
var ErrMediumRPCEnded = errors.New("medium RPC ended")

// progName is the name of the program
const progName = "fleetcost"

// DefaultAuthOptions is an auth.Options struct prefilled with chrome-infra
// defaults.
var DefaultAuthOptions = chromeinfra.SetDefaultAuthOptions(auth.Options{
	Scopes:     GetAuthScopes(DefaultAuthScopes),
	SecretsDir: SecretsDir(),
})

// GetAuthScopes get environment scopes if set
// Otherwise, return default scopes
func GetAuthScopes(defaultScopes []string) []string {
	e := os.Getenv("OAUTH_SCOPES")
	if e != "" {
		return strings.Split(e, "|")
	}
	return defaultScopes
}

// SecretsDir customizes the location for auth-related secrets.
func SecretsDir() string {
	configDir := os.Getenv("XDG_CACHE_HOME")
	if configDir == "" {
		configDir = filepath.Join(os.Getenv("HOME"), ".cache")
	}
	return filepath.Join(configDir, progName, "auth")
}

// DefaultAuthScopes is the default scopes for shivas login
var DefaultAuthScopes = []string{auth.OAuthScopeEmail, "https://www.googleapis.com/auth/spreadsheets"}
