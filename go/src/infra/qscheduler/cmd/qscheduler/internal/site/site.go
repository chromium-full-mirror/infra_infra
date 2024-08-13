// Copyright 2018 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package site contains site local constants for the qscheduler tool.
package site

import (
	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/hardcoded/chromeinfra"
)

// Environment contains environment specific values.
type Environment struct {
	QSchedulerHost string
}

// Prod is the environment for prod.
var Prod = Environment{
	QSchedulerHost: "qscheduler-prod.chromium.org",
}

// Dev is the environment for dev.
var Dev = Environment{
	QSchedulerHost: "qscheduler-dev.chromium.org",
}

// DefaultAuthOptions is an auth.Options struct prefilled with chrome-infra
// defaults.
var DefaultAuthOptions auth.Options

func init() {
	DefaultAuthOptions = chromeinfra.DefaultAuthOptions()
	DefaultAuthOptions.Scopes = []string{auth.OAuthScopeEmail}
}

// DefaultPRPCOptions is used for PRPC clients.  If it is nil, the
// default value is used.  See prpc.Options for details.
//
// This is provided so it can be overridden for testing.
var DefaultPRPCOptions *prpc.Options
