// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/api/metadata"
	"go.chromium.org/chromiumos/config/go/test/artifact"
	"go.chromium.org/luci/common/errors"
	rdb_pb "go.chromium.org/luci/resultdb/proto/v1"

	"go.chromium.org/infra/cros/cmd/cft/common/files"
	"go.chromium.org/infra/cros/cmd/cft/publish/commonutils"
	"go.chromium.org/infra/cros/cmd/cft/publish/commonutils/clients/rdbclient"
	"go.chromium.org/infra/cros/cmd/cft/publish/commonutils/libs/rdblib"
	"go.chromium.org/infra/cros/cmd/cft/publish/commonutils/storage"
)

const (
	// Path to rdb executable
	RdbExecutablePath = "/usr/bin/rdb"

	// Path to result_adapter executable
	ResultAdapterExecutablePath = "/usr/bin/result_adapter"

	// Path to temp directory for rdb-publish
	RdbTempDirName = "rdb-publish-temp"

	// File name for test result json
	TestResultJsonFileName = "testResult.json"

	// File name for invocation-level properties json
	InvPropertiesFile = "invProperties.json"

	// File name for code sources under test
	CodeSourcesJsonFileName = "sources.jsonpb"

	// File path for the chromium test results in gtest format
	ChromiumResultJSONFilePathForGtest = "chromium/results/output.json"

	// File path for the chromium test results in native format
	ChromiumResultJSONFilePathForNative = "results/native_results.jsonl"

	// Result format for rdb
	ResultAdapterResultFormat = "cros-test-result"

	// MaxSizeInvocationProperties is the maximum size of the invocation level
	// properties stored in ResultDB.
	MaxSizeInvocationProperties = 16 * 1024 // 16 KB

	// Path to all results artifact directory
	ResultsArtifactDir = "/tmp/test/results"
)

type RdbPublishService struct {
	RetryCount                int
	CurrentInvocationId       string
	TestResultProto           *artifact.TestResult
	TesthausURL               string
	TempDirPath               string
	Sources                   *metadata.PublishRdbMetadata_Sources
	BaseVariant               map[string]string
	PostProcessResponses      *api.RunActivitiesResponse
	FirmwareProvisionResponse *api.FirmwareProvisionResponse
	Is3DRun                   bool
}

// NewRdbPublishService creates a new ResultDB publish service to interact with
// ResultDB.
func NewRdbPublishService(req *api.PublishRequest) (*RdbPublishService, error) {
	m, err := UnpackMetadata(req)
	if err != nil {
		return nil, err
	}

	if err = commonutils.ValidateRDBPublishRequest(req, m); err != nil {
		return nil, err
	}

	retryCount := 0
	if req.GetRetryCount() > 0 {
		retryCount = int(req.GetRetryCount())
	}

	// Populate additional info in test results.
	testResult := m.GetTestResult()
	postProcessResponses := m.GetPostProcessResponses()
	fwProvisionResponse := m.GetFirmwareProvisionResponse()
	if testResult != nil {
		ingestEqCInfo(testResult, req)

		// Fetches the existing build metadata if available in the test result.
		buildMetadata := testResult.GetTestInvocation().GetPrimaryExecutionInfo().GetBuildInfo().GetBuildMetadata()
		if buildMetadata == nil {
			buildMetadata = &artifact.BuildMetadata{}
			testResult.TestInvocation.PrimaryExecutionInfo.BuildInfo.BuildMetadata = buildMetadata
		}

		ingestPostProcessResponses(testResult, postProcessResponses)
		ingestFirmwareProvisionResponse(buildMetadata, fwProvisionResponse)
	}

	return &RdbPublishService{
		RetryCount:                retryCount,
		CurrentInvocationId:       m.GetCurrentInvocationId(),
		TestResultProto:           testResult,
		TesthausURL:               m.GetTesthausUrl(),
		Sources:                   m.GetSources(),
		BaseVariant:               m.GetBaseVariant(),
		PostProcessResponses:      postProcessResponses,
		FirmwareProvisionResponse: fwProvisionResponse,
		Is3DRun:                   req.GetIs_3DRun(),
	}, nil
}

// UploadToRdb uploads test results to ResultDB.
func (rps *RdbPublishService) UploadToRdb(ctx context.Context) error {
	rdbClient := rdbclient.RdbClient{RdbExecutablePath: RdbExecutablePath, ResultAdapterExecutablePath: ResultAdapterExecutablePath}
	rdbLib := rdblib.RdbLib{CurrentInvocation: rps.CurrentInvocationId, RdbClient: &rdbClient}

	// Create temp dir if required
	if rps.TempDirPath == "" {
		dirPath, err := commonutils.MakeTempDir(ctx, "", RdbTempDirName)
		if err != nil {
			return fmt.Errorf("error during creating temp dir for rdb publish: %w", err)
		}
		rps.TempDirPath = dirPath
	}

	// Write test result to file
	testResult := rps.TestResultProto
	testResultFilePath := path.Join(rps.TempDirPath, TestResultJsonFileName)
	err := commonutils.WriteProtoJsonFile(ctx, testResultFilePath, testResult)
	if err != nil {
		return fmt.Errorf("error during writing test result proto to file: %w", err)
	}
	log.Printf("test result file created in path: %q for the test result: %s before uploading to rdb", testResultFilePath, testResult)

	// Write test invocation proto to invocation-level properties file
	testInv := testResult.GetTestInvocation()
	invPropertiesFilePath := path.Join(rps.TempDirPath, InvPropertiesFile)
	err = commonutils.WriteProtoJsonFile(ctx, invPropertiesFilePath, testInv)
	if err != nil {
		return fmt.Errorf("error during writing test invocation proto to file: %w", err)
	}
	log.Printf("invocation properties file created in path: %q", invPropertiesFilePath)

	sourcesFilePath := path.Join(rps.TempDirPath, CodeSourcesJsonFileName)
	hasSources, err := downloadSources(ctx, sourcesFilePath, rps.Sources)
	if err != nil {
		return fmt.Errorf("downloading sources: %w", err)
	}
	if hasSources {
		log.Printf("code sources under test stored at path: %q", sourcesFilePath)
	}

	// Upload invocation artifacts
	if rps.TesthausURL != "" {
		artifact := rdb_pb.Artifact{ArtifactId: "testhaus_logs", ContentType: "text/x-uri", Contents: []byte(rps.TesthausURL)}
		err := rdbLib.UploadInvocationArtifacts(ctx, &artifact)
		if err != nil {
			return fmt.Errorf("error during rdb invocation artifact upload: %w", err)
		}
	}

	// Build the resultdb upload config for test results.
	config := &rdbclient.RdbStreamConfig{}
	baseTags := map[string]string{}
	baseVariant := rps.BaseVariant
	testhausBaseURL := rps.TesthausURL

	if isChromiumTest(testResult) {
		// For Chromium test results
		config, err = chromiumTestRDBConfig(testResult, baseTags, baseVariant, testhausBaseURL, testResultFilePath)
		if err != nil {
			return fmt.Errorf("error during creating the resultdb config for chromium tests: %w", err)
		}
	} else {
		// For ChromeOS test results
		config = &rdbclient.RdbStreamConfig{
			BaseTags:        baseTags,
			BaseVariant:     baseVariant,
			ResultFile:      testResultFilePath,
			ResultFormat:    ResultAdapterResultFormat,
			TesthausBaseURL: testhausBaseURL,
		}
	}

	if hasSources {
		config.SourcesFile = sourcesFilePath
	}

	if validateInvProperties(testInv) {
		config.InvPropertiesFile = invPropertiesFilePath
	} else {
		log.Printf("invocation properties file is skipped from the upload because it exceeds the maximum size of: %d bytes", MaxSizeInvocationProperties)
	}

	_, err = os.Stat(ResultsArtifactDir)
	if err != nil {
		log.Printf("Warning: test result artifact directory: %q does not exist: %v ", ResultsArtifactDir, err)
	} else {
		config.ArtifactDir = ResultsArtifactDir
	}

	// Upload test results.
	err = rdbLib.UploadTestResults(ctx, config, rps.TestResultProto)
	if err != nil {
		return fmt.Errorf("error during rdb test results upload: %w", err)
	}

	return nil
}

// UnpackMetadata unpacks the Any metadata field into PublishGcsMetadata
func UnpackMetadata(req *api.PublishRequest) (*metadata.PublishRdbMetadata, error) {
	if req == nil {
		return nil, fmt.Errorf("input proto is nil")
	}

	var m metadata.PublishRdbMetadata
	if err := req.Metadata.UnmarshalTo(&m); err != nil {
		return &m, fmt.Errorf("improperly formatted input proto metadata, %s", err)
	}
	return &m, nil
}

// downloadSources prepares the sources.jsonpb file required by the `rdb`
// command at the given destPath, using information in the given sources proto.
func downloadSources(ctx context.Context, destPath string, sources *metadata.PublishRdbMetadata_Sources) (ok bool, err error) {
	if sources == nil {
		return false, nil
	}

	// Create Google Storage client with default credentials file.
	gsClient, err := storage.NewGSClient(ctx, "")
	if err != nil {
		log.Printf("error while creating new gs client: %s", err)
		return false, fmt.Errorf("creating new gs client: %w", err)
	}

	err = gsClient.DownloadFile(ctx, sources.GsPath, destPath)
	if err != nil {
		if err == storage.ErrObjectNotExist {
			// Not all builds will have source metadata available.
			// This is normal.
			log.Printf("Google storage file %s not found, continuing without sources", sources.GsPath)
			return false, nil
		}
		return false, errors.Annotate(err, "downloading sources.jsonpb").Err()
	}

	sourcesProto := &rdb_pb.Sources{}
	err = files.ReadProtoJSONFile(ctx, destPath, sourcesProto)
	if err != nil {
		return false, errors.Annotate(err, "reading sources.jsonpb").Err()
	}

	// IsDirty indicates whether the source code tested is completely
	// described by the commit and changelists contained in the Sources
	// proto.
	//
	// If the build used other sources (e.g. uncommitted changes,
	// such as a new version of a dependency) or if the deployment was
	// dirty, this shall be set to true.
	if sources.IsDeploymentDirty {
		sourcesProto.IsDirty = true
	}

	err = commonutils.WriteProtoJsonFile(ctx, destPath, sourcesProto)
	if err != nil {
		return false, errors.Annotate(err, "writing sources.jsonpb").Err()
	}
	return true, nil
}

// validateInvProperties returns false if properties is invalid.
func validateInvProperties(properties *artifact.TestInvocation) bool {
	return proto.Size(properties) <= MaxSizeInvocationProperties
}

// isChromiumTest returns true if the current test run is for chromium test.
func isChromiumTest(testResult *artifact.TestResult) bool {
	if len(testResult.TestRuns) == 0 {
		return false
	}

	// Chromium tests in Skylab set the resultdb_settings flag in the test args.
	// This validation logic is aligned with the both non-CFT and CFT flows in
	// TRv1.
	testArgs := testResult.TestRuns[0].GetExecutionMetadata().GetTestArgs()
	_, ok := testArgs["resultdb_settings"]
	return ok
}

// extractBaseChromiumRDBConfig extracts base resultdb config from test_args for
// chromium test results.
func extractBaseChromiumRDBConfig(testArgs map[string]string) (map[string]interface{}, error) {
	rdbSettingsBytes, err := base64.StdEncoding.DecodeString(testArgs["resultdb_settings"])
	if err != nil {
		return nil, fmt.Errorf("error decoding the resultdb config base64 string for chromium results: %v", err)
	}

	// Unmarshal the JSON bytes into a map
	rdbSettings := map[string]interface{}{}
	err = json.Unmarshal(rdbSettingsBytes, &rdbSettings)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling JSON for chromium resultdb config: %v", err)
	}

	return rdbSettings, nil
}

// fetchChromeosTags fetches ChromeOS specific tags from the test results.
func fetchChromeosTags(testResult *artifact.TestResult) map[string]string {
	primaryExecutionInfo := testResult.GetTestInvocation().GetPrimaryExecutionInfo()
	if primaryExecutionInfo == nil {
		return map[string]string{}
	}

	tags := map[string]string{}
	buildInfo := primaryExecutionInfo.GetBuildInfo()
	if buildInfo != nil {
		buildName := buildInfo.Name
		tags["image"] = buildName
		tags["build"] = strings.Split(buildName, "/")[1]
	}

	envInfo := primaryExecutionInfo.GetEnvInfo()
	var buildbucketInfo *artifact.BuildbucketInfo
	switch envInfo.(type) {
	case *artifact.ExecutionInfo_SkylabInfo:
		skylabInfo := primaryExecutionInfo.GetSkylabInfo()
		if skylabInfo != nil {
			buildbucketInfo = skylabInfo.GetBuildbucketInfo()
		}
	case *artifact.ExecutionInfo_SatlabInfo:
		satlabInfo := primaryExecutionInfo.GetSatlabInfo()
		if satlabInfo != nil {
			buildbucketInfo = satlabInfo.GetBuildbucketInfo()
		}
	default:
		log.Printf("unsupported env info type: %v", envInfo)
	}
	if buildbucketInfo != nil {
		tags["ancestor_buildbucket_ids"] = strings.Trim(
			strings.Join(strings.Fields(
				fmt.Sprint(buildbucketInfo.GetAncestorIds())), ","),
			"[]")
	}

	return tags
}

// chromiumTestRDBConfig creates the resultdb config for chromium tests.
func chromiumTestRDBConfig(testResult *artifact.TestResult, baseTags map[string]string, baseVariant map[string]string, testhausBaseURL string, crosTestResultFile string) (*rdbclient.RdbStreamConfig, error) {
	if len(testResult.TestRuns) == 0 {
		return nil, fmt.Errorf("chromium test result is empty")
	}

	testArgs := testResult.TestRuns[0].GetExecutionMetadata().GetTestArgs()
	chromiumConfig, err := extractBaseChromiumRDBConfig(testArgs)
	if err != nil {
		return nil, err
	}

	for k, v := range chromiumConfig["base_variant"].(map[string]interface{}) {
		baseVariant[k] = v.(string)
	}

	resultFormat := chromiumConfig["result_format"].(string)
	resultFilePath := ""
	switch resultFormat {
	case "tast":
		// For tast, use the default ChromeOS result format and result file.
		resultFormat = ResultAdapterResultFormat
		resultFilePath = crosTestResultFile
	case "gtest":
		// For gtest, use the chromium result format and its result file.
		resultFileDir := testResult.TestRuns[0].GetTestCaseInfo().GetTestCaseResult().GetResultDirPath().GetPath()
		resultFilePath = path.Join(resultFileDir, ChromiumResultJSONFilePathForGtest)

		chromeosTags := fetchChromeosTags(testResult)
		for k, v := range chromeosTags {
			baseTags[k] = v
		}
	case "native":
		// For native, use the chromium result format and its result file.
		testCase := testResult.TestRuns[0].GetTestCaseInfo().GetTestCaseResult()
		resultFileDir := path.Join(testCase.GetResultDirPath().GetPath(), testCase.GetTestCaseMetadata().GetTestCase().GetName())
		resultFilePath = path.Join(resultFileDir, ChromiumResultJSONFilePathForNative)
	}

	config := &rdbclient.RdbStreamConfig{
		BaseTags:                baseTags,
		BaseVariant:             baseVariant,
		ResultFile:              resultFilePath,
		ResultFormat:            resultFormat,
		TesthausBaseURL:         testhausBaseURL,
		TestIdPrefix:            chromiumConfig["test_id_prefix"].(string),
		CoerceNegativeDuration:  chromiumConfig["coerce_negative_duration"].(bool),
		ExonerateUnexpectedPass: chromiumConfig["exonerate_unexpected_pass"].(bool),
		Include:                 chromiumConfig["include"].(bool),
	}

	log.Printf("successfully created the resultdb config for chromium tests: %v", config)
	return config, nil
}

// ingestEqCInfo ingests the 3D EqC info into the test invocation.
func ingestEqCInfo(testResult *artifact.TestResult, req *api.PublishRequest) {
	is3DRun := req.GetIs_3DRun()
	if !is3DRun {
		return
	}
	testInv := testResult.GetTestInvocation()
	testInv.Is_3DRun = is3DRun

	metadata, err := UnpackMetadata(req)
	if err != nil {
		log.Printf("Unpacking the metadata in the publish request: %s", err)
		return
	}

	testRuns := testResult.GetTestRuns()
	eqcHashMap := make(map[string]string)
	for _, items := range metadata.GetPublishKeys() {
		if items.GetSubject() == EQCSubjectKey {
			eqcInfoMap := items.GetKeyValues()
			tests := make([]string, 0, len(testRuns))
			err := json.Unmarshal([]byte(eqcInfoMap["eqcTests"]), &tests)
			if err != nil {
				log.Printf("Failed to unmarshal the associated tests from the EqC info: %s", eqcInfoMap)
				continue
			}

			for _, test := range tests {
				eqcHashMap[test] = eqcInfoMap["eqcHash"]
			}
		}
	}

	for _, testRun := range testRuns {
		testName := testRun.GetTestCaseInfo().GetTestCaseResult().GetTestCaseId().GetValue()
		eqcHash := eqcHashMap[testName]
		if eqcHash != "" {
			testRun.EqcInfo = &artifact.EqcInfo{
				EqcHash: eqcHashMap[testName],
			}
			log.Printf("Associated the EqC hash: %s with the test: %s", eqcHash, testName)
		}
	}

	log.Printf("successfully ingested the EqC info: %#v", eqcHashMap)
}

// ingestPostProcessResponses ingests the responses of the post process service
// into the test result.
func ingestPostProcessResponses(testResult *artifact.TestResult, postProcessResps *api.RunActivitiesResponse) {
	if postProcessResps == nil {
		log.Printf("post process response is empty")
		return
	}

	log.Printf("start to ingest post process responses: %#v", postProcessResps)

	buildMetadata := testResult.GetTestInvocation().GetPrimaryExecutionInfo().GetBuildInfo().GetBuildMetadata()
	for _, resp := range postProcessResps.Responses {
		switch resp.GetResponse().(type) {
		case *api.RunActivityResponse_GetFwInfoResponse:
			populateFirmwareInfo(buildMetadata, resp.GetGetFwInfoResponse())
			populateKernelInfo(buildMetadata, resp.GetGetFwInfoResponse())
			populateGSCFirmwareInfo(testResult, resp.GetGetFwInfoResponse())
		case *api.RunActivityResponse_GetGfxInfoResponse:
			populateGfxInfo(buildMetadata, resp.GetGetGfxInfoResponse())
		case *api.RunActivityResponse_GetAvlInfoResponse:
			populateAVLInfo(testResult, resp.GetGetAvlInfoResponse())
		case *api.RunActivityResponse_GetGscInfoResponse:
			populateGSCInfo(testResult, resp.GetGetGscInfoResponse())
		case *api.RunActivityResponse_GetServoInfoResponse:
			populateServoInfo(buildMetadata, resp.GetGetServoInfoResponse())
		default:
			log.Printf("post process response type: %T is not supported", resp.GetResponse())
		}
	}

	log.Printf("successfully ingested post process responses: %#v", postProcessResps)
}

// ingestFirmwareProvisionResponse ingests the response of the firmware
// provision service into the test result.
func ingestFirmwareProvisionResponse(buildMetadata *artifact.BuildMetadata, fwProvisionResponse *api.FirmwareProvisionResponse) {
	if fwProvisionResponse == nil {
		log.Printf("firmware provision response is empty")
		return
	}

	log.Printf("start to ingest firmware provision response: %s", fwProvisionResponse)

	// Fetches the existing firmware info if available in the build metadata.
	firmware := buildMetadata.GetFirmware()
	if firmware == nil {
		firmware = &artifact.BuildMetadata_Firmware{}
	}

	if fwProvisionResponse.GetApRoVersion() != "" {
		firmware.ApRoVersion = fwProvisionResponse.ApRoVersion
	}

	if fwProvisionResponse.GetApRwVersion() != "" {
		firmware.ApRwVersion = fwProvisionResponse.ApRwVersion
	}

	if fwProvisionResponse.GetEcRoVersion() != "" {
		firmware.EcRoVersion = fwProvisionResponse.EcRoVersion
	}

	if fwProvisionResponse.GetEcRwVersion() != "" {
		firmware.EcRwVersion = fwProvisionResponse.EcRwVersion
	}

	buildMetadata.Firmware = firmware

	log.Printf("successfully populated AP & EC firmware info from firmware provision response: %s", fwProvisionResponse)
}

// populateFirmwareInfo populates firmware info and kernel version.
func populateFirmwareInfo(buildMetadata *artifact.BuildMetadata, fwInfoResp *api.GetFWInfoResponse) {
	if fwInfoResp == nil {
		log.Printf("firmware info response is empty")
		return
	}

	log.Printf("start to populate firmware info: %#v", fwInfoResp)

	// Fetches the existing firmware info if available in the build metadata.
	firmware := buildMetadata.GetFirmware()
	if firmware == nil {
		firmware = &artifact.BuildMetadata_Firmware{}
	}

	if fwInfoResp.GetRoFwid() != "" {
		firmware.RoVersion = fwInfoResp.GetRoFwid()
	}
	if fwInfoResp.GetRwFwid() != "" {
		firmware.RwVersion = fwInfoResp.GetRwFwid()
	}
	buildMetadata.Firmware = firmware

	log.Printf("successfully populated firmware info: %#v", fwInfoResp)
}

// populateKernelInfo populates kernel info.
func populateKernelInfo(buildMetadata *artifact.BuildMetadata, fwInfoResp *api.GetFWInfoResponse) {
	if fwInfoResp == nil {
		log.Printf("firmware info response is empty")
		return
	}

	log.Printf("start to populate kernel info: %#v", fwInfoResp)

	kernel := &artifact.BuildMetadata_Kernel{}
	if fwInfoResp.GetKernelVersion() != "" {
		kernel.Version = fwInfoResp.GetKernelVersion()
	}
	buildMetadata.Kernel = kernel

	log.Printf("successfully populated kernel info: %#v", fwInfoResp)
}

// populateGSCFirmwareInfo populates GSC firmware info.
func populateGSCFirmwareInfo(testResult *artifact.TestResult, fwInfoResp *api.GetFWInfoResponse) {
	if fwInfoResp == nil {
		log.Printf("firmware info response is empty")
		return
	}

	log.Printf("start to populate GSC firmware info from: %#v", fwInfoResp)

	for _, testRun := range testResult.GetTestRuns() {
		testCaseInfo := testRun.GetTestCaseInfo()
		testCaseResult := testCaseInfo.GetTestCaseResult()
		if testCaseResult == nil {
			continue
		}
		testName := testCaseInfo.TestCaseResult.GetTestCaseId().GetValue()

		// Note that for devboard tests the GSC firmware info here comes from a
		// fake dut, and will be replaced by the values in gsc_info.json which
		// is processed by the populateGSCInfo function below.
		if testCaseInfo.GscInfo != nil {
			log.Printf("Skipped populating GSC firmware info because of existing GSC devboard info: %s for test: %q", testCaseInfo.GscInfo, testName)
			continue
		}

		gscInfo := &artifact.GscInfo{}
		if fwInfoResp.GscRo != "" {
			gscInfo.GscRoVersion = fwInfoResp.GscRo
		}

		if fwInfoResp.GscRw != "" {
			gscInfo.GscRwVersion = fwInfoResp.GscRw
		}
		testCaseInfo.GscInfo = gscInfo

		log.Printf("successfully populated GSC firmware info: %s for test: %q", testCaseInfo.GscInfo, testName)
	}

	log.Printf("successfully populated GSC firmware info from: %#v", fwInfoResp)
}

// populateGfxInfo populates graphics info.
func populateGfxInfo(buildMetadata *artifact.BuildMetadata, gfxInfoResp *api.GetGfxInfoResponse) {
	if gfxInfoResp == nil {
		log.Printf("graphics info response is empty")
		return
	}

	log.Printf("start to populate graphics info: %#v", gfxInfoResp)

	gfxInfo := &artifact.BuildMetadata_GfxInfo{}
	gfxLabels := gfxInfoResp.GetGfxLabels()
	if gfxLabels != nil {
		gfxInfo.DisplayPanelName = gfxLabels["display_panel_name"]
		gfxInfo.DisplayPresentHdr = gfxLabels["display_present_hdr"]
		gfxInfo.DisplayPresentPsr = gfxLabels["display_present_psr"]
		gfxInfo.DisplayPresentVrr = gfxLabels["display_present_vrr"]
		gfxInfo.DisplayRefreshRate = gfxLabels["display_refresh_rate"]
		gfxInfo.DisplayResolution = gfxLabels["display_resolution"]
		gfxInfo.GpuFamily = gfxLabels["gpu_family"]
		gfxInfo.GpuId = gfxLabels["gpu_id"]
		gfxInfo.GpuOpenGlesVersion = gfxLabels["gpu_open_gles_version"]
		gfxInfo.GpuVendor = gfxLabels["gpu_vendor"]
		gfxInfo.GpuVulkanVersion = gfxLabels["gpu_vulkan_version"]
		gfxInfo.PlatformCpuVendor = gfxLabels["platform_cpu_vendor"]
		gfxInfo.PlatformDiskSize, _ = strconv.ParseUint(gfxLabels["platform_disk_size"], 10, 64)
		gfxInfo.PlatformMemorySize, _ = strconv.ParseUint(gfxLabels["platform_memory_size"], 10, 64)
	}
	buildMetadata.GfxInfo = gfxInfo

	log.Printf("successfully populated graphics info: %#v", gfxInfoResp)
}

// populateAVLInfo populates AVL qualification info.
func populateAVLInfo(testResult *artifact.TestResult, avlInfoResp *api.GetAvlInfoResponse) {
	if avlInfoResp == nil {
		log.Printf("AVL info response is empty")
		return
	}

	log.Printf("start to populate AVL qual info: %#v", avlInfoResp)

	avlInfoMap := avlInfoResp.GetAvlInfos()
	for _, testRun := range testResult.GetTestRuns() {
		testCaseInfo := testRun.GetTestCaseInfo()
		testCaseResult := testCaseInfo.GetTestCaseResult()
		if testCaseResult == nil {
			continue
		}

		avlInfo := avlInfoMap[testCaseResult.GetTestCaseId().GetValue()]
		if avlInfo == nil {
			continue
		}

		testCaseAVLInfo := &artifact.AvlInfo{}
		if avlInfo.GetAvlComponentType() != "" {
			testCaseAVLInfo.AvlComponentType = avlInfo.GetAvlComponentType()
		}
		if avlInfo.GetAvlPartFirmware() != "" {
			testCaseAVLInfo.AvlPartFirmware = avlInfo.GetAvlPartFirmware()
		}
		if avlInfo.GetAvlPartModel() != "" {
			testCaseAVLInfo.AvlPartModel = avlInfo.GetAvlPartModel()
		}
		testCaseInfo.AvlInfo = testCaseAVLInfo
	}

	log.Printf("successfully populated AVL qual info: %#v", avlInfoResp)
}

// populateGSCInfo populates GSC devboard info.
func populateGSCInfo(testResult *artifact.TestResult, gscInfoResp *api.GetGscInfoResponse) {
	if gscInfoResp == nil {
		log.Printf("GSC devboard info response is empty")
		return
	}

	log.Printf("start to populate GSC devboard info: %#v", gscInfoResp)

	gscInfoMap := gscInfoResp.GetGscInfos()
	for _, testRun := range testResult.GetTestRuns() {
		testCaseInfo := testRun.GetTestCaseInfo()
		testCaseResult := testCaseInfo.GetTestCaseResult()
		if testCaseResult == nil {
			continue
		}

		gscInfoAny := gscInfoMap[testCaseResult.GetTestCaseId().GetValue()]
		if gscInfoAny == nil || len(gscInfoAny.Value) == 0 {
			continue
		}

		gscInfo := &artifact.GscInfo{}
		opts := proto.UnmarshalOptions{DiscardUnknown: true}
		if err := anypb.UnmarshalTo(gscInfoAny, gscInfo, opts); err != nil {
			log.Printf("failed to unmarshal GSC devboard info: %s", gscInfoAny)
			continue
		}
		testCaseInfo.GscInfo = gscInfo
	}

	log.Printf("successfully populated GSC devboard info: %#v", gscInfoResp)
}

// populateServoInfo populates servo info.
func populateServoInfo(buildMetadata *artifact.BuildMetadata, servoInfoResp *api.GetServoInfoResponse) {
	if servoInfoResp == nil {
		log.Printf("servo info response is empty")
		return
	}

	log.Printf("start to populate servo info: %#v", servoInfoResp)

	servoInfoAny := servoInfoResp.GetServoInfo()
	servoInfo := &artifact.BuildMetadata_ServoInfo{}
	opts := proto.UnmarshalOptions{DiscardUnknown: true}
	if err := anypb.UnmarshalTo(servoInfoAny, servoInfo, opts); err != nil {
		log.Printf("failed to unmarshal servo info: %s", servoInfoAny)
	}
	buildMetadata.ServoInfo = servoInfo

	log.Printf("successfully populated servo info: %#v", servoInfoResp)
}
