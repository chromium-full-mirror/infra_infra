// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements the post-process for finding tests based on tags.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/artifact"

	"go.chromium.org/infra/cros/cmd/cft/common/portdiscovery"
	"go.chromium.org/infra/cros/cmd/cft/post-test/post-process/commands"
	"go.chromium.org/infra/cros/cmd/cft/post-test/post-process/common"
)

const (
	defaultRootPath        = "/tmp/test/post-process"
	defaultInputFileName   = "request.json"
	defaultOutputFileName  = "result.json"
	defaultTestMetadataDir = "/tmp/test/metadata"
)

// Version is the version info of this command. It is filled in during emerge.
var Version = "<unknown>"
var defaultPort = 8010

type args struct {
	// Common input params.
	logPath   string
	inputPath string
	output    string
	version   bool

	// Server mode params
	port        int
	dutendpoint string
}

func parseAndRunCmds(req *api.Request, log *log.Logger, dutClient api.DutServiceClient, testResult *artifact.TestResult) (*api.RunActivityResponse, error) {
	log.Printf("Start to run post process activity: %q", req)

	switch op := req.Request.(type) {
	case *api.Request_GetFwInfoRequest:
		log.Println("GettingFWInfo Task")
		cmdResp, err := commands.GetFwInfo(log, dutClient)
		if err != nil {
			return nil, err
		}
		return &api.RunActivityResponse{
			Response: &api.RunActivityResponse_GetFwInfoResponse{
				GetFwInfoResponse: cmdResp,
			},
		}, err
	case *api.Request_GetFilesFromDutRequest:
		log.Println("GetFilesFromDut Task")
		cmdResp, err := commands.GetFilesFromDUT(log, op.GetFilesFromDutRequest, dutClient)
		if err != nil {
			return nil, err
		}
		return &api.RunActivityResponse{
			Response: &api.RunActivityResponse_GetFilesFromDutResponse{
				GetFilesFromDutResponse: cmdResp,
			},
		}, err
	case *api.Request_GetGfxInfoRequest:
		log.Println("GetGfxInfo Task")
		cmdResp, err := commands.GetGfxInfo(log, dutClient)
		if err != nil {
			return nil, err
		}
		return &api.RunActivityResponse{
			Response: &api.RunActivityResponse_GetGfxInfoResponse{
				GetGfxInfoResponse: cmdResp,
			},
		}, err
	case *api.Request_GetAvlInfoRequest:
		log.Println("GetAvlInfo Task")
		cmdResp, err := commands.GetAvlInfo(log, testResult)
		if err != nil {
			return nil, err
		}
		return &api.RunActivityResponse{
			Response: &api.RunActivityResponse_GetAvlInfoResponse{
				GetAvlInfoResponse: cmdResp,
			},
		}, err
	case *api.Request_GetGscInfoRequest:
		log.Println("GetGscInfo Task")
		cmdResp := commands.GscInfo(log, testResult)
		return &api.RunActivityResponse{
			Response: &api.RunActivityResponse_GetGscInfoResponse{
				GetGscInfoResponse: cmdResp,
			},
		}, nil
	case *api.Request_GetServoInfoRequest:
		log.Println("GetServoInfo Task")
		cmdResp := commands.ServoInfo(log, testResult)
		return &api.RunActivityResponse{
			Response: &api.RunActivityResponse_GetServoInfoResponse{
				GetServoInfoResponse: cmdResp,
			},
		}, nil
	case *api.Request_GetUsbInfoRequest:
		log.Println("GetUsbInfo Task")
		cmdResp, err := commands.GetUsbInfo(log, testResult)
		if err != nil {
			return nil, err
		}
		return &api.RunActivityResponse{
			Response: &api.RunActivityResponse_GetUsbInfoResponse{
				GetUsbInfoResponse: cmdResp,
			},
		}, nil
	case *api.Request_GetStressTestInfoRequest:
		log.Println("GetStressTestInfo Task")
		cmdResp := commands.GetStressTestInfo(log, testResult)
		return &api.RunActivityResponse{
			Response: &api.RunActivityResponse_GetStressTestInfoResponse{
				GetStressTestInfoResponse: cmdResp,
			},
		}, nil
	default:
		log.Println("None")
		return nil, fmt.Errorf("can only be one of registered types")
	}
}

// startServer is the entry point for running post-process (PostTestService) in server mode.
func startServer(d []string) int {
	a := args{}
	defaultLogPath := "/tmp/post-process"
	fs := flag.NewFlagSet("Run post-process", flag.ExitOnError)
	fs.StringVar(&a.logPath, "log", defaultLogPath, fmt.Sprintf("Path to record finder logs. Default value is %s", defaultLogPath))
	fs.IntVar(&a.port, "port", defaultPort, fmt.Sprintf("Specify the port for the server. Default value %d.", defaultPort))
	fs.StringVar(&a.dutendpoint, "dutendpoint", "", "Specify the endpoint for the running dut-service in the form of ip:port")

	fs.Parse(d)

	logFile, err := common.CreateLogFile(a.logPath)
	if err != nil {
		log.Fatalln("Failed to create log file", err)
		return 2
	}
	defer logFile.Close()

	logger := common.NewLogger(logFile)

	if a.dutendpoint != "" {
		log.Println("warning: CLI arg 'dutendpoint' is deprecated, please use the StartUp RPC instead.")
	}

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", a.port))
	if err != nil {
		logger.Fatalln("Failed to create a net listener: ", err)
		return 2
	}
	logger.Println("Starting PostTestService at ", l.Addr().String())

	// Write port number to ~/.cftmeta for go/cft-port-discovery
	err = portdiscovery.WriteServiceMetadata("post-process", l.Addr().String(), logger)
	if err != nil {
		logger.Println("Warning: error when writing to metadata file: ", err)
	}

	server, closer, err := NewServer(logger, a.dutendpoint)
	defer closer()
	if err != nil {
		log.Println(err)
		return 2
	}
	err = server.Serve(l)
	if err != nil {
		logger.Fatalln("Failed to initialize server: ", err)
		return 2
	}
	return 0
}

// readInput reads a RunActivitiesRequest jsonproto file and returns a pointer to RunTestsRequest.
func readInput(fileName string) (*api.RunActivitiesRequest, error) {
	f, err := os.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("fail to read file %v: %v", fileName, err)
	}
	req := api.RunActivitiesRequest{}
	if err := protojson.Unmarshal(f, &req); err != nil {
		return nil, fmt.Errorf("fail to unmarshal file %v: %v", fileName, err)
	}
	return &req, nil
}

// writeOutput writes a RunActivityResponse json.
func writeOutput(output string, resp *api.RunActivitiesResponse) error {
	f, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("fail to create file %v: %v", output, err)
	}
	if json, err := protojson.Marshal(resp); err != nil {
		return fmt.Errorf("failed to marshall response to file %v: %v", output, err)
	} else {
		f.Write(json)
	}
	return nil
}

// runCLI is the entry point for running post-process (PostTestService) in CLI mode.
func runCLI(ctx context.Context, d []string) int {
	t := time.Now()
	defaultLogPath := filepath.Join(defaultRootPath, t.Format("20060102-150405"))
	defaultRequestFile := filepath.Join(defaultRootPath, defaultInputFileName)
	defaultResultFile := filepath.Join(defaultRootPath, defaultOutputFileName)

	a := args{}

	fs := flag.NewFlagSet("Run post-process", flag.ExitOnError)
	fs.StringVar(&a.logPath, "log", defaultLogPath, fmt.Sprintf("Path to record finder logs. Default value is %s", defaultLogPath))
	fs.StringVar(&a.inputPath, "input", defaultRequestFile, "specify the test finder request json input file")
	fs.StringVar(&a.output, "output", defaultResultFile, "specify the test finder request json input file")
	fs.StringVar(&a.dutendpoint, "dutendpoint", "", "Specify the endpoint for the running dut-service in the form of ip:port")
	fs.BoolVar(&a.version, "version", false, "print version and exit")
	fs.Parse(d)

	if a.version {
		fmt.Println("post-process version ", Version)
		return 0
	}

	logFile, err := common.CreateLogFile(a.logPath)
	if err != nil {
		log.Fatalln("Failed to create log file", err)
		return 2
	}
	defer logFile.Close()

	logger := common.NewLogger(logFile)

	logger.Println("post-process version ", Version)

	logger.Println("Reading input file: ", a.inputPath)
	req, err := readInput(a.inputPath)
	if err != nil {
		logger.Println("Error: ", err)
		return 2
	}

	if a.dutendpoint == "" {
		log.Fatalln("need dut-server endpoint")
		return 2
	}
	dutAddr := a.dutendpoint

	dutConn, err := grpc.Dial(dutAddr, grpc.WithInsecure())
	if err != nil {
		log.Printf("DutConn Failed!")
		return 2
	}

	defer dutConn.Close()
	dutClient := api.NewDutServiceClient(dutConn)
	log.Printf("Dut Conn Established")

	rspn, err := parseCommandsAndRun(req, logger, dutClient)
	if err != nil {
		return 2
	}

	logger.Println("Writing output file: ", a.output)
	if err := writeOutput(a.output, rspn); err != nil {
		logger.Println("Error: ", err)
		return 2
	}

	return 0
}

func parseCommandsAndRun(req *api.RunActivitiesRequest, log *log.Logger, dutClient api.DutServiceClient) (*api.RunActivitiesResponse, error) {
	log.Printf("Start to parse post process activities: %s", req)
	resp := &api.RunActivitiesResponse{
		Responses: make([]*api.RunActivityResponse, 0, len(req.Requests)),
	}

	testResult := &artifact.TestResult{}
	if req.TestResult != nil {
		if err := req.TestResult.UnmarshalTo(testResult); err != nil {
			return nil, fmt.Errorf("failed to convert test result proto, %s", err)
		}
	}

	for _, subReq := range req.Requests {
		r, err := parseAndRunCmds(subReq, log, dutClient, testResult)
		if err != nil {
			log.Printf("Failed to run post process activity: %s due to: %s", subReq, err.Error())
			continue
		}
		log.Printf("Successfully ran post process activity: %s with response: %s", subReq, r)
		resp.Responses = append(resp.Responses, r)
	}
	log.Printf("Sucessfully finished post process activities: %s", req)
	return resp, nil

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

func mainInternal(ctx context.Context) int {
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
		return startServer(os.Args[2:])
	case runVersion:
		log.Printf("TestFinderService version: %s", Version)
		return 0
	}
	return 0
}

func main() {
	os.Exit(mainInternal(context.Background()))
}
