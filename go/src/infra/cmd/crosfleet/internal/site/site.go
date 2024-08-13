// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package site contains site local constants for the crosfleet tool.
package site

import (
	"fmt"

	"go.chromium.org/luci/auth"
	buildbucket_pb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/api/gitiles"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/hardcoded/chromeinfra"
)

// Environment contains environment specific values.
type Environment struct {
	SwarmingService string
	UFSService      string

	// Buildbucket-specific values.
	BuildbucketService string
	DefaultCTPBuilder  *buildbucket_pb.BuilderID
	DUTLeaserBuilder   *buildbucket_pb.BuilderID
}

// Prod is the environment for prod.
var Prod = Environment{
	SwarmingService: "chromeos-swarming.appspot.com",
	UFSService:      "ufs.api.cr.dev",

	BuildbucketService: "cr-buildbucket.appspot.com",
	DefaultCTPBuilder: &buildbucket_pb.BuilderID{
		Project: "chromeos",
		Bucket:  "testplatform",
		Builder: "cros_test_platform",
	},
	DUTLeaserBuilder: &buildbucket_pb.BuilderID{
		Project: "chromeos",
		Bucket:  "test_runner",
		Builder: "dut_leaser",
	},
}

// Dev is the environment for dev.
var Dev = Environment{
	SwarmingService: "chromeos-swarming.appspot.com",
	UFSService:      "staging.ufs.api.cr.dev",

	BuildbucketService: "cr-buildbucket.appspot.com",
	DefaultCTPBuilder: &buildbucket_pb.BuilderID{
		Project: "chromeos",
		Bucket:  "testplatform",
		Builder: "cros_test_platform-dev",
	},
	DUTLeaserBuilder: &buildbucket_pb.BuilderID{
		Project: "chromeos",
		Bucket:  "test_runner",
		Builder: "dut_leaser",
	},
}

// DefaultAuthOptions is an auth.Options struct prefilled with chrome-infra
// defaults.
var DefaultAuthOptions auth.Options

func init() {
	DefaultAuthOptions = chromeinfra.DefaultAuthOptions()
	DefaultAuthOptions.Scopes = []string{auth.OAuthScopeEmail, gitiles.OAuthScope}
}

// VersionNumber is the service version number for the crosfleet tool.
const VersionNumber = 4

// DefaultPRPCOptions is used for PRPC clients.  If it is nil, the
// default value is used.  See prpc.Options for details.
//
// This is provided so it can be overridden for testing.
var DefaultPRPCOptions = prpcOptionWithUserAgent(fmt.Sprintf("crosfleet/%d", VersionNumber))

// prpcOptionWithUserAgent create prpc option with custom UserAgent.
//
// DefaultOptions provides Retry ability in case we have issue with service.
func prpcOptionWithUserAgent(userAgent string) *prpc.Options {
	options := *prpc.DefaultOptions()
	options.UserAgent = userAgent
	return &options
}
