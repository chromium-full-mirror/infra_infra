// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package test_finder implements the cros-test-finder for finding tests based on tags.
package test_finder

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/exp/maps"
	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/cft/common/errors"
	"go.chromium.org/infra/cros/cmd/cft/common/finder"
	"go.chromium.org/infra/cros/cmd/cft/common/metadata"
	"go.chromium.org/infra/cros/cmd/cft/common/portdiscovery"
	"go.chromium.org/infra/cros/cmd/cft/cros-test-finder/centralizedsuite"
	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/servertemplate"
)

const (
	defaultRootPath        = "/tmp/test/cros-test-finder"
	DefaultLogPath         = "/tmp/filters"
	defaultInputFileName   = "request.json"
	defaultOutputFileName  = "result.json"
	defaultTestMetadataDir = "/tmp/test/metadata"
)

var errInvalidRequest = fmt.Errorf("invalid cros-test-finder input request")
var errMissingTestMetadata = fmt.Errorf("test has no metadata")

// createLogFile creates a file and its parent directory for logging purpose.
func createLogFile(fullPath string) (*os.File, error) {
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return nil, errors.NewStatusError(errors.IOCreateError,
			fmt.Errorf("failed to create directory %v: %w", fullPath, err))
	}

	logFullPathName := filepath.Join(fullPath, "log.txt")

	// Log the full output of the command to disk.
	logFile, err := os.Create(logFullPathName)
	if err != nil {
		return nil, errors.NewStatusError(errors.IOCreateError,
			fmt.Errorf("failed to create file %v: %w", fullPath, err))
	}
	return logFile, nil
}

// newLogger creates a logger. Using go default logger for now.
func newLogger(logFiles ...*os.File) *log.Logger {
	writers := []io.Writer{os.Stderr}
	for _, logFile := range logFiles {
		writers = append(writers, logFile)
	}
	mw := io.MultiWriter(writers...)
	return log.New(mw, "", log.LstdFlags|log.LUTC)
}

// readInput reads a CrosTestFinderRequest jsonproto file and returns a pointer to RunTestsRequest.
func readInput(fileName string) (*api.CrosTestFinderRequest, error) {
	f, err := os.ReadFile(fileName)
	if err != nil {
		return nil, errors.NewStatusError(errors.IOAccessError,
			fmt.Errorf("fail to read file %v: %w", fileName, err))
	}
	req := api.CrosTestFinderRequest{}
	if err := protojson.Unmarshal(f, &req); err != nil {
		return nil, errors.NewStatusError(errors.UnmarshalError,
			fmt.Errorf("fail to unmarshal file %v: %w", fileName, err))
	}
	return &req, nil
}

// writeOutput writes a CrosTestFinderResponse json.
func writeOutput(output string, resp *api.CrosTestFinderResponse) error {
	f, err := os.Create(output)
	if err != nil {
		return errors.NewStatusError(errors.IOCreateError,
			fmt.Errorf("fail to create file %v: %w", output, err))
	}
	if json, err := protojson.Marshal(resp); err != nil {
		return errors.NewStatusError(errors.MarshalError,
			fmt.Errorf("failed to marshall response to file %v: %w", output, err))
	} else {
		_, _ = f.Write(json)
	}
	return nil
}

// combineTestSuiteNames combines a list of test suite names to one single name.
func combineTestSuiteNames(suites []*api.TestSuite) string {
	if len(suites) == 0 {
		return "CombinedSuite"
	}
	var names []string
	for _, s := range suites {
		names = append(names, s.Name)
	}
	return strings.Join(names, ",")
}

// metadataToTestSuite convert a list of test metadata to a test suite.
func metadataToTestSuite(name string, mdList []*api.TestCaseMetadata, metadataRequired bool) *api.TestSuite {

	if metadataRequired {
		return &api.TestSuite{
			Name: name,
			Spec: &api.TestSuite_TestCasesMetadata{
				TestCasesMetadata: &api.TestCaseMetadataList{Values: mdList},
			},
		}
	}
	testInfos := []*api.TestCase{}
	for _, md := range mdList {
		testInfos = append(testInfos, &api.TestCase{
			Id:           md.GetTestCase().GetId(),
			Dependencies: md.GetTestCase().GetDependencies(),
		})
	}
	return &api.TestSuite{
		Name: name,
		Spec: &api.TestSuite_TestCases{
			TestCases: &api.TestCaseList{TestCases: testInfos},
		},
	}

}

// testsToMetadata converts the given set of tests to a list of metadata for the
// tests; returns errMissingTestMetadata if a test is not in the given metadata
// list.
func testsToMetadata(tests map[string]struct{}, metadataList []*api.TestCaseMetadata) ([]*api.TestCaseMetadata, error) {
	metadataMap := make(map[string]*api.TestCaseMetadata, len(metadataList))
	for _, metadata := range metadataList {
		testID := metadata.GetTestCase().GetId().GetValue()
		metadataMap[testID] = metadata
	}

	testList := maps.Keys(tests)
	filteredMetadata := make([]*api.TestCaseMetadata, len(testList))
	for i, test := range testList {
		metadata, ok := metadataMap[test]
		if !ok {
			return nil, fmt.Errorf("%w: %v", errMissingTestMetadata, test)
		}
		filteredMetadata[i] = metadata
	}
	return filteredMetadata, nil
}

// matchedTestsForCentralizedSuite returns a list of testMetadata for all the tests in
// the given centralized suite.
func matchedTestsForCentralizedSuite(mappingsLoader centralizedsuite.MappingsLoader, metadataList []*api.TestCaseMetadata, centralizedSuiteID string) ([]*api.TestCaseMetadata, error) {
	mappings, err := mappingsLoader.Load()
	if err != nil {
		return nil, err
	}
	tests, err := mappings.TestsIn(centralizedSuiteID)
	if err != nil {
		return nil, err
	}
	return testsToMetadata(tests, metadataList)
}

// getSelectedTestMetadata will get the tests & their metadata from the provided
// suite OR centralized suite, cannot be both.
func getSelectedTestMetadata(mappingsLoader centralizedsuite.MappingsLoader, metadataList []*api.TestCaseMetadata, req *api.CrosTestFinderRequest) (string, []*api.TestCaseMetadata, error) {
	testSuites := req.GetTestSuites()
	centralizedSuiteID := req.GetCentralizedSuite()
	if len(testSuites) > 0 && centralizedSuiteID != "" {
		return "", nil, fmt.Errorf("%w, cannot provide both a SuiteSet and Suites in the same request, must provide one or the other", errInvalidRequest)
	}

	if req.GetCentralizedSuite() != "" {
		selectedTestMetadata, err := matchedTestsForCentralizedSuite(mappingsLoader, metadataList, centralizedSuiteID)
		return centralizedSuiteID, selectedTestMetadata, err
	}

	selectedTestMetadata, err := finder.MatchedTestsForSuites(metadataList, testSuites)
	return combineTestSuiteNames(testSuites), selectedTestMetadata, err
}

// Version is the version info of this command. It is filled in during emerge.
var Version = "<unknown>"
var defaultPort = 8010

type args struct {
	// Common input params.
	logPath     string
	inputPath   string
	output      string
	metadataDir string
	version     bool

	// Server mode params
	port int
}

func FindTests(logger *log.Logger, req *api.CrosTestFinderRequest, metadataDir string) (*api.CrosTestFinderResponse, error) {
	logger.Println("Reading metadata from directory: ", metadataDir)

	allTestMetadata, err := metadata.ReadDir(metadataDir)
	if err != nil {
		logger.Println("Error: ", err)
		return nil, errors.NewStatusError(errors.IOCreateError,
			fmt.Errorf("failed to read directory %v: %w", metadataDir, err))
	}

	mappingsLoader := centralizedsuite.NewFileLoader()
	suiteName, selectedTestMetadata, err := getSelectedTestMetadata(mappingsLoader, allTestMetadata.Values, req)
	if err != nil {
		logger.Println("Error: ", err)
		return nil, err
	}

	resultTestSuite := metadataToTestSuite(suiteName, selectedTestMetadata, req.GetMetadataRequired())

	rspn := &api.CrosTestFinderResponse{TestSuites: []*api.TestSuite{resultTestSuite}}
	return rspn, nil

}

// runCLI is the entry point for running cros-test (TestFinderService) in CLI mode.
func runCLI(ctx context.Context, d []string) int {
	t := time.Now()
	defaultLogPath := filepath.Join(defaultRootPath, t.Format("20060102-150405"))
	defaultRequestFile := filepath.Join(defaultRootPath, defaultInputFileName)
	defaultResultFile := filepath.Join(defaultRootPath, defaultOutputFileName)

	a := args{}

	fs := flag.NewFlagSet("Run cros-test-finder", flag.ExitOnError)
	fs.StringVar(&a.logPath, "log", defaultLogPath, fmt.Sprintf("Path to record finder logs. Default value is %s", defaultLogPath))
	fs.StringVar(&a.inputPath, "input", defaultRequestFile, "specify the test finder request json input file")
	fs.StringVar(&a.output, "output", defaultResultFile, "specify the test finder request json input file")
	fs.StringVar(&a.metadataDir, "metadatadir", defaultTestMetadataDir, "specify a directory that contain all test metadata proto files.")
	fs.BoolVar(&a.version, "version", false, "print version and exit")
	fs.Parse(d)

	if a.version {
		fmt.Println("cros-test-finder version ", Version)
		return 0
	}

	logFile, err := createLogFile(a.logPath)
	if err != nil {
		log.Fatalln("Failed to create log file", err)
		return 2
	}
	defer logFile.Close()

	logger := newLogger(logFile)
	logger.Println("cros-test-finder version ", Version)

	logger.Println("Reading input file: ", a.inputPath)
	req, err := readInput(a.inputPath)
	if err != nil {
		logger.Println("Error: ", err)
		return errors.WriteError(os.Stderr, err)
	}

	rspn, err := FindTests(logger, req, a.metadataDir)
	if err != nil {
		return 2
	}

	logger.Println("Writing output file: ", a.output)
	if err := writeOutput(a.output, rspn); err != nil {
		logger.Println("Error: ", err)
		return errors.WriteError(os.Stderr, err)
	}

	return 0
}

// startServer is the entry point for running cros-test-finder (TestFinderService) in server mode.
func startServer(flagSet *flag.FlagSet, executorGenerator servertemplate.ExecutorGeneratorFunc, name string) error {
	a := args{}
	t := time.Now()
	flagSet.StringVar(&a.logPath, "log", DefaultLogPath, fmt.Sprintf("Base path to record logs. Default value is %s", DefaultLogPath))
	flagSet.IntVar(&a.port, "port", defaultPort, fmt.Sprintf("Specify the port for the server. Default value %d.", defaultPort))

	flagSet.Parse(os.Args[2:])

	logFile, err := common.CreateLogFile(filepath.Join(a.logPath, name, t.Format("20060102-150405")))
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()

	logger := common.NewLogger(logFile)
	log.SetOutput(logger.Writer())

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", a.port))
	if err != nil {
		return fmt.Errorf("failed to create a net listener: %w", err)
	}
	// Write port number to ~/.cftmeta for go/cft-port-discovery
	err = portdiscovery.WriteServiceMetadata(name, l.Addr().String(), logger)
	if err != nil {
		return fmt.Errorf("failed to write metadata port: %w", err)
	}
	server := NewServer(logger, a.logPath, name, executorGenerator)

	err = server.Serve(l)
	if err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}
	logger.Println("Starting generic filter service on port ", a.port)

	return nil
}

// Specify run mode for CLI.
type runMode string

const (
	runCli     runMode = "cli"
	runServer  runMode = "server"
	runVersion runMode = "version"
	runHelp    runMode = "help"

	runCliDefault runMode = "cliDefault"
)

func getRunMode() (runMode, error) {
	if len(os.Args) > 1 {
		for _, a := range os.Args {
			if a == "-version" {
				return runVersion, nil
			}
		}
		switch strings.ToLower(os.Args[1]) {
		case "cli":
			return runCli, nil
		case "server":
			return runServer, nil
		case "help":
			return runHelp, nil
		}
	}

	// If we did not find special run mode then just run CLI to match legacy behavior.
	return runCliDefault, nil
}

type TestFinderFilter struct {
	servertemplate.Filter
}

func NewTestFinderFilter() servertemplate.Filter {
	return &TestFinderFilter{}
}

func (*TestFinderFilter) Executor(req *api.InternalTestplan, log *log.Logger, commonParams *common.CommonFilterParams) (*api.InternalTestplan, error) {
	log.Println("Executing cros-test-finder as filter.")

	findTestReq, _ := common.ToTestFinderRequest(req)

	log.Printf("Custom TF Adaptor Request: %s", req)

	findTestResp, err := FindTests(log, findTestReq, defaultTestMetadataDir)
	if err != nil {
		// log error but don't return it as we want enumeration error happening for this
		log.Printf("filter grpc execution failure: %s", err.Error())
		return req, nil
	}

	log.Println("Backfilling results")
	err = common.FillTestCasesIntoTestPlan(req, findTestResp)

	return req, nil
}

func TestFinderInternal(ctx context.Context) int {
	runMode, err := getRunMode()
	if err != nil {
		log.Fatalln(err)
		return 2
	}
	switch runMode {

	case runCliDefault:
		log.Printf("No mode specified, assuming CLI.")
		return runCLI(ctx, os.Args[1:])
	case runCli:
		log.Printf("Running CLI mode!")
		return runCLI(ctx, os.Args[2:])
	case runServer:
		log.Printf("Running server mode!")
		fs := flag.NewFlagSet("Run cros-test-finder server", flag.ExitOnError)
		err := startServer(fs, NewTestFinderFilter, "cros-test-finder")
		if err != nil {
			return 2
		}
		return 0
	case runVersion:
		log.Printf("TestFinderService version: %s", Version)
		return 0
	}
	return 0
}
