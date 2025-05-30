// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/oauth2"
	"google.golang.org/api/idtoken"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"
)

// ConnectWithService connects with the service at the provided server address.
func ConnectWithService(ctx context.Context, serverAddress string) (*grpc.ClientConn, error) {
	if serverAddress == "" {
		return nil, fmt.Errorf("cannot connect to empty service address")
	}
	var err error
	step, ctx := build.StartStep(ctx, "Connect to server")
	defer func() { step.End(err) }()

	logging.Infof(ctx, "Trying to connect with address %q with %s timeout", serverAddress, ServiceConnectionTimeout.String())
	ctx, cancel := context.WithTimeout(context.Background(), ServiceConnectionTimeout)
	defer cancel()
	// TODO(azrahman): remove deprecated use.
	conn, err := grpc.DialContext(ctx, serverAddress, getGrpcDialOpts(ctx)...)
	if err != nil {
		return nil, errors.Annotate(err, "error during connecting to service address %s: ", serverAddress).Err()
	}

	return conn, nil
}

// ConnectWithCloudService dials into a cloud service at the provided server endpoint,
// and authenticates with using the audience provided.
func ConnectWithCloudService(ctx context.Context, endpoint *labapi.IpEndpoint, audience string) (*grpc.ClientConn, error) {
	if endpoint == nil || endpoint.GetAddress() == "" {
		return nil, fmt.Errorf("Cannot connect to empty service address.")
	}
	serviceAddress := fmt.Sprintf("%s:%d", endpoint.GetAddress(), endpoint.GetPort())
	var err error
	step, ctx := build.StartStep(ctx, "Connect to server")
	defer func() { step.End(err) }()

	logging.Infof(ctx, "Trying to connect with address %q with %s timeout", serviceAddress, ServiceConnectionTimeout.String())
	ctx, cancel := context.WithTimeout(ctx, ServiceConnectionTimeout)
	defer cancel()
	conn, err := grpc.Dial(serviceAddress, getCloudGrpcDialOpts(ctx, endpoint.GetAddress(), audience)...)
	if err != nil {
		return nil, errors.Annotate(err, "error during connecting to service address %s: ", serviceAddress).Err()
	}

	return conn, nil
}

// getGrpcDialOpts provides the grpc dial options used
// to connect to a service.
func getGrpcDialOpts(ctx context.Context) []grpc.DialOption {
	opts := []grpc.DialOption{
		// TODO(azrahman): remove deprecated use.
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxFilterMsgSize)),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(MaxFilterMsgSize)),
	}

	return opts
}

type CloudRunAuth struct {
	Token *oauth2.Token
}

func (a *CloudRunAuth) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": "Bearer " + a.Token.AccessToken,
	}, nil
}

func (a *CloudRunAuth) RequireTransportSecurity() bool {
	return true
}

// getCloudGrpcDialOpts provides the grpc dial options used
// to connect to a cloud service.
func getCloudGrpcDialOpts(ctx context.Context, host, audience string) []grpc.DialOption {
	systemRoots, err := x509.SystemCertPool()
	if err != nil {
		logging.Infof(ctx, "could not get system cert pool: %s", err)
	}
	cred := credentials.NewTLS(&tls.Config{
		RootCAs: systemRoots,
	})

	tokenSource, err := idtoken.NewTokenSource(ctx, audience)
	if err != nil {
		logging.Infof(ctx, "failed getting service account credentials: %s", err)
	}
	token, err := tokenSource.Token()
	if err != nil {
		logging.Infof(ctx, "failed to get Token, %s", err)
	}
	opts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithAuthority(host),
		grpc.WithTransportCredentials(cred),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxFilterMsgSize)),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(MaxFilterMsgSize)),
		grpc.WithPerRPCCredentials(&CloudRunAuth{Token: token}),
	}

	return opts
}

// GetServerAddressFromGetContResponse gets the server address
// from get container response.
func GetServerAddressFromGetContResponse(resp *testapi.GetContainerResponse) (string, error) {
	if resp == nil || len(resp.Container.GetPortBindings()) == 0 {
		return "", fmt.Errorf("cannot retrieve address from empty response")
	}
	hostIP := resp.Container.GetPortBindings()[0].GetHostIp()
	hostPort := resp.Container.GetPortBindings()[0].GetHostPort()

	if hostIP == "" || hostPort == 0 {
		return "", fmt.Errorf("hostIp or HostPort is empty")
	}

	return fmt.Sprintf("%s:%v", hostIP, hostPort), nil
}

// GetIPEndpoint creates IpEndpoint from provided server address.
// Server address example: (address:port) -> localhost:8080.
func GetIPEndpoint(serverAddress string) (*labapi.IpEndpoint, error) {
	addressInfo := strings.Split(serverAddress, ":")
	if len(addressInfo) != 2 {
		return nil, fmt.Errorf("invalid dut server address")
	}
	port, err := strconv.Atoi(addressInfo[1])
	if err != nil {
		return nil, fmt.Errorf("error during extracting port info: %w", err)
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

// GetFreePort finds an available port on the running OS
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
func IsCqRun(testSuite []*testapi.TestSuite) bool {
	for _, suite := range testSuite {
		if cqRunPattern.MatchString(suite.Name) {
			return true
		}
	}
	return false
}
