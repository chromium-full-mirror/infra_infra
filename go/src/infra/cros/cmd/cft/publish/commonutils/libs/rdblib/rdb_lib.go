// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package rdblib

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/protobuf/encoding/prototext"

	"go.chromium.org/chromiumos/config/go/test/artifact"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform"
	"go.chromium.org/luci/common/api/gitiles"
	gitiles_pb "go.chromium.org/luci/common/proto/gitiles"
	realms_config "go.chromium.org/luci/common/proto/realms"
	rdb_pb "go.chromium.org/luci/resultdb/proto/v1"

	"go.chromium.org/infra/cros/cmd/cft/publish/commonutils"
	"go.chromium.org/infra/cros/cmd/cft/publish/commonutils/clients/rdbclient"
)

var (
	supportedResultAdapterFormats = map[string]bool{
		"gtest":              true,
		"json":               true,
		"native":             true,
		"single":             true,
		"tast":               true,
		"skylab-test-runner": true,
		"cros-test-result":   true,
	}

	boardVariantAllowlist = map[string]bool{
		"64":                  true,
		"-arc-r":              true,
		"-arc-s":              true,
		"-arc-t":              true,
		"-arc-u":              true,
		"-arc-v":              true,
		"-borealis":           true,
		"-kernelnext":         true,
		"-manatee-kernelnext": true,
		"-userdebug":          true,
	}

	baseVMBoardAllowlist = map[string]bool{
		"amd64-generic": true,
		"betty":         true,
	}
)

const (
	// Max size allowed is 500. Keeping it 490 to be safer.
	RpcBatchSize = 490

	// Resultdb service name
	RdbServiceName = "luci.resultdb.v1.Recorder"

	// Artifacts method name
	ArtifactsMethodName = "BatchCreateArtifacts"

	// Test results method name
	TestResultsMethodName = "BatchCreateTestResults"

	// Test exoneration method name
	TestExonerationMethodName = "BatchCreateTestExonerations"

	// Missing test cases upload retries
	MissingTestCasesUploadRetries = 2

	// Rdb query result limit
	RdbQueryResultLimit = 1000

	// Chrome internal Gitiles host
	ChromeInternalGitilesHost = "chrome-internal.googlesource.com"

	// ChromeOS infra config project on Gitiles
	ChromeOSInfraConfigGitilesProject = "chromeos/infra/config"

	// Main branch on Gitiles
	GitilesMainBranch = "main"

	// Realm config file path in infra/config repo on Gitiles
	RealmConfigFilePath = "generated/realms.cfg"

	// ChromeOS partner VM realm which is visible to all partners
	ChromeOSPartnerVMRealm = "chromeos:partner-vmtest"
)

type RdbLib struct {
	CurrentInvocation string
	RdbClient         *rdbclient.RdbClient
}

// UploadTestResults uploads test results to rdb
func (rdblib *RdbLib) UploadTestResults(ctx context.Context, rdbStreamConfig *rdbclient.RdbStreamConfig, testResult *artifact.TestResult) error {
	if rdbStreamConfig.ResultFormat == "" {
		return fmt.Errorf("result_format can not be empty for rdb upload")
	}
	if _, ok := supportedResultAdapterFormats[rdbStreamConfig.ResultFormat]; !ok {
		return fmt.Errorf("result_format %q could not be found in supported adapter formats: %T", rdbStreamConfig.ResultFormat, supportedResultAdapterFormats)
	}
	if rdbStreamConfig.ResultFile == "" {
		return fmt.Errorf("result_file can not be empty for rdb upload")
	}
	resultAdapterArgs := []string{rdblib.RdbClient.ResultAdapterExecutablePath, rdbStreamConfig.ResultFormat, "-result-file", rdbStreamConfig.ResultFile}

	if rdbStreamConfig.ArtifactDir != "" {
		resultAdapterArgs = append(resultAdapterArgs, "-artifact-directory", rdbStreamConfig.ArtifactDir)
		resultAdapterArgs = append(resultAdapterArgs, "-enable-invocation-artifacts-upload", "true")
	}

	if rdbStreamConfig.TesthausBaseURL != "" {
		resultAdapterArgs = append(resultAdapterArgs, "-testhaus-base-url", rdbStreamConfig.TesthausBaseURL)
	}

	realm := testResultRealm(ctx, testResult)
	if realm != "" {
		rdbStreamConfig.Include = true
		rdbStreamConfig.Realm = realm
		log.Printf("Successfully set the board.model realm: %q", realm)
	}

	resultAdapterArgs = append(resultAdapterArgs, "--", "echo")

	rdbStreamConfig.Cmds = resultAdapterArgs

	cmd, err := rdblib.RdbClient.StreamCommand(ctx, rdbStreamConfig)
	if err != nil {
		return fmt.Errorf("error in rdb upload stream command creation: %s", err.Error())
	}

	cmdName := "rdb-upload"
	stdout, stderr, err := commonutils.RunCommand(ctx, cmd, cmdName, nil, true)
	commonutils.LogOutputs(cmdName, stdout, stderr)
	if err != nil {
		return fmt.Errorf("error in %q cmd: %s", cmdName, err.Error())
	}

	return nil
}

// testResultRealm gets the luci realm for test results. Returns an empty value
// if no valid realm is generated.
func testResultRealm(ctx context.Context, testResult *artifact.TestResult) string {
	if testResult == nil {
		return ""
	}

	hostname := testResult.GetTestInvocation().GetDutTopology().GetId().GetValue()
	if hostname == "vm" {
		return partnerVMRealm(testResult)
	}

	validRealms, err := validRealms(ctx)
	if err != nil {
		log.Printf("Failed to get the valid realm list due to the error: %s", err.Error())
		return ""
	}
	return boardModelRealm(testResult, validRealms)

}

// boardModelRealm gets the board-model realm for this test result.
// Returns an empty value if no appropriate board-model realm was found.
func boardModelRealm(testResult *artifact.TestResult, validRealms map[string]bool) string {
	if testResult == nil {
		return ""
	}

	// If the test is a multi-DUT test, don't use the board-model realm.
	// TODO(b/251688396): Handle multi-DUT tests.
	if len(testResult.TestInvocation.GetSecondaryExecutionsInfo()) > 0 {
		return ""
	}

	executionInfo := testResult.GetTestInvocation().GetPrimaryExecutionInfo()
	board := executionInfo.GetBuildInfo().GetBoard()
	model := executionInfo.GetDutInfo().GetDut().GetChromeos().GetDutModel().GetModelName()
	image := executionInfo.GetBuildInfo().GetName()

	// Exit early if we don't know the board, model, or image.
	if board == "" || model == "" || image == "" {
		return ""
	}

	// Don't use the board-model realm unless it is a base or allowlisted
	// variant build.
	// Note that variant builds also will be associated with board-model realm
	// instead of board-variant-model realm which isn't supported.
	if !isBuildPartnerVisible(board, image) {
		return ""
	}

	boardModelRealm := board + "-" + model
	if _, ok := validRealms[boardModelRealm]; !ok {
		log.Printf("The generated board.model realm: %q is invalid", boardModelRealm)
		return ""
	}

	return "chromeos:" + boardModelRealm
}

// partnerVMRealm gets the partner vm realm for this test result. Returns an
// empty value if the vm board is not allowed to share with partners.
func partnerVMRealm(testResult *artifact.TestResult) string {
	executionInfo := testResult.GetTestInvocation().GetPrimaryExecutionInfo()

	// Returns early if vm test results are for staging testing.
	image := executionInfo.GetBuildInfo().GetName()
	if strings.HasPrefix(image, "staging-") {
		return ""
	}

	// Checks base vm boards (e.g. "betty") or vm board variants
	// (e.g. "betty-arc-r").
	board := executionInfo.GetBuildInfo().GetBoard()
	parts := strings.Split(board, "-")
	if baseVMBoardAllowlist[board] || (len(parts) > 0) && baseVMBoardAllowlist[parts[0]] {
		return ChromeOSPartnerVMRealm
	}

	return ""
}

// validRealms gets the full list of valid realms from the ChromeOS infra config
// file on the gitiles.
func validRealms(ctx context.Context) (map[string]bool, error) {
	ts, err := createTokenSource(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create token from json: %w", err)
	}

	// Reads the realm config file from the infra/config repo on Gitiles.
	client := oauth2.NewClient(ctx, ts)
	gitilesClient, err := gitiles.NewRESTClient(client, ChromeInternalGitilesHost, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create gitiles client: %w", err)
	}

	req := &gitiles_pb.DownloadFileRequest{
		Project:    ChromeOSInfraConfigGitilesProject,
		Committish: GitilesMainBranch,
		Path:       RealmConfigFilePath,
	}
	resp, err := gitilesClient.DownloadFile(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get the realm config file due to the error: %w", err)
	}

	realmsConfig := &realms_config.RealmsCfg{}
	if err = prototext.Unmarshal([]byte(resp.GetContents()), realmsConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal the realm config file due to the error: %w", err)
	}

	validRealms := make(map[string]bool, len(realmsConfig.Realms))
	for _, realm := range realmsConfig.Realms {
		if realm.Name != "" {
			validRealms[realm.Name] = true
		}
	}
	return validRealms, nil

}

// createTokenSource creates an oauth2.TokenSource from service account key
// credentials.
func createTokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	creds, err := google.FindDefaultCredentials(ctx)
	if err != nil {
		log.Printf("Error loading credentials: %s", err)
		return nil, err
	}
	return creds.TokenSource, nil
}

// isBuildPartnerVisible returns True if a build is visible to partners or False
// otherwise. A build is partner visible if it's a base or allowlisted variant
// build.
func isBuildPartnerVisible(board string, image string) bool {
	// Matches the base board name and fetches the variant suffix if it exists.
	buildPattern := fmt.Sprintf("^%s(.*)-[a-z]+/R[0-9.\\-]+$", board)
	buildRegex := regexp.MustCompile(buildPattern)
	match := buildRegex.FindStringSubmatch(image)

	if match == nil {
		return false
	}

	// Checks if it's a base build or an allowlisted variant build.
	variant := match[1]
	return variant == "" || boardVariantAllowlist[variant]
}

// UploadInvocationArtifacts uploads invocation artifacts to rdb
func (rdblib *RdbLib) UploadInvocationArtifacts(ctx context.Context, artifact *rdb_pb.Artifact) error {
	req := rdb_pb.BatchCreateArtifactsRequest{Requests: []*rdb_pb.CreateArtifactRequest{{Parent: rdblib.CurrentInvocation, Artifact: artifact}}}

	rdbRpcConfig := &rdbclient.RdbRpcConfig{ServiceName: RdbServiceName, MethodName: ArtifactsMethodName, IncludeUpdateToken: true}
	cmd, err := rdblib.RdbClient.RpcCommand(ctx, rdbRpcConfig)
	if err != nil {
		return fmt.Errorf("error in rpc command creation: %s", err.Error())
	}

	_, _, err = commonutils.RunCommand(ctx, cmd, "rdb-upload-invocation-artifacts", &req, true)

	if err != nil {
		return fmt.Errorf("error in rdb-upload-invocation-artifacts: %s", err.Error())
	}

	return nil
}

// TODO (b/241154998): we may not need ReportMissingTestCases as we can just create test results for missing cases
// and upload them as part of regular test results. Evaluate while integrating with new adapter
// and remove if this is not required.

// ReportMissingTestCases uploads missing test cases info to rdb
func (rdblib *RdbLib) ReportMissingTestCases(ctx context.Context, testNames []string, baseVariant map[string]string, buildbucketId string) error {
	if len(testNames) == 0 {
		log.Println("no missing test(s) to upload")
		return nil
	}

	variant := rdb_pb.Variant{Def: baseVariant}
	var reqsList []*rdb_pb.CreateTestResultRequest
	for _, testName := range testNames {
		testResult := rdb_pb.TestResult{TestId: testName, ResultId: buildbucketId, Status: rdb_pb.TestStatus_SKIP, Expected: false, Variant: &variant}
		testResultReq := rdb_pb.CreateTestResultRequest{Invocation: rdblib.CurrentInvocation, TestResult: &testResult}
		reqsList = append(reqsList, &testResultReq)
	}

	var batchedReqs [][]*rdb_pb.CreateTestResultRequest

	for i := 0; i < len(reqsList); i += RpcBatchSize {
		end := i + RpcBatchSize

		if end > len(reqsList) {
			end = len(reqsList)
		}

		batchedReqs = append(batchedReqs, reqsList[i:end])
	}

	for batchNum, reqs := range batchedReqs {
		batchReq := rdb_pb.BatchCreateTestResultsRequest{Invocation: rdblib.CurrentInvocation, Requests: reqs}

		rdbRpcConfig := &rdbclient.RdbRpcConfig{ServiceName: RdbServiceName, MethodName: TestResultsMethodName, IncludeUpdateToken: true}

		cmd, err := rdblib.RdbClient.RpcCommand(ctx, rdbRpcConfig)
		if err != nil {
			return fmt.Errorf("error during getting rpc command: %s", err.Error())
		}

		errMsg := ""
		for range MissingTestCasesUploadRetries {
			errMsg = ""
			_, _, err := commonutils.RunCommand(ctx, cmd, "rdb-upload-missing-tests", &batchReq, true)
			if err != nil {
				errMsg = fmt.Sprintf("error in rdb-upload-missing-tests batch %d: %s", batchNum, err.Error())
				log.Println(errMsg)
				continue
			}
			break
		}
		if errMsg != "" {
			return fmt.Errorf(errMsg)
		}
	}
	return nil
}

// ApplyExonerations applies exoneration to test results
func (rdblib *RdbLib) ApplyExonerations(ctx context.Context, invocationIds []string, defaultBehavior test_platform.Request_Params_TestExecutionBehavior, behaviorOverrideMap map[string]test_platform.Request_Params_TestExecutionBehavior, variantFilter map[string]string) error {
	testExecBehaviorFunc := func(testName string) test_platform.Request_Params_TestExecutionBehavior {
		overrideBehavior, ok := behaviorOverrideMap[testName]
		if !ok {
			overrideBehavior = test_platform.Request_Params_BEHAVIOR_UNSPECIFIED
		}
		if overrideBehavior > defaultBehavior {
			return overrideBehavior
		} else {
			return defaultBehavior
		}
	}

	isNonCriticalFunc := func(testResult *rdb_pb.TestResult) bool {
		testExecBehavior := testExecBehaviorFunc(testResult.GetTestId())
		isNonCritical := testExecBehavior == test_platform.Request_Params_NON_CRITICAL

		//A test results's variant must contain all attributes of the variant filter.
		resultVariantValueMap := commonutils.GetValueBoolMap(testResult.GetVariant().GetDef())
		variantFilterValueMap := commonutils.GetValueBoolMap(variantFilter)
		containsVariantFilter := commonutils.IsSubsetOf(variantFilterValueMap, resultVariantValueMap)

		return isNonCritical && containsVariantFilter
	}

	parsedIds, err := ParseInvocationIds(invocationIds)
	if err != nil {
		return err
	}

	rdbQueryConfig := &rdbclient.RdbQueryConfig{InvocationIds: parsedIds, VariantsWithUnexpectedResults: true, TestResultFields: []string{"variant", "testId", "status", "expected"}, Merge: false, Limit: RdbQueryResultLimit}
	cmd, err := rdblib.RdbClient.QueryCommand(ctx, rdbQueryConfig)
	if err != nil {
		return fmt.Errorf("error during getting query command: %s", err.Error())
	}
	stdout, _, err := commonutils.RunCommand(ctx, cmd, "rdb-query", nil, true)
	if err != nil {
		return fmt.Errorf("error during executing query command: %s", err.Error())
	}

	invMap, err := Deserialize(stdout)
	if err != nil {
		return fmt.Errorf("error during deserializing query output: %s", err.Error())
	}

	var testExonerations []*rdb_pb.TestExoneration
	for invId, inv := range invMap {
		unexpectedResults := inv.testResults
		if len(unexpectedResults) == 0 {
			log.Printf("no test results found for invocation id %s", invId)
			continue
		}

		for _, result := range unexpectedResults {
			if !result.Expected && result.Status != rdb_pb.TestStatus_PASS && isNonCriticalFunc(result) {
				explanationHtml := "failed but is not critical"
				if result.Status == rdb_pb.TestStatus_SKIP {
					explanationHtml = "unexpectedly skipped but is not critical"
				}

				testExoneration := rdb_pb.TestExoneration{TestId: result.TestId, Variant: result.Variant, ExplanationHtml: explanationHtml, Reason: rdb_pb.ExonerationReason(3)}
				testExonerations = append(testExonerations, &testExoneration)
			}
		}
	}

	if len(testExonerations) > 0 {
		err := rdblib.Exonerate(ctx, testExonerations)
		if err != nil {
			return fmt.Errorf("error during exoneration upload: %s", err.Error())
		}
	} else {
		log.Printf("no qualified test exoneration found")
	}
	return nil
}

// Exonerate exonerates test results based on provided exoneration info
func (rdblib *RdbLib) Exonerate(ctx context.Context, testExonerations []*rdb_pb.TestExoneration) error {
	if len(testExonerations) == 0 {
		return fmt.Errorf("no test exoneration info provided for exonerate command")
	}

	rdbRpcConfig := &rdbclient.RdbRpcConfig{ServiceName: RdbServiceName, MethodName: TestExonerationMethodName, IncludeUpdateToken: true}

	batchNum := 0
	for i := 0; i < len(testExonerations); i += RpcBatchSize {
		batchNum++
		end := i + RpcBatchSize

		if end > len(testExonerations) {
			end = len(testExonerations)
		}

		var createTeRequests []*rdb_pb.CreateTestExonerationRequest
		for _, te := range testExonerations[i:end] {
			createTeRequests = append(createTeRequests, &rdb_pb.CreateTestExonerationRequest{TestExoneration: te})
		}
		batchReq := rdb_pb.BatchCreateTestExonerationsRequest{Invocation: rdblib.CurrentInvocation, RequestId: uuid.New().String(), Requests: createTeRequests}

		// Create a new RPC command to batch upload each individual chunk of
		// exonerations.
		cmd, err := rdblib.RdbClient.RpcCommand(ctx, rdbRpcConfig)
		if err != nil {
			return fmt.Errorf("error during getting rpc command: %s", err.Error())
		}

		_, _, err = commonutils.RunCommand(ctx, cmd, "rdb-rpc-batch-exoneration", &batchReq, true)
		if err != nil {
			errMsg := fmt.Sprintf("error in rdb-rpc-batch-exoneration batch %d: %s", batchNum, err.Error())
			log.Println(errMsg)
			return fmt.Errorf(errMsg)
		}
	}
	return nil
}
