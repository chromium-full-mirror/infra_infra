// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executor

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	buildapi "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/cros_test_platformV2/containers"
	managers "go.chromium.org/infra/cros/cmd/cros_test_platformV2/docker_managers"
)

type FilterExecutor struct {
	Ctr managers.ContainerManager

	conn *grpc.ClientConn

	binaryName    string
	containerPath string

	client testapi.GenericFilterServiceClient
}

func newFilterExecutor(ctr managers.ContainerManager, req *api.CTPFilter, containerMetadata map[string]*buildapi.ContainerImageInfo) (*FilterExecutor, error) {
	var err error
	// For non-default filters, the given request might not include the path. If not try to find
	// it from the containermetadata.
	if req.Container.Digest == "" {
		req, err = ResolvedContainer(req.Container.Name, containerMetadata)
		if err != nil {
			return nil, err
		}
	}

	// TODO, add configurable globals for project, regsitry.
	path := fmt.Sprintf("%s/%s/%s@%s", "us-docker.pkg.dev", "cros-registry/test-services", req.Container.Name, req.Container.Digest)
	fmt.Printf("Container path: %s\n\n", path)

	return &FilterExecutor{Ctr: ctr, binaryName: req.Container.Name, containerPath: path}, nil
}

func (ex *FilterExecutor) Execute(ctx context.Context, cmd string, resp *api.InternalTestplan) (*api.InternalTestplan, error) {
	if cmd == "run" {
		return ex.run(resp)
	} else if cmd == "init" {
		fmt.Println("FILTER INIT!")
		err := ex.init()
		return nil, err
	} else if cmd == "stop" {
		return nil, nil // Stop containers.
	}
	return nil, fmt.Errorf("invalid command given: %s\n", cmd)
}

func (ex *FilterExecutor) run(req *api.InternalTestplan) (*api.InternalTestplan, error) {
	resp, err := ex.client.Execute(context.Background(), req)
	if err != nil {
		return resp, fmt.Errorf("err running filter: %w", err)
	}

	return resp, nil
}

// init starts the container, creates a client.
func (ex *FilterExecutor) init() error {
	fmt.Println("Starting container")
	ctx := context.Background()
	// Call build the genericTempaltedContainer interface.
	container := containers.NewContainer(ex.binaryName, ex.containerPath, ex.Ctr)
	template := &api.Template{Container: &api.Template_Generic{
		Generic: &testapi.GenericTemplate{
			DockerArtifactDir: fmt.Sprintf("/tmp/%s", ex.binaryName),
			BinaryArgs: []string{
				"server", "-port", "0",
			},
			BinaryName: ex.binaryName,
		},
	}}

	// Process does the init, run, getServer.
	serverAddress, err := container.ProcessContainer(ctx, template)
	if err != nil {
		fmt.Printf("error processing container:%s \n", err)

		return errors.Annotate(err, "error processing container: ").Err()
	}
	fmt.Println("Started container")

	// Connect with the service.
	conn, err := common.ConnectWithService(ctx, serverAddress)
	if err != nil {
		fmt.Printf(
			"error during connecting with %s server at %s: %s",
			ex.binaryName,
			serverAddress,
			err.Error())
		return err
	}
	ex.conn = conn
	fmt.Println("Connected to container")

	filterClient := api.NewGenericFilterServiceClient(conn)
	if filterClient == nil {
		return fmt.Errorf("could not connect to GenericFilterClient: %w", err)
	}
	ex.client = filterClient

	return nil

}
