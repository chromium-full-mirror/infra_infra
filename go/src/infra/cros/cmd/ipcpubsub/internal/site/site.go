// Copyright 2018 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package site contains site local constants for the qscheduler tool.
package site

import (
	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/hardcoded/chromeinfra"
)

// OAuthScopeCloudPlatform is a scope for using Google Cloud Platform
const OAuthScopeCloudPlatform = "https://www.googleapis.com/auth/cloud-platform"

// OAuthScopePubsub is a scope for using Google Cloud Pubsub
const OAuthScopePubsub = "https://www.googleapis.com/auth/pubsub"

// DefaultAuthOptions is an auth.Options struct prefilled with chrome-infra
// defaults.
//
// TODO(akeshet): This is copied from the Go swarming client.  We
// should probably get our own OAuth client credentials at some point.
var DefaultAuthOptions auth.Options

func init() {
	DefaultAuthOptions = chromeinfra.DefaultAuthOptions()
	DefaultAuthOptions.Scopes = []string{OAuthScopeCloudPlatform, OAuthScopePubsub}
}
