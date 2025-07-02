// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package site contains site local constants for the shivas
package site

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/tsmon"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/hardcoded/chromeinfra"

	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// Environment contains environment specific values.
type Environment struct {
	AdminService           string
	InventoryService       string
	UnifiedFleetService    string
	SwarmingService        string
	SwarmingServiceAccount string
	LogdogService          string
	QueenService           string
}

var TSMonFlags = tsmon.NewFlags()

// Prod is the environment for prod.
func Prod(isLocal bool) Environment {
	if isLocal {
		return localDevEnv
	}
	return prodEnv
}

// prodEnv is the prod environment.
var prodEnv = Environment{
	// TODO(gregorynisbet): remove once stable version moves to UFS.
	AdminService:     "chromeos-skylab-bot-fleet.appspot.com",
	InventoryService: "cros-lab-inventory.appspot.com",
	//TODO(eshwarn) : Change it to prod during release
	UnifiedFleetService: "ufs.api.cr.dev",
	SwarmingService:     "https://chromeos-swarming.appspot.com/",
	// TODO(crbug/1128496): remove when fixed
	SwarmingServiceAccount: "skylab-admin-task@chromeos-service-accounts.iam.gserviceaccount.com",
	LogdogService:          "luci-logdog.appspot.com",
	QueenService:           "drone-queen-prod.appspot.com",
}

// Dev is the environment for dev.
func Dev(isLocal bool) Environment {
	if isLocal {
		return localDevEnv
	}
	return devEnv
}

// localDevEnv is the local dev environment.
var localDevEnv = Environment{
	// TODO(gregorynisbet): remove once stable version moves to UFS.
	AdminService:        "skylab-staging-bot-fleet.appspot.com",
	InventoryService:    "0.0.0.0:8082",
	UnifiedFleetService: "127.0.0.1:8800",
	SwarmingService:     "https://chromium-swarm-dev.appspot.com/",
	// TODO(crbug/1128496): remove when fixed
	SwarmingServiceAccount: "skylab-admin-task@chromeos-service-accounts-dev.iam.gserviceaccount.com",
	LogdogService:          "luci-logdog-dev.appspot.com",
	QueenService:           "drone-queen-dev.appspot.com",
}

// devEnv is the environment for the dev cloud project.
var devEnv = Environment{
	// TODO(gregorynisbet): remove once stable version moves to UFS.
	AdminService:        "skylab-staging-bot-fleet.appspot.com",
	InventoryService:    "cros-lab-inventory-dev.appspot.com",
	UnifiedFleetService: "staging.ufs.api.cr.dev",
	SwarmingService:     "https://chromium-swarm-dev.appspot.com/",
	// TODO(crbug/1128496): remove when fixed
	SwarmingServiceAccount: "skylab-admin-task@chromeos-service-accounts-dev.iam.gserviceaccount.com",
	LogdogService:          "luci-logdog-dev.appspot.com",
	QueenService:           "drone-queen-dev.appspot.com",
}

// CommonFlags controls some commonly-used CLI flags.
type CommonFlags struct {
	verbose bool
}

// Register sets up the common flags.
func (f *CommonFlags) Register(fl *flag.FlagSet) {
	// Register the tsmon flags and set flush to manual
	TSMonFlags.Flush = tsmon.FlushManual
	TSMonFlags.Register(fl)
	fl.BoolVar(&f.verbose, "verbose", false, "log more details")
}

// Verbose returns if the command is set to verbose mode.
func (f *CommonFlags) Verbose() bool {
	return f.verbose
}

// OutputFlags controls output-related CLI flags.
type OutputFlags struct {
	json   bool
	tsv    bool
	full   bool
	noemit bool
}

// Register sets up the output flags.
func (f *OutputFlags) Register(fl *flag.FlagSet) {
	fl.BoolVar(&f.json, "json", false, "log output in json format")
	fl.BoolVar(&f.tsv, "tsv", false, "log output in tsv format (without title)")
	fl.BoolVar(&f.full, "full", false, "log full output in specified format, only works for GET command(latency is high). Users can also set os env SHIVAS_FULL_MODE to enable this.")
	fl.BoolVar(&f.noemit, "noemit", false, "specifies NOT to emit/print unpopulated fields in json format. Users can also set os env SHIVAS_NO_JSON_EMIT to enable this.")
}

// JSON returns if the output is logged in json format
func (f *OutputFlags) JSON() bool {
	return f.json
}

// Tsv returns if the output is logged in tsv format (without title)
func (f *OutputFlags) Tsv() bool {
	return f.tsv
}

// Full returns if the full format of output is logged in tsv format (without title)
func (f *OutputFlags) Full() bool {
	return f.full
}

// NoEmit returns if output json should NOT print/emit unpopulated fields
func (f *OutputFlags) NoEmit() bool {
	return f.noemit
}

// EnvFlags controls selection of the environment: either prod (default) or dev.
type EnvFlags struct {
	local     bool
	dev       bool
	namespace string
}

// Register sets up the -dev argument.
func (f *EnvFlags) Register(fl *flag.FlagSet) {
	fl.BoolVar(&f.local, "local", false, "Run locally (or use env var SHIVAS_ENV=LOCAL)")
	fl.BoolVar(&f.dev, "dev", false, "Run in dev environment (or use env var SHIVAS_ENV=DEV)")
	fl.StringVar(&f.namespace, "namespace", "", fmt.Sprintf("namespace where data resides. Users can also set os env SHIVAS_NAMESPACE. Valid namespaces: [%s]", strings.Join(ufsUtil.ValidClientNamespaceStr(), ", ")))

	shivasEnv := strings.ToLower(os.Getenv("SHIVAS_ENV"))
	f.local = shivasEnv == "local"
	f.dev = shivasEnv == "dev"
}

// Function validate checks the flags for correctness.
//
// For example, -local and -dev are mutually exclusive.
func (f *EnvFlags) validate() error {
	devlikeFlags := 0
	if f.local {
		devlikeFlags++
	}
	if f.dev {
		devlikeFlags++
	}
	if devlikeFlags > 1 {
		return errors.New("exactly one of -dev and -local may be specified")
	}
	return nil
}

// Env returns the environment, either dev or prod.
func (f EnvFlags) Env() Environment {
	if err := f.validate(); err != nil {
		panic(err.Error())
	}
	if f.local {
		return Dev(true)
	}
	if f.dev {
		return Dev(false)
	}
	return Prod(false)
}

// Namespace returns the namespace and validates the namespace is:
// 1) nonempty
// 2) a namespace that UFS expects
// 3) a namespace the command expects
//
// Supports default namespaces if defaultNS != "". These will be returned if
// no value is passed via flag or env var
func (f EnvFlags) Namespace(validNSList []string, defaultNS string) (string, error) {
	ns := strings.ToLower(f.namespace)
	if ns == "" {
		ns = strings.ToLower(os.Getenv("SHIVAS_NAMESPACE"))
	}
	// If ns is empty, user did not pass in namespace via flag or env var.
	if ns == "" {
		// no default + no input = error
		if defaultNS == "" {
			return ns, errors.New(fmt.Sprintf("namespace is a required field. Users can also set os env SHIVAS_NAMESPACE. Valid namespaces: [%s]", strings.Join(ufsUtil.ValidClientNamespaceStr(), ", ")))
		}
		ns = defaultNS
	}
	// This is a separate check from `IsClientNamespace` to ensure that
	// we catch if `validNSList` contains strings UFS does not expect.
	// If validNSList is empty, we ignore it
	if validNSList != nil && !Contains(validNSList, ns) {
		return ns, errors.New(fmt.Sprintf("namespace %s is invalid. Users can also set os env SHIVAS_NAMESPACE. Valid namespaces for this command: [%s]", ns, strings.Join(validNSList, ", ")))
	}
	// This catches a namespace which the local command thinks is valid, but
	// will be rejected by UFS.
	if !ufsUtil.IsClientNamespace(ns) {
		return ns, errors.New(fmt.Sprintf("namespace %s is invalid. Users can also set os env SHIVAS_NAMESPACE. Valid namespaces: [%s]", ns, strings.Join(ufsUtil.ValidClientNamespaceStr(), ", ")))
	}
	return ns, nil
}

var (
	// OSLikeNamespaces are namespaces that store primarily chromeos data.
	// This includes a separate partner namespace which stores chromeos data.
	OSLikeNamespaces = []string{ufsUtil.OSNamespace, ufsUtil.OSPartnerNamespace}
	// AllNamespaces contain all namespaces UFS considers valid.
	AllNamespaces = ufsUtil.ValidClientNamespaceStr()
)

// Contains determines if `arr` has the element `str` as one of it's elements
func Contains(arr []string, str string) bool {
	for _, s := range arr {
		if s == str {
			return true
		}
	}
	return false
}

// DefaultAuthScopes is the default scopes for shivas login
var DefaultAuthScopes = []string{auth.OAuthScopeEmail, "https://www.googleapis.com/auth/spreadsheets", "https://www.googleapis.com/auth/cloud-platform"}

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
	return filepath.Join(configDir, "shivas", "auth")
}

// VersionNumber is the version number for the tool. It follows the Semantic
// Versioning Specification (http://semver.org) and the format is:
// "MAJOR.MINOR.0+BUILD_TIME".
// We can ignore the PATCH part (i.e. it's always 0) to make the maintenance
// work easier.
// We can also print out the build time (e.g. 20060102150405) as the METADATA
// when show version to users.
var VersionNumber = fmt.Sprintf("%d.%d.%d", Major, Minor, Patch)

// Major is the Major version number
const Major = 7

// Minor is the Minor version number
const Minor = 2

// Patch is the PAtch version number
const Patch = 0

// DefaultPRPCOptions is used for PRPC clients.  If it is nil, the
// default value is used.  See prpc.Options for details.
//
// This is provided so it can be overridden for testing.
func DefaultPRPCOptions(e EnvFlags) *prpc.Options {
	if e.local {
		return &prpc.Options{
			Insecure:  true,
			UserAgent: fmt.Sprintf("shivas/%s", VersionNumber),
		}
	}
	return ProdDefaultPRPCOptions()
}

// ProdDefaultPRPCOptions returns UFS PRPC client options for the prod project
func ProdDefaultPRPCOptions() *prpc.Options {
	return prpcOptionWithUserAgent(fmt.Sprintf("%s/%s", "shivas", VersionNumber))
}

// CipdInstalledPath is the installed path for shivas package.
var CipdInstalledPath = "infra/shivas/"

// prpcOptionWithUserAgent create prpc option with custom UserAgent.
//
// DefaultOptions provides Retry ability in case we have issue with service.
func prpcOptionWithUserAgent(userAgent string) *prpc.Options {
	options := *prpc.DefaultOptions()
	options.UserAgent = userAgent
	return &options
}
