// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package driver implements drivers to execute tests.
package driver

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"google.golang.org/protobuf/encoding/protojson"
	"gopkg.in/yaml.v2"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/test/execution/errors"

	"infra/cros/cmd/cft/execution/cros-test/internal/common"
	"infra/cros/cmd/cft/execution/cros-test/internal/device"
	"infra/cros/cmd/cft/execution/cros-test/internal/tastrpc"
)

// TastDriver runs tast and report its results.
type TastDriver struct {
	// logger provides logging service.
	logger *log.Logger
}

// NewTastDriver creates a new driver to run tast tests.
func NewTastDriver(logger *log.Logger) *TastDriver {
	return &TastDriver{
		logger: logger,
	}
}

// Name returns the name of the driver.
func (td *TastDriver) Name() string {
	return "tast"
}

// RunTests drives a test framework to execute tests.
func (td *TastDriver) RunTests(ctx context.Context, resultsDir string, req *api.CrosTestRequest, tlwAddr string, tests []*api.TestCaseMetadata) (*api.CrosTestResponse, error) {
	testNamesToIds := getTestNamesToIds(tests)
	testNamesToMetadata := getTestNamesToMetadata(tests)
	testNames := getTestNames(tests)

	reportServer, err := tastrpc.NewReportsServer(0, testNames, testNamesToIds, testNamesToMetadata, resultsDir)
	if err != nil {
		return nil, errors.NewStatusError(errors.ServerStartingError,
			fmt.Errorf("failed to create tast report server: %w", err))
	}
	defer reportServer.Stop()

	primary, err := device.FillDUTInfo(req.Primary, "")
	if err != nil {
		return nil, errors.NewStatusError(errors.InvalidArgument,
			fmt.Errorf("cannot get address from primary device: %w", err))
	}

	companions, androidCompanions, err := common.Companions(req.Companions)
	if err != nil {
		return nil, errors.NewStatusError(errors.InvalidArgument,
			fmt.Errorf("cannot get companion devices information: %w", err))
	}

	yamlPath, err := genHostInfoYAML(primary)
	if err != nil {
		return nil, fmt.Errorf("failed to generate info yaml: %w", err)
	}

	labConfigPath, err := genLabConfigJsonpb(resultsDir, primary, companions, androidCompanions)
	if err != nil {
		return nil, fmt.Errorf("failed to generate DUT lab config file: %w", err)
	}

	// Get tast execution args.
	customTastArgs, _, err := common.UnpackMetadata(req)
	if err != nil {
		return nil, err
	}

	userTastArgs, runTimeVars, err := splitExtraArg(common.ExtraArgs(req))
	if err != nil {
		return nil, fmt.Errorf("failed to split extra arguments: %w", err)
	}

	userTastArgs, err = mergeTastArgs(userTastArgs, customTastArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to merge Tast flags: %w", err)
	}

	// Be nice and clean up.
	defer os.Remove(yamlPath)
	args := newTastArgs(primary, companions, androidCompanions, testNames,
		resultsDir, reportServer.Address(), yamlPath, userTastArgs, runTimeVars, labConfigPath)

	err = common.WriteHostInfoToFile(resultsDir, primary.Addr, primary, td.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to generate hostinfo: %w", err)

	}

	// Run tast.
	cmd := exec.Command("/usr/bin/tast", genArgList(args)...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to capture tast stderr: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		td.logger.Println("Failed to capture tast stdout: ", err)
		return nil, errors.NewStatusError(errors.IOCaptureError,
			fmt.Errorf("failed to capture tast stdout: %w", err))
	}
	td.logger.Println("Running Tast ", cmd.String())
	if err := cmd.Start(); err != nil {
		td.logger.Println("Failed to run tast: ", err)
		return nil, errors.NewStatusError(errors.CommandStartingError,
			fmt.Errorf("failed to run tast: %w", err))
	}
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		common.TestScanner(stderr, td.logger, "tast")
	}()

	go func() {
		defer wg.Done()
		common.TestScanner(stdout, td.logger, "tast")
	}()

	wg.Wait()

	MissingTestErrMsg := ""
	err = cmd.Wait()
	if err != nil {
		td.logger.Println("Failed to run tast: ", err)
		MissingTestErrMsg = fmt.Sprintf("Test did not run due to %s", err)
	}

	testResults := reportServer.TestsReports()
	missingResults := reportServer.MissingTestsReports(MissingTestErrMsg)
	results := append(testResults, missingResults...)
	reportWarnings := reportServer.Warnings()
	if len(reportWarnings) > 0 {
		for _, w := range reportWarnings {
			td.logger.Printf("%s\n", w)
		}
	}
	reportErrors := reportServer.Errors()
	if len(reportErrors) > 0 {
		for _, e := range reportErrors {
			td.logger.Printf("%v\n", e)
		}
		return &api.CrosTestResponse{TestCaseResults: results}, reportErrors[len(reportErrors)-1]
	}

	_ = common.PublishTkoStatusFile(resultsDir, results)

	return &api.CrosTestResponse{TestCaseResults: results}, nil
}

// Command name and flag names.
const (
	httpPrefix                = "http://"
	runSubcommand             = "run"
	verboseFlag               = "-verbose"
	logTimeFlag               = "-logtime"
	defaultSysServicesTimeout = "600"

	// Tast flags
	attachdebuggerFlag         = "-attachdebugger"
	buildFlag                  = "-build"
	buildArtifactsURLFlag      = "-buildartifactsurl"
	buildBundleFlag            = "-buildbundle"
	buildOutDirFlag            = "-buildoutdir"
	buildWorkspaceFlag         = "-buildworkspace"
	checkBuildDepsFlag         = "-checkbuilddeps"
	checkTestDepsFlag          = "-checktestdeps"
	companionDUTFlag           = "-companiondut"
	connectionTimeoutFlag      = "-connectiontimeout"
	continueAfterFailureFlag   = "-continueafterfailure"
	debuggerPortForwardingFlag = "-debuggerportforwarding"
	defaultVarsDirFlag         = "-defaultvarsdir"
	devServerFlag              = "-devservers"
	downloadDataFlag           = "-downloaddata"
	downloadPrivateBundlesFlag = "-downloadprivatebundles"
	dutLabConfigFlag           = "-dutlabconfig"
	ephemeralDevserverFlag     = "-ephemeraldevserver"
	excludeSkippedFlag         = "-excludeskipped"
	extraAllowedBucketsFlag    = "-extraallowedbuckets"
	extraUseFlagsFlag          = "-extrauseflags"
	failForTestsFlag           = "-failfortests"
	installBuildDepsFlags      = "-installbuilddeps"
	keyDirFlag                 = "-keydir"
	keyFileFlag                = "-keyfile"
	localBundleDirFlag         = "-localbundledir"
	localdatadirFlag           = "-localdatadir"
	localOutDirFlag            = "-localoutdir"
	localRunnerFlag            = "-localrunner"
	localTempDirFlag           = "-localtempdir"
	maxSysMsgLogSizeFlag       = "-maxsysmsglogsize"
	maxTestFailuresFlag        = "-maxtestfailures"
	maybeMissingVarsFlag       = "-maybemissingvars"
	proxyFlag                  = "-proxy"
	proxyCommandFlag           = "-proxycommand"
	remoteBundleDirFlag        = "-remotebundledir"
	remoteDataDirFlag          = "-remotedatadir"
	remoteRunnerFlag           = "-remoterunner"
	remoteTempDirFlag          = "-remotetempdir"
	reportsServerFlag          = "-reports_server"
	resultsDirFlag             = "-resultsdir"
	testRetriesFlag            = "-retries"
	testRepeatsFlag            = "-repeats"
	shardIndexFlag             = "-shardindex"
	shardMethodFlag            = "-shardmethod"
	sshRetriesFlag             = "-sshretries"
	sysInfoFlag                = "-sysinfo"
	systemServicesTimeoutFlag  = "-systemservicestimeout"
	testFilterFileFlag         = "-testfilterfile"
	timeOutFlag                = "-timeout"
	tlwServerFlag              = "-tlwserver"
	varFlag                    = "-var"
	varsFileFlag               = "-varsfile"
	waitUntilReadyFlag         = "-waituntilready"
	waitUntilTimeoutFlag       = "-waituntilreadytimeout"
)

// tastFlags include all Tast Flags.
var tastFlags = map[string]struct{}{
	attachdebuggerFlag:         {},
	buildFlag:                  {},
	buildArtifactsURLFlag:      {},
	buildBundleFlag:            {},
	buildOutDirFlag:            {},
	buildWorkspaceFlag:         {},
	checkBuildDepsFlag:         {},
	checkTestDepsFlag:          {},
	companionDUTFlag:           {},
	connectionTimeoutFlag:      {},
	continueAfterFailureFlag:   {},
	debuggerPortForwardingFlag: {},
	defaultVarsDirFlag:         {},
	devServerFlag:              {},
	downloadDataFlag:           {},
	downloadPrivateBundlesFlag: {},
	dutLabConfigFlag:           {},
	ephemeralDevserverFlag:     {},
	excludeSkippedFlag:         {},
	extraAllowedBucketsFlag:    {},
	extraUseFlagsFlag:          {},
	failForTestsFlag:           {},
	installBuildDepsFlags:      {},
	keyDirFlag:                 {},
	keyFileFlag:                {},
	localBundleDirFlag:         {},
	localdatadirFlag:           {},
	localOutDirFlag:            {},
	localRunnerFlag:            {},
	localTempDirFlag:           {},
	maxSysMsgLogSizeFlag:       {},
	maxTestFailuresFlag:        {},
	maybeMissingVarsFlag:       {},
	proxyFlag:                  {},
	proxyCommandFlag:           {},
	remoteBundleDirFlag:        {},
	remoteDataDirFlag:          {},
	remoteRunnerFlag:           {},
	remoteTempDirFlag:          {},
	reportsServerFlag:          {},
	resultsDirFlag:             {},
	testRetriesFlag:            {},
	testRepeatsFlag:            {},
	shardIndexFlag:             {},
	shardMethodFlag:            {},
	sshRetriesFlag:             {},
	sysInfoFlag:                {},
	systemServicesTimeoutFlag:  {},
	testFilterFileFlag:         {},
	timeOutFlag:                {},
	tlwServerFlag:              {},
	varFlag:                    {},
	varsFileFlag:               {},
	waitUntilReadyFlag:         {},
	waitUntilTimeoutFlag:       {},
}

// allowedTastFlag includes all Tast Flags that are allowed to be overridden.
var allowedTastFlag = map[string]struct{}{
	buildArtifactsURLFlag:    {},
	connectionTimeoutFlag:    {},
	continueAfterFailureFlag: {},
	downloadDataFlag:         {},
	excludeSkippedFlag:       {},
	extraAllowedBucketsFlag:  {},
	testRepeatsFlag:          {},
	extraUseFlagsFlag:        {},
	failForTestsFlag:         {},
	maxSysMsgLogSizeFlag:     {},
	maybeMissingVarsFlag:     {},
}

// splitExtraArg split args into two sets. Those with "-" prefix are considered as
// Tast flags. Those without are considered as runtime variables. It also checks
// if the specified Tast flags exist or are allowed to be overridden.
func splitExtraArg(args []*api.Arg) (tastArgs, runtimeVars []*api.Arg, err error) {
	for _, a := range args {
		// If a flag does not start with "-", consider this as a runtime variable.
		if !strings.HasPrefix(a.GetFlag(), "-") {
			runtimeVars = append(runtimeVars, a)
			continue
		}
		if _, ok := tastFlags[a.GetFlag()]; !ok {
			return nil, nil, fmt.Errorf("invalid Tast flag: %s", a.GetFlag())
		}
		if _, ok := allowedTastFlag[a.GetFlag()]; !ok {
			// nolint:stylecheck // ST1005 "Tast" is a proper noun and can be capitalized
			return nil, nil, fmt.Errorf("Tast flag %s is not allowed to be overridden", a.GetFlag())
		}
		tastArgs = append(tastArgs, a)
	}
	return tastArgs, runtimeVars, nil
}

// mergeTastArgs merges arguments from user args and custom Args.
func mergeTastArgs(userArgs, customArgs []*api.Arg) (args []*api.Arg, err error) {
	// Put all customArgs in a map.
	lookup := make(map[string]int)
	for i, a := range customArgs {
		flag := a.GetFlag()
		if !strings.HasPrefix(flag, "-") {
			flag = "-" + flag
		}
		if _, ok := allowedTastFlag[flag]; !ok {
			// nolint:stylecheck // ST1005 "Tast" is a proper noun and can be capitalized
			return nil, fmt.Errorf("Tast flag %s is not allowed to be overridden", flag)
		}
		lookup[flag] = i
		args = append(args, &api.Arg{Flag: flag, Value: a.GetValue()})
	}
	for _, a := range userArgs {
		if i, ok := lookup[a.GetFlag()]; !ok {
			args = append(args, a)
		} else {
			// the user specified flag overrides the custom flag.
			args[i] = a
		}
	}
	return args, err
}

type flagValue struct {
	flag  string
	value string
}

// runArgs stores arguments to invoke Tast
type runArgs struct {
	primary    *device.DutInfo       // The information of the primary machine.
	patterns   []string              // The names of test to be run.
	tastFlags  map[string]string     // The flags for tast.
	runFlags   []flagValue           // The flags for tast run command.
	companions []*device.DutInfo     // The information of the companion DUTs to be used for testing.
	androids   []*device.AndroidInfo // The information of the android companions.

}

// newTastArgs created an argument structure for invoking tast
func newTastArgs(primary *device.DutInfo,
	companionDuts []*device.DutInfo, androids []*device.AndroidInfo,
	tests []string, resultsDir, rsAddress string, varsFilePath string,
	customTastArgs, runtimeVars []*api.Arg, labConfigFile string) *runArgs {
	runFlags := []flagValue{
		{sshRetriesFlag, "2"},
		{buildFlag, "false"},
		{downloadPrivateBundlesFlag, "true"},
		{resultsDirFlag, resultsDir},
		{reportsServerFlag, rsAddress},
		{varsFileFlag, varsFilePath},
		{systemServicesTimeoutFlag, defaultSysServicesTimeout}, // Longer timeout for VMs.
	}

	if labConfigFile != "" {
		runFlags = append(runFlags, flagValue{dutLabConfigFlag, labConfigFile})
	}

	setDefaultRetries := true
	for _, a := range customTastArgs {
		if a.GetFlag() == "" {
			continue
		}
		if a.GetFlag() == testRepeatsFlag || a.GetFlag() == testRetriesFlag {
			// Do not set default retries flag if overridden or repeat flag is set.
			setDefaultRetries = false
		}
		runFlags = append(runFlags, flagValue{a.GetFlag(), a.GetValue()})
	}

	for _, a := range runtimeVars {
		if a.GetFlag() == "" {
			continue
		}
		runFlags = append(runFlags, flagValue{varFlag, fmt.Sprintf("%s=%s", a.GetFlag(), a.GetValue())})
	}

	if setDefaultRetries {
		// TODO b/270193958 remove this hardcode for a var.
		runFlags = append(runFlags, flagValue{testRetriesFlag, "1"})
	}

	return &runArgs{
		primary: primary,
		tastFlags: map[string]string{
			verboseFlag: "true",
			logTimeFlag: "false",
		},
		runFlags:   runFlags,
		patterns:   tests, // TO-DO Support Tags
		companions: companionDuts,
		androids:   androids,
	}
}

// genArgList generates argument list for invoking tast
func genArgList(args *runArgs) (argList []string) {
	for flag, value := range args.tastFlags {
		argList = append(argList, fmt.Sprintf("%v=%v", flag, value))
	}
	argList = append(argList, runSubcommand)
	for _, fv := range args.runFlags {
		argList = append(argList, fmt.Sprintf("%v=%v", fv.flag, fv.value))
	}
	for _, c := range args.companions {
		if c.DevboardServer != "" {
			// Skipped if the DUT is a devboard.
			continue
		}
		// example: -companiondut=cd1:127.0.0.1:2222
		argList = append(argList, fmt.Sprintf("%v=%s:%s", companionDUTFlag, c.Role, c.Addr))
	}

	if len(args.androids) > 0 {
		// example: -var=android.companions=host1:1C291FDEE00923:pixel5a
		var androidArgs []string
		for _, a := range args.androids {
			androidArgs = append(androidArgs,
				fmt.Sprintf("%s:%s:%s", a.AssoicateAddr, a.Serial, a.ModelName))
		}
		argList = append(argList,
			fmt.Sprintf("%v=android.companions=%s", varFlag, strings.Join(androidArgs, ",")))
	}

	// Fill in the servo var flags.
	servoStrs := ""
	if args.primary.Servo != "" {
		// Fill in the old servo var flag for backward compatibility.
		// example -var=servo=labstation:9996/
		argList = append(argList, fmt.Sprintf("%v=servo=%s", varFlag, args.primary.Servo))
		// Fill in the servo var flag
		servoStrs = fmt.Sprintf(":%s", args.primary.Servo)
	}
	for _, c := range args.companions {
		if c.Servo != "" {
			servoStrs = fmt.Sprintf("%s,%s:%s", servoStrs, c.Role, c.Servo)
		}
	}
	if servoStrs != "" {
		// example: -var=servers.servo=:labstation:9995,cd1:labstation:9998
		argList = append(argList, fmt.Sprintf("%v=servers.servo=%s", varFlag, servoStrs))
	}

	if args.primary.CacheServer != "" {
		// example: var=servers.dut=:d1:22,cd1:d2:22,cd3:d3:22
		argList = append(argList, fmt.Sprintf("%v=%s", devServerFlag, httpPrefix+args.primary.CacheServer))
	} else {
		// Fill in DUT server var flags.
		dutServerStrs := ""
		if args.primary.DutServer != "" {
			// Fill in the servo var flag
			dutServerStrs = fmt.Sprintf(":%s", args.primary.DutServer)
		}
		for _, c := range args.companions {
			if c.DutServer != "" {
				dutServerStrs = fmt.Sprintf("%s,%s:%s", dutServerStrs, c.Role, c.DutServer)
			}
		}
		if dutServerStrs != "" {
			// example: var=servers.dut=:d1:22,cd1:d2:22,cd3:d3:22
			argList = append(argList, fmt.Sprintf("%v=servers.dut=%s", varFlag, dutServerStrs))
		}
	}

	// Fill in libs server var flag.
	libsServerStr := ""
	if args.primary.LibsServer != "" {
		libsServerStr = fmt.Sprintf(":%s", args.primary.LibsServer)
	}
	if libsServerStr != "" {
		// example: var=servers.libs=:d1:22
		argList = append(argList, fmt.Sprintf("%v=servers.libs=%s", varFlag, libsServerStr))
	}

	// Fill in RPM var flags.
	// Fill in the frontendAddress var flags.
	if args.primary.FrontendAddress != "" {
		argList = append(argList, fmt.Sprintf("%v=frontendAddress=%s", varFlag, args.primary.FrontendAddress))
	}
	// Fill in the powerunitHostname var flags.
	if args.primary.PowerUnitHostName != "" {
		argList = append(argList, fmt.Sprintf("%v=powerunitHostname=%s", varFlag, args.primary.PowerUnitHostName))
	}
	// Fill in the powerunitOutlet var flags.
	if args.primary.PowerUnitOutlet != "" {
		argList = append(argList, fmt.Sprintf("%v=powerunitOutlet=%s", varFlag, args.primary.PowerUnitOutlet))
	}
	// Fill in the hydraHostname var flags.
	if args.primary.HydraHostName != "" {
		argList = append(argList, fmt.Sprintf("%v=hydraHostname=%s", varFlag, args.primary.HydraHostName))
	}

	// Fill in Provision server var flags.
	provisionServerStrs := ""
	if args.primary.ProvisionServer != "" {
		// Fill in the servo var flag
		provisionServerStrs = fmt.Sprintf(":%s", args.primary.ProvisionServer)
	}
	for _, c := range args.companions {
		if c.ProvisionServer != "" {
			provisionServerStrs = fmt.Sprintf("%s,%s:%s", provisionServerStrs, c.Role, c.ProvisionServer)
		}
	}
	if provisionServerStrs != "" {
		// example: -var=servers.provision=primary:p1:22,cd1:p2:22,cd2:p2:22
		argList = append(argList, fmt.Sprintf("%v=servers.provision=%s", varFlag, provisionServerStrs))
	}

	// Fill in devboard server var flags.
	devboardServerStrs := ""
	if args.primary.DevboardServer != "" {
		// Fill in the servo var flag
		devboardServerStrs = fmt.Sprintf(":%s", args.primary.DevboardServer)
	}
	for _, c := range args.companions {
		if c.DevboardServer != "" {
			devboardServerStrs = fmt.Sprintf("%s,%s:%s", devboardServerStrs, c.Role, c.DevboardServer)
		}
	}
	if devboardServerStrs != "" {
		// example: -var=servers.devboard=primary:p1:22,cd1:p2:22,cd2:p2:22
		argList = append(argList, fmt.Sprintf("%v=servers.devboard=%s", varFlag, devboardServerStrs))
	}

	if args.primary.DevboardServer != "" {
		// If the DUT is devboard, use "-" as the DUT address.
		argList = append(argList, "-")
	} else {
		argList = append(argList, args.primary.Addr)
	}
	argList = append(argList, args.patterns...)
	return argList
}

// Labels contains AutotestHostInfoLabels
// Note, the name is intentionally `AutotestHostInfoLabels` as that is a key string for parsing.
type Labels struct {
	AutotestHostInfoLabels string
}

func getLabelsString(dut *device.DutInfo) (string, error) {
	_, labels, err := device.AppendChromeOsLabels(dut)
	if err != nil {
		return "", fmt.Errorf("topology failed: %w", err)
	}
	var attrStr string
	var attrList []string
	for _, label := range labels {
		attrList = append(attrList, fmt.Sprintf("\"%v\"", label))

	}

	joind := strings.Join(attrList[:], ", ")
	attrStr = fmt.Sprintf("[%v]", joind)

	return attrStr, nil
}

func genHostInfoYAML(dut *device.DutInfo) (string, error) {
	generateLabels, err := getLabelsString(dut)
	if err != nil {
		return "", err
	}
	labels := Labels{
		AutotestHostInfoLabels: generateLabels,
	}

	yamlData, err := yaml.Marshal(&labels)
	if err != nil {
		return "", err
	}

	file, err := os.CreateTemp("/tmp", "hostinfoyaml")
	if err != nil {
		return "", err
	}

	err = os.WriteFile(file.Name(), yamlData, 0644)
	if err != nil {
		return "", err
	}
	return file.Name(), nil
}

func genLabConfigJsonpb(dir string, primary *device.DutInfo, companions []*device.DutInfo, androidCompanion []*device.AndroidInfo) (string, error) {
	dutLabInfo, err := device.GenLabConfig(primary, companions, androidCompanion)
	if err != nil {
		return "", fmt.Errorf("failed generate lab information for DUTs: %w", err)
	}
	encoded, err := protojson.Marshal(dutLabInfo)
	if err != nil {
		return "", fmt.Errorf("failed marshal lab information for DUTs: %w", err)
	}
	path := filepath.Join(dir, "dutlabconfig.jsonpb")
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return "", fmt.Errorf("failed write lab information for DUTs: %w", err)
	}
	return path, nil
}
