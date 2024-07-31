// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package site contains site local constants for the paris tool.
package site

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"go.chromium.org/luci/auth"
	buildbucket_pb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/api/gitiles"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/hardcoded/chromeinfra"
)

// OAuth scope for the whole of cloud platform.
const CloudOAuthScope = "https://www.googleapis.com/auth/cloud-platform"

// Environment contains environment specific values.
type Environment struct {
	AdminService   string
	ServiceAccount string

	// UFS-specific values
	UFSService string
}

// BuildbucketBuilderInfo contains information for initializing a
// Buildbucket client that talks to a specific builder.
type BuildbucketBuilderInfo struct {
	Host      string
	BuilderID *buildbucket_pb.BuilderID
}

// Wrapped returns the environment wrapped in a helper type to satisfy
// the worker.Environment interface and swarming.Environment interface.
func (e Environment) Wrapped() EnvWrapper {
	return EnvWrapper{e: e}
}

// EnvWrapper wraps Environment to satisfy the worker.Environment
// interface and swarming.Environment interface.
type EnvWrapper struct {
	e Environment
}

// Prod is the environment for prod.
var Prod = Environment{
	AdminService:   "chromeos-skylab-bot-fleet.appspot.com",
	ServiceAccount: "skylab-admin-task@chromeos-service-accounts.iam.gserviceaccount.com",

	UFSService: "ufs.api.cr.dev",
}

// Dev is the environment for dev.
var Dev = Environment{
	AdminService:   "chromeos-skylab-bot-fleet.appspot.com",
	ServiceAccount: "skylab-admin-task@chromeos-service-accounts-dev.iam.gserviceaccount.com",

	UFSService: "staging.ufs.api.cr.dev",
}

// EnvFlags controls selection of the environment: either prod (default) or dev.
type EnvFlags struct {
	dev bool
}

// Register sets up the -dev argument.
func (f *EnvFlags) Register(fl *flag.FlagSet) {
	fl.BoolVar(&f.dev, "dev", false, "Run in dev environment.")
}

// Env returns the environment, either dev or prod.
func (f EnvFlags) Env() Environment {
	if f.dev {
		return Dev
	}
	return Prod
}

// DefaultAuthOptions is an auth.Options struct prefilled with chrome-infra
// defaults.
var DefaultAuthOptions = auth.Options{
	// Note that ClientSecret is not really a secret since it's hardcoded into
	// the source code (and binaries). It's totally fine, as long as it's callback
	// URI is configured to be 'localhost'. If someone decides to reuse such
	// ClientSecret they have to run something on user's local machine anyway
	// to get the refresh_token.
	ClientID:          "446450136466-mj75ourhccki9fffaq8bc1e50di315po.apps.googleusercontent.com",
	ClientSecret:      "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",
	LoginSessionsHost: chromeinfra.LoginSessionsHost,
	SecretsDir:        SecretsDir(),
	Scopes:            []string{auth.OAuthScopeEmail, gitiles.OAuthScope},
}

// EthernetHookCallbackOptions includes OAuth scopes that include, at minimum, the ability to read from Google Storage.
var EthernetHookCallbackOptions = func() auth.Options {
	o := DefaultAuthOptions
	o.Scopes = append(o.Scopes, CloudOAuthScope)
	return o
}()

// CommonFlags contains generic CLI flags that are theoretically common to all commands.
type CommonFlags struct {
	Verbose bool
}

// Register sets up the common flags.
func (f *CommonFlags) Register(fl *flag.FlagSet) {
	fl.BoolVar(&f.Verbose, "verbose", false, `set log level to "debug"`)
}

// VersionNumber is the version number for the tool. It follows the Semantic
// Versioning Specification (http://semver.org) and the format is:
// "MAJOR.MINOR.0+BUILD_TIME".
// We can ignore the PATCH part (i.e. it's always 0) to make the maintenance
// work easier.
// We can also print out the build time (e.g. 20060102150405) as the METADATA
// when show version to users.
const VersionNumber = "2.1.2"

// DefaultPRPCOptions is used for PRPC clients.  If it is nil, the
// default value is used.  See prpc.Options for details.
//
// This is provided so it can be overridden for testing.
var DefaultPRPCOptions = prpcOptionWithUserAgent(fmt.Sprintf("paris/%s", VersionNumber))

// UFSPRPCOptions is used for UFS PRPC clients.
var UFSPRPCOptions = prpcOptionWithUserAgent("paris/6.0.0")

// SecretsDir returns an absolute path to a directory (in $HOME) to keep secret
// files in (e.g. OAuth refresh tokens) or an empty string if $HOME can't be
// determined (happens in some degenerate cases, it just disables auth token
// cache).
func SecretsDir() string {
	configDir := os.Getenv("XDG_CACHE_HOME")
	if configDir == "" {
		configDir = filepath.Join(os.Getenv("HOME"), ".cache")
	}
	return filepath.Join(configDir, "paris", "auth")
}

// prpcOptionWithUserAgent create prpc option with custom UserAgent.
//
// DefaultOptions provides Retry ability in case we have issue with service.
func prpcOptionWithUserAgent(userAgent string) *prpc.Options {
	options := *prpc.DefaultOptions()
	options.UserAgent = userAgent
	return &options
}

// Client tag used for PARIS tasks scheduling.
const ClientTag = "client:paris"
