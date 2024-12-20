// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mock_servod_client generate servod service request to test
// the servod server.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	api_xmlrpc "go.chromium.org/chromiumos/config/go/api/test/xmlrpc"
	"go.chromium.org/chromiumos/config/go/test/api"
)

func mainInternal() int {
	if len(os.Args) < 2 {
		log.Println("No command specifiec")
		return 1
	}
	ctx := context.Background()
	switch strings.ToLower(os.Args[1]) {
	case "run":
		log.Printf("Testing servod service")
		return run(ctx, os.Args[2:])
	case "generate":
		log.Printf("Generating example request file")
		return generate(ctx, os.Args[2:])
	default:
		log.Printf("Unknown command: %s", os.Args[1])
		return 1
	}
}

type servoInfo struct {
	servoNexusClient api.ServodServiceClient
	grpcConn         *grpc.ClientConn
	requests         *ServoServiceRequests
}

type ServoServiceRequests struct {
	Requests []*ServoRequest `json:"requests"`
}

type ServoRequest struct {
	StartServodRequest   *api.StartServodRequest   `json:"startServoRequest,omitempty"`
	StopServodRequest    *api.StopServodRequest    `json:"stopServoRequest,omitempty"`
	ExecCmdRequest       *api.ExecCmdRequest       `json:"execCmdRequest,omitempty"`
	CallServodRequest    *CallServodRequest        `json:"callServodRequest,omitempty"`
	LogCheckPointRequest *api.LogCheckPointRequest `json:"logCheckPointRequest,omitempty"`
	SaveLogsRequest      *api.SaveLogsRequest      `json:"saveLogsRequest,omitempty"`
}

type CallServodRequest struct {
	ServoHostPath             string
	ServodDockerContainerName string
	ServodPort                int32
	Method                    int      `json:"method,omitempty"`
	Args                      []string `json:"args,omitempty"`
}

type generateOptions struct {
	servoHostPath string
	// The servod Docker container name.
	servodDockerContainerName string
	// The --PORT parameter value for servod command.
	servodPort int32
	// The --BOARD parameter value for servod command.
	board string
	// The --MODEL parameter value for servod command.
	model string
	// The --SERIALNAME parameter value for servod command.
	serialName string
}

func generate(ctx context.Context, d []string) int {
	o := generateOptions{}
	fs := flag.NewFlagSet("Generate examples", flag.ExitOnError)
	fs.StringVar(&o.servoHostPath, "servo_host_path", "", "The servo host.")
	fs.StringVar(&o.servodDockerContainerName, "servod_docker_container_name", "", "The servod container.")
	var port int
	fs.IntVar(&port, "servo_port", 9999, "The servo port.")
	fs.StringVar(&o.board, "board", "", "The board for the DUT.")
	fs.StringVar(&o.model, "model", "", "The model for the DUT.")
	fs.StringVar(&o.serialName, "serial", "", "The serial name for the servo.")

	var outputFile string
	fs.StringVar(&outputFile, "output", "req.json", "The path of the output file ")
	if err := fs.Parse(d); err != nil {
		log.Fatal("Failed to parse command line input: ", err)
	}
	o.servodPort = int32(port)

	req := o.generateRequests(ctx)
	b, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile(outputFile, b, 0666); err != nil {
		log.Fatal(err)
	}
	return 0
}

func (o *generateOptions) generateRequests(ctx context.Context) *ServoServiceRequests {
	servoHostPath := o.servoHostPath
	if servoHostPath == "" && o.servodDockerContainerName == "" {
		servoHostPath = "servohost"
	}

	return &ServoServiceRequests{
		Requests: []*ServoRequest{
			{
				StartServodRequest: &api.StartServodRequest{
					ServoHostPath:             servoHostPath,
					ServodDockerContainerName: o.servodDockerContainerName,
					ServodPort:                o.servodPort,
					Board:                     o.board,
					Model:                     o.model,
					SerialName:                o.serialName,
				},
			},
			{
				ExecCmdRequest: &api.ExecCmdRequest{
					ServoHostPath:             servoHostPath,
					ServodDockerContainerName: o.servodDockerContainerName,
					Command:                   "hostname",
				},
			},
			{
				CallServodRequest: &CallServodRequest{
					ServoHostPath:             servoHostPath,
					ServodDockerContainerName: o.servodDockerContainerName,
					ServodPort:                o.servodPort,
					Method:                    int(api.CallServodRequest_GET),
					Args: []string{
						"servo_type",
					},
				},
			},
			{
				SaveLogsRequest: &api.SaveLogsRequest{
					ServoHostPath:             servoHostPath,
					ServodDockerContainerName: o.servodDockerContainerName,
					Dest:                      "./logs",
					ServodPorts:               []int32{o.servodPort},
				},
			},
			{
				StopServodRequest: &api.StopServodRequest{
					ServoHostPath:             servoHostPath,
					ServodDockerContainerName: o.servodDockerContainerName,
					ServodPort:                o.servodPort,
				},
			},
		},
	}
}

func run(ctx context.Context, d []string) int {
	s := servoInfo{}
	fs := flag.NewFlagSet("Run servo service mock tests", flag.ExitOnError)
	var servoNexusHost string
	fs.StringVar(&servoNexusHost, "servo_nexus_host", "", "The servo nexus host.")
	var inputFile string
	fs.StringVar(&inputFile, "input", "req.json", "The path of the file ")
	if err := fs.Parse(d); err != nil {
		log.Fatal("Failed to parse command line input: ", err)
	}

	b, err := os.ReadFile(inputFile)
	if err != nil {
		log.Fatal("ReadFile: ", err)
	}

	err = json.Unmarshal(b, &s.requests)
	if err != nil {
		log.Fatal("Failed to unmarshal input: ", err)
	}

	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:    150 * time.Minute,
			Timeout: 150 * time.Minute,
		}),
	}
	s.grpcConn, err = grpc.NewClient(servoNexusHost, dialOpts...)
	if err != nil {
		log.Fatal(err)
	}
	defer s.grpcConn.Close()

	s.servoNexusClient = api.NewServodServiceClient(s.grpcConn)

	if err := s.mockRequests(ctx); err != nil {
		log.Fatal(err)
	}

	return 0
}

func (s *servoInfo) mockRequests(ctx context.Context) error {
	for _, r := range s.requests.Requests {
		if r.StartServodRequest != nil {
			if err := s.startServod(ctx, r.StartServodRequest); err != nil {
				log.Printf("failed to start servod: %v", err)
			}
			continue
		}
		if r.StopServodRequest != nil {
			if err := s.stopServod(ctx, r.StopServodRequest); err != nil {
				log.Printf("failed to stop servod: %v", err)
			}
			continue
		}
		if r.ExecCmdRequest != nil {
			if err := s.execCmd(ctx, r.ExecCmdRequest); err != nil {
				log.Printf("failed to execute command: %v", err)
			}
			continue
		}
		if r.CallServodRequest != nil {
			if err := s.callServod(ctx, r.CallServodRequest); err != nil {
				log.Printf("failed to call servod: %v", err)
			}
			continue
		}
		if r.LogCheckPointRequest != nil {
			if err := s.logCheckPoint(ctx, r.LogCheckPointRequest); err != nil {
				log.Printf("failed to checkpoint on logs: %v", err)
			}
			continue
		}
		if r.SaveLogsRequest != nil {
			if err := s.saveLogs(ctx, r.SaveLogsRequest); err != nil {
				log.Printf("failed to save logs: %v", err)
			}
			continue
		}
	}
	return nil
}

func (s *servoInfo) startServod(ctx context.Context, req *api.StartServodRequest) error {
	rspn, err := s.servoNexusClient.StartServod(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to call StartServod: %w", err)
	}
	// TODO: add waiting for the response.
	if s := rspn.GetError(); s != nil {
		// Error here is a failure in the provision attempt.
		return fmt.Errorf("received error while waiting for StartServod response: %w", err)
	}
	log.Println("StartServod was successful")
	return nil
}

func (s *servoInfo) stopServod(ctx context.Context, req *api.StopServodRequest) error {
	rspn, err := s.servoNexusClient.StopServod(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to call StopServod: %w", err)
	}
	// TODO: add waiting for the response.
	if s := rspn.GetError(); s != nil {
		// Error here is a failure in the provision attempt.
		return fmt.Errorf("received error while waiting for StopServod response: %w", err)
	}
	log.Println("StopServod was successful")
	return nil
}

func (s *servoInfo) execCmd(ctx context.Context, req *api.ExecCmdRequest) error {
	rspn, err := s.servoNexusClient.ExecCmd(ctx, req)
	if err != nil {
		return err
	}
	output := string(rspn.Stdout)
	errMsg := string(rspn.Stderr)
	log.Printf("ExecCmd Response: stdout: %q  -- stderr: %q\n", output, errMsg)
	return nil
}

func (s *servoInfo) callServod(ctx context.Context, req *CallServodRequest) error {
	var args []*api_xmlrpc.Value
	for _, arg := range req.Args {
		args = append(args, &api_xmlrpc.Value{ScalarOneof: &api_xmlrpc.Value_String_{String_: arg}})
	}
	rspn, err := s.servoNexusClient.CallServod(ctx,
		&api.CallServodRequest{
			ServoHostPath:             req.ServoHostPath,
			ServodDockerContainerName: req.ServodDockerContainerName,
			ServodPort:                req.ServodPort,
			Method:                    api.CallServodRequest_Method(req.Method),
			Args:                      args,
		},
	)
	if err != nil {
		return err
	}
	if rspn.GetSuccess() != nil {
		log.Printf("ExecCmd successful response: %s", rspn.GetSuccess().GetResult().GetString_())
	}
	if rspn.GetFailure() != nil {
		log.Printf("ExecCmd failure response: %s", rspn.GetFailure().GetErrorMessage())
	}
	return nil
}

func (s *servoInfo) logCheckPoint(ctx context.Context, req *api.LogCheckPointRequest) error {
	rspn, err := s.servoNexusClient.LogCheckPoint(ctx, req)
	if err != nil {
		return err
	}
	b, err := json.Marshal(rspn)
	if err != nil {
		return fmt.Errorf("failed to marshal LogCheckPoint response: %w", err)
	}
	log.Printf("LogCheckPoint Response: %s", string(b))
	return nil
}

func (s *servoInfo) saveLogs(ctx context.Context, req *api.SaveLogsRequest) error {
	_, err := s.servoNexusClient.SaveLogs(ctx, req)
	if err != nil {
		return err
	}
	log.Println("SaveLogs request was successful")
	return nil
}

func main() {
	os.Exit(mainInternal())
}
