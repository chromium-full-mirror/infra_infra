// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"fmt"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.chromium.org/chromiumos/config/go/test/api"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"
)

// ConnectWithService connects with the service at the provided server address.
func ConnectWithService(ctx context.Context, serverAddress string) (*grpc.ClientConn, error) {
	if serverAddress == "" {
		return nil, fmt.Errorf("Cannot connect to empty service address.")
	}
	var err error
	step, ctx := build.StartStep(ctx, "Connect to server")
	defer func() { step.End(err) }()

	logging.Infof(ctx, "Trying to connect with address %q with %s timeout", serverAddress, ServiceConnectionTimeout.String())
	ctx, cancel := context.WithTimeout(context.Background(), ServiceConnectionTimeout)
	defer cancel()
	conn, err := grpc.DialContext(ctx, serverAddress, getGrpcDialOpts(ctx)...)
	if err != nil {
		return nil, errors.Annotate(err, "error during connecting to service address %s: ", serverAddress).Err()
	}

	return conn, nil
}

// getGrpcDialOpts provides the grpc dial options used
// to connect to a service.
func getGrpcDialOpts(ctx context.Context) []grpc.DialOption {
	opts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(32 * 1024 * 1024)),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(32 * 1024 * 1024)),
	}

	return opts
}

// GetServerAddressFromGetContResponse gets the server address
// from get container response.
func GetServerAddressFromGetContResponse(resp *testapi.GetContainerResponse) (string, error) {
	if resp == nil || len(resp.Container.GetPortBindings()) == 0 {
		return "", fmt.Errorf("Cannot retrieve address from empty response.")
	}
	hostIp := resp.Container.GetPortBindings()[0].GetHostIp()
	hostPort := resp.Container.GetPortBindings()[0].GetHostPort()

	if hostIp == "" || hostPort == 0 {
		return "", fmt.Errorf("HostIp or HostPort is empty.")
	}

	return fmt.Sprintf("%s:%v", hostIp, hostPort), nil
}

// GetIpEndpoint creates IpEndpoint from provided server address.
// Server address example: (address:port) -> localhost:8080.
func GetIpEndpoint(serverAddress string) (*labapi.IpEndpoint, error) {
	addressInfo := strings.Split(serverAddress, ":")
	if len(addressInfo) != 2 {
		return nil, fmt.Errorf("invalid dut server address!")
	}
	port, err := strconv.Atoi(addressInfo[1])
	if err != nil {
		return nil, fmt.Errorf("error during extracting port info: %s", err)
	}

	return &labapi.IpEndpoint{Address: addressInfo[0], Port: int32(port)}, nil
}

// GetServerAddress creates a string from provided IpEndpoint.
// Example: IpEndpoint{Address:localhost, port:8080} -> localhost:8080.
func GetServerAddress(endpoint *labapi.IpEndpoint) string {
	if endpoint == nil {
		return ""
	}

	return fmt.Sprintf("%s:%d", endpoint.GetAddress(), endpoint.GetPort())
}

// Finds an available port on the running OS
// to prevent collisions between services
func GetFreePort() uint16 {
	l, err := net.Listen("tcp", ":0")
	defer l.Close()
	if err != nil {
		log.Fatalf("Failed to get port, %s", err)
	}

	return uint16(l.Addr().(*net.TCPAddr).Port)
}

// cqRunPattern matches precisely the suite names for CQ runs.
var cqRunPattern = regexp.MustCompile(`^bvt-tast-cq.*|^cq-.*`)

// IsCqRun determines if the current execution is a CQ run
func IsCqRun(testSuite []*api.TestSuite) bool {
	for _, suite := range testSuite {
		if cqRunPattern.MatchString(suite.Name) {
			return true
		}
	}
	return false
}
