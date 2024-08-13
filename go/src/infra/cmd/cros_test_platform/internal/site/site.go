// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package site contains functions and constants related to execution of this
// tool in specific environments (e.g., developer workstation vs buildbucket
// build)
package site

import (
	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/api/gitiles"
	"go.chromium.org/luci/common/gcloud/gs"
	"go.chromium.org/luci/hardcoded/chromeinfra"
)

// DefaultAuthOptions is an auth.Options struct prefilled with command-wide
// defaults.
//
// These defaults support invocation of the command in developer environments.
// The recipe invodation in a BuildBucket should override these defaults.
var DefaultAuthOptions auth.Options

func init() {
	DefaultAuthOptions = chromeinfra.DefaultAuthOptions()
	DefaultAuthOptions.Scopes = append(gs.ReadOnlyScopes, gitiles.OAuthScope, auth.OAuthScopeEmail)
}
