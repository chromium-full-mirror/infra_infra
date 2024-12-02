// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package site contains site local constants for the satlab
package site

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/hardcoded/chromeinfra"

	shivassite "infra/cmd/shivas/site"
)

// AppPrefix is the prefix to use the satlab CLI.
var AppPrefix = "satlab"

// DevCrosAdmService is the dev CrOSSkylabAdmin service.
const DevCrosAdmService = "skylab-staging-bot-fleet.appspot.com"

// ProdCrosAdmService is the prod CrOSSkylabAdmin service.
const ProdCrosAdmService = "chromeos-skylab-bot-fleet.appspot.com"

// DevUFSService is the dev UFS service
const DevUFSService = "staging.ufs.api.cr.dev"

// ProdUFSService is the prod UFS service
const ProdUFSService = "ufs.api.cr.dev"

// Satlab is just the string "satlab". The literal string "satlab" is used in many
// places to create resource names.
const Satlab = "satlab"

const (
	// SatlabSAFilename is service account filename on user GCS bucket.
	SatlabSAFilename = "pubsub-key-do-not-delete.json"
	// SkylabDroneKeyFilename is filename of service account key to use by drone container
	SkylabDroneKeyFilename = "skylab-drone.json"
	// KeyFolder is where keys and config lives.
	KeyFolder = "/home/satlab/keys"
	// SatlabConfigFilename to be downloaed from GCS.
	SatlabConfigFilename = "satlab-config.json"
	BotoConfigFilename   = ".boto"
)

const RecoveryVersionDirectory = "/home/satlab/keys/recovery_versions/"

// RepairBuilderName is the var used to determine what Repair builder
// name will be used to schedule Repair builds.
const RepairBuilderName = "repair"

const (
	// CTPBuilderBucketEnv is the env var used to determine what bucket the
	// ctp task should run in.
	CTPBuilderBucketEnv = "CTP_BUILDER_BUCKET"
	// CTPBuilderNameEnv is the env var used to determine what CTP builder
	// name be used to schedule CTP builds.
	CTPBuilderNameEnv = "CTP_BUILDER_NAME"
	// DeployBuilderBucketEnv is the env var used to determine what bucket the
	// deploy task should run in.
	DeployBuilderBucketEnv = "DEPLOY_BUILDER_BUCKET"
	// GCSBucket is the bucket for staging images.
	// (in case of internal users staging is not needed)
	GCSImageBucketEnv = "GCS_IMAGE_BUCKET"
	// GCSBucket is the partner bucket for storing custom images, results and testplans.
	GCSPartnerBucketEnv = "GCS_PARTNER_BUCKET"
	// LUCIProjectEnv is the env var used to determine what LUCI project bb
	// tasks should run in.
	LUCIProjectEnv = "LUCI_PROJECT"
	// UFSNamespaceEnv is the env var used to determine what namespace should
	// be used to interface with UFS.
	UFSNamespaceEnv = "UFS_NAMESPACE"
	// UFSZoneEnv is the env var used to determine what zone should
	// be used to interface with UFS.
	UFSZoneEnv = "UFS_ZONE"
	// BotPrefix is the env var used to get bot information
	BotPrefix = "BOT_PREFIX"
	// BotoPath is the env var to get boto config file path
	BotoPath = "BOTO_PATH"

	// ServiceAccountKeyPathEnv defines the Service account key path to be used by
	// moblab api.
	ServiceAccountKeyPathEnv = "SERVICE_ACCOUNT_KEY_PATH"

	// Log file used for satlab-rpcserver
	RPCServerLogFileEnv = "SATLAB_RPCSERVER_LOGFILE"

	// DefaultLUCIProject is the LUCI project to specify if `LUCIProjectEnv` is
	// not present.
	DefaultLUCIProject = "chromeos"
	// DefaultDeployBuilderBucket is the bb bucket to specify if
	// `DeployBuilderBucketEnv` is not specified.
	DefaultDeployBuilderBucket = "labpack_runner"
	// DefaultCTPBuilderBucket is the bb bucket to specify if
	// `CTPBuilderBucketEnv` is not specified.
	DefaultCTPBuilderBucket = "testplatform"
	// DefaultCTPBuilderName is the bb bucket to specify if
	// `CTPBuilderNameEnv` is not specified.
	DefaultCTPBuilderName = "cros_test_platform"
	// DefaultGCSImageBucket is the GCS bucket to specify if
	// `GCSImageBucketEnv` is not specified.
	DefaultGCSImageBucket = "chromeos-image-archive"
	// DefaultGCSPartnerBucket is the GCS bucket to specify if
	// `GCSPartnerBucketEnv` is not specified.
	DefaultGCSPartnerBucket = "chromeos-satlab-internal-users"
	// DefaultNamespace is the default namespace to use for all operations.
	DefaultNamespace = "os"
	// DefaultZone is the default value for the zone command line flag.
	DefaultZone = "satlab"
	// DefaultServiceAccountKeyPathEnv is the default path for for service account.
	DefaultServiceAccountKeyPathEnv = "/home/satlab/keys/pubsub-key-do-not-delete.json"
	// SwarmingServiceHost is the host of swarming service address
	SwarmingServiceHost = "chromeos-swarming.appspot.com"

	DefaultRPCServerLogFile = "/var/log/satlab/satlab_rpcserver.log"
	// DefaultTestRunnerBuilderName is the  test_runner builder name
	DefaultTestRunnerBuilderName = "test_runner"
	// BotoAccessKeyId is the boto key of the boto config
	BotoAccessKeyId = "gs_access_key_id"
	// BotoSecretAccessKey is the boto secret key of the boto config
	BotoSecretAccessKey = "gs_secret_access_key"
)

const SatlabSetupInstruction = `
--------------------------------------------------------------------------------
IMPORTANT: Please read below information
--------------------------------------------------------------------------------
Setting up Satlab requires special privileges, you need access to
the Google Storage Bucket and to download artifacts.
This script will authenticate the user to access the required artifacts.

The following the are steps this script executes:

1. Asks for Google Storage Bucket name

  *  Google Partners have to provide the GS bucket information assigned to them.

  *  Satlab internal users(googlers), please use "chromeos-satlab-internal-users"
     as your GS bucket

2. Follow prompt to autheticate user.
3. Pulls in required artifacts once the user is verified.
4. Finally reboots the chromebox. (You must restart the chromebox inorder to
   finish the satlab setup)
--------------------------------------------------------------------------------
`

// Swarming task tags used to get task details.
const (
	BuildBucketIDTag          = "buildbucket_build_id"
	BuilderTag                = "builder"
	DisplayNameTag            = "display_name"
	DutNameTag                = "dut_name"
	ParentBuildBucketIDTag    = "parent_buildbucket_id"
	LabelPoolTag              = "label-pool"
	LabelSuiteTag             = "label-suite"
	SatlabIDTag               = "satlab-id"
	LabelBoardTag             = "label-board"
	LabelModelTag             = "label-model"
	LabelImageTag             = "label-image"
	Suite                     = "suite"
	TestPlan                  = "testplan"
	TestPlanIDTag             = "test-plan-id"
	TestTypeTag               = "test-type"
	BugIDTag                  = "bug_id"
	QualRunIDTag              = "qual_run_id"
	MiloSite                  = "http://ci.chromium.org/b/"
	TesthausURLTemplate       = "https://tests.chromeos.goog/p/chromeos/search?union=AND&viewMode=List&"
	BuildBucketID             = "buildbucket_id"
	TaskLinkTemplate          = "https://chromeos-swarming.appspot.com/task?id="
	DeployBuilderName         = "deploy"
	TaskRunningTest           = "Running"
	TaskRunningRepair         = "Repairing"
	TaskRunningDeploy         = "Deploying"
	TestRunnerSwarmingPoolEnv = "TEST_RUNNER_SWARMING_POOL"
	CTPSwarmingPoolEnv        = "CTP_SWARMING_POOL"
	CPConAccountIDEnv         = "CPCON_ACCOUNT_ID"
	CPConLinkTemplate         = "https://chromeos.google.com/partner/console/a/%s/pvsAvlDetails?runId=%s&issueId=%s"
)

// CommonFlags controls some commonly-used CLI flags.
type CommonFlags struct {
	Verbose  bool
	SatlabID string
}

// Register sets up the common flags.
func (f *CommonFlags) Register(fl *flag.FlagSet) {
	fl.BoolVar(&f.Verbose, "verbose", false, "whether to log verbosely")
	fl.StringVar(&f.SatlabID, "satlab-id", "", "the ID for the satlab in question")
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
	fl.BoolVar(&f.full, "full", false, "log full output in specified format")
	fl.BoolVar(&f.noemit, "noemit", false, "specifies NOT to emit/print unpopulated fields in json format.")
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
	dev       bool
	namespace string
}

// Register sets up the -dev argument.
func (f *EnvFlags) Register(fl *flag.FlagSet) {
	fl.BoolVar(&f.dev, "dev", false, "Run in dev environment.")
	fl.StringVar(&f.namespace, "namespace", "", "namespace where data resides.")
}

// GetNamespace determines the namespace, in descending priority, by
//  1. Specified in flag
//  2. Specified in environment
//  3. Default to `os`
func (f *EnvFlags) GetNamespace() string {
	return GetNamespace(f.namespace)
}

// GetNamespace determines the namespace, in descending priority, by
//  1. use parameter namespace.
//  2. Specified in environment
//  3. Default to `os`
func GetNamespace(namespace string) string {
	if namespace != "" {
		return namespace
	}
	ns := os.Getenv(UFSNamespaceEnv)
	if ns == "" {
		return DefaultNamespace
	}
	return ns

}

// IsPartner determines if user is internal or external. Returns true if external.
func IsPartner() bool {
	return os.Getenv(UFSNamespaceEnv) == "os-partner"
}

// GetCrosAdmService returns the hostname of the CrOSSkylabAdmin service that is
// appropriate for the given environment.
func (f *EnvFlags) GetCrosAdmService() string {
	if f.dev {
		return DevCrosAdmService
	}
	return ProdCrosAdmService
}

// GetUFSService returns the hostname of the UFS service appropriate for an environment
func (f *EnvFlags) GetUFSService() string {
	return GetUFSService(f.dev)
}

// GetUFSService returns the hostname of the UFS service by given a flag
func GetUFSService(dev bool) string {
	if dev {
		return DevUFSService
	}
	return ProdUFSService
}

// GetLUCIProject determines what LUCI project we expect any buildbucket tasks
// to run in, based on the environment.
func GetLUCIProject() string {
	project := os.Getenv(LUCIProjectEnv)
	if project == "" {
		return DefaultLUCIProject
	}
	return project
}

// GetDeployBucket determines which bucket we expect any deploy tasks
// to run in, based on the environment.
func GetDeployBucket() string {
	bucket := os.Getenv(DeployBuilderBucketEnv)
	if bucket == "" {
		return DefaultDeployBuilderBucket
	}
	return bucket
}

// GetCTPBucket determines which bucket we expect any ctp tasks
// to run in, based on the environment.
func GetCTPBucket() string {
	bucket := os.Getenv(CTPBuilderBucketEnv)
	if bucket == "" {
		return DefaultCTPBuilderBucket
	}
	return bucket
}

// GetCTPBuilder determines which builder we expect any ctp build
// to run in, based on the environment.
func GetCTPBuilder() string {
	builder := os.Getenv(CTPBuilderNameEnv)
	if builder == "" {
		return DefaultCTPBuilderName
	}
	return builder
}

// GetGCSImageBucket determines which Google storage image bucket
// to use, based on the environment.
func GetGCSImageBucket() string {
	imageBucket := os.Getenv(GCSImageBucketEnv)
	if imageBucket == "" {
		return DefaultGCSImageBucket
	}
	return imageBucket
}

// GetGCSPartnerBucket determines which Google storage image bucket
// to use, based on the environment.
func GetGCSPartnerBucket() string {
	partnerBucket := os.Getenv(GCSPartnerBucketEnv)
	if partnerBucket == "" {
		return DefaultGCSPartnerBucket
	}
	return partnerBucket
}

// GetUFSZone determines which ZONE the DUTs belongs to,
// based on the environment.
func GetUFSZone() string {
	zone := os.Getenv(UFSZoneEnv)
	if zone == "" {
		return DefaultZone
	}
	return zone
}

// GetServiceAccountPath specifies the service account key path
// to be used with moblab api
func GetServiceAccountPath() string {
	saPath := os.Getenv(ServiceAccountKeyPathEnv)
	if saPath == "" {
		return DefaultServiceAccountKeyPathEnv
	}
	return saPath
}

// MaybePrepend adds a prefix with a leading dash unless the string already
// begins with the prefix in question.
func MaybePrepend(prefix string, satlabID string, content string) string {
	if prefix == "" || satlabID == "" {
		return content
	}
	satlabPrefix := fmt.Sprintf("%s-%s", prefix, satlabID)
	if strings.HasPrefix(content, satlabPrefix) {
		return content
	}
	return fmt.Sprintf("%s-%s", satlabPrefix, content)
}

// GetFullyQualifiedHostname takes in primitives used to create the fully qualified hostname and produces that hostname
// specifiedSatlabID refers to a user specified ID which should take precedence over the fetchedSatlabID which should be fetched in an automated fashion
func GetFullyQualifiedHostname(specifiedSatlabID string, fetchedSatlabID, prefix string, content string) string {
	//uses either a user provided or automatically fetched Satlab ID to build the fully qualified host name
	// no-op if the hostname is already in expected format
	satlabIDToUse := specifiedSatlabID
	if satlabIDToUse == "" {
		satlabIDToUse = fetchedSatlabID
	}
	return MaybePrepend(prefix, satlabIDToUse, content)
}

// DefaultAuthScopes is the default scopes for shivas login
var DefaultAuthScopes = []string{auth.OAuthScopeEmail}

// DefaultAuthOptions is an auth.Options struct prefilled with chrome-infra
// defaults.
var DefaultAuthOptions = chromeinfra.SetDefaultAuthOptions(auth.Options{
	Scopes:     shivassite.GetAuthScopes(DefaultAuthScopes),
	SecretsDir: SecretsDir(),
})

// SecretsDir customizes the location for auth-related secrets.
func SecretsDir() string {
	configDir := os.Getenv("XDG_CACHE_HOME")
	if configDir == "" {
		configDir = filepath.Join(os.Getenv("HOME"), ".cache")
	}
	return filepath.Join(configDir, "satlab", "auth")
}

// // VersionNumber is the version number for the tool. It follows the Semantic
// // Versioning Specification (http://semver.org) and the format is:
// // "MAJOR.MINOR.0+BUILD_TIME".
// // We can ignore the PATCH part (i.e. it's always 0) to make the maintenance
// // work easier.
// // We can also print out the build time (e.g. 20060102150405) as the METADATA
// // when show version to users.
var VersionNumber = fmt.Sprintf("%d.%d.%d", Major, Minor, Patch)

// // Major is the Major version number
const Major = 0

// // Minor is the Minor version number
const Minor = 1

// // Patch is the PAtch version number
const Patch = 0

// DefaultPRPCOptions is used for PRPC clients.  If it is nil, the
// default value is used.  See prpc.Options for details.
//
// This is provided so it can be overridden for testing.
var DefaultPRPCOptions = prpcOptionWithUserAgent(fmt.Sprintf("satlab/%s", VersionNumber))

// CipdInstalledPath is the installed path for satlab package.
// This is the path to the directory containing main.go relative to the repo root.
var CipdInstalledPath = "infra/cros/satlab/satlab/"

// prpcOptionWithUserAgent create prpc option with custom UserAgent.
//
// DefaultOptions provides Retry ability in case we have issue with service.
func prpcOptionWithUserAgent(userAgent string) *prpc.Options {
	options := *prpc.DefaultOptions()
	options.UserAgent = userAgent
	return &options
}

func GetBotPrefix() string {
	return os.Getenv(BotPrefix)
}

// GetRPCServerLogFile determines the log file to
// use for logging
func GetRPCServerLogFile() string {
	logfilename := os.Getenv(RPCServerLogFileEnv)
	if logfilename == "" {
		return DefaultRPCServerLogFile
	}
	return logfilename
}

// GetAuthOption returns the correct auth option for CLI and RPC calls
//
// If the call is already authenticated with user scope, the login credential is reused.
// If no existing credential found the service account key is used as authentication method.
func GetAuthOption(ctx context.Context) auth.Options {
	a := auth.NewAuthenticator(ctx, auth.SilentLogin, DefaultAuthOptions)
	if err := a.CheckLoginRequired(); err != nil {
		return auth.Options{
			Method:                 auth.ServiceAccountMethod,
			ServiceAccountJSONPath: GetServiceAccountPath(),
		}
	}
	return DefaultAuthOptions
}

// GetBotoPath get the boto file path
func GetBotoPath() string {
	botoFilePath := os.Getenv(BotoPath)
	if botoFilePath == "" {
		return filepath.Join(KeyFolder, BotoConfigFilename)
	}
	return botoFilePath
}

// DefaultCTPTimeoutMins is the default build timeout.
const DefaultCTPTimeoutMins = 360

// MaxIshCTPTimeoutMins is the maximum timeout (minus one minute) that can
// be scheduled as a CTP execution.
const MaxIshCTPTimeoutMins = 4319

// GetTestRunnerSwarmingPool determines which Partner Swarming Pool for test runner,
// based on the environment.
func GetTestRunnerSwarmingPool() string {
	return os.Getenv(TestRunnerSwarmingPoolEnv)
}

// GetCTPSwarmingPool determines which Partner Swarming Pool for CTP,
// based on the environment.
func GetCTPSwarmingPool() string {
	return os.Getenv(CTPSwarmingPoolEnv)
}

func GetCPConAccountID() string {
	return os.Getenv(CPConAccountIDEnv)
}
