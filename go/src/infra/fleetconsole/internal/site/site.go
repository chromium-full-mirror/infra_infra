// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package site contains settings related to the fleet console project as a whole.
package site

import (
	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/hardcoded/chromeinfra"
)

// DefaultAuthOptions is an auth.Options struct prefilled with chrome-infra
// defaults.
var DefaultAuthOptions = chromeinfra.SetDefaultAuthOptions(auth.Options{
	Scopes: []string{auth.OAuthScopeEmail},
})
