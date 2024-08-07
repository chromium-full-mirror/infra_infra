// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package site

import (
	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/gcloud/gs"
	"go.chromium.org/luci/hardcoded/chromeinfra"
)

// DefaultAuthOptions is an auth.Options struct prefilled with chrome-infra
// defaults.
var DefaultAuthOptions auth.Options

func init() {
	DefaultAuthOptions = chromeinfra.DefaultAuthOptions()
	DefaultAuthOptions.Scopes = append(gs.ReadWriteScopes, auth.OAuthScopeEmail)
}
