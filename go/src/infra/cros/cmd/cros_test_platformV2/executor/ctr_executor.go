// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executor

import (
	"context"
	"fmt"

	"go.chromium.org/chromiumos/config/go/test/api"
	testapi "go.chromium.org/chromiumos/config/go/test/api"

	managers "go.chromium.org/infra/cros/cmd/cros_test_platformV2/docker_managers"
)

// CrosProvisionExecutor represents executor
// for all crostoolrunner (ctr) related commands.
type CtrExecutor struct {
	Ctr                        managers.ContainerManager
	CrosProvisionServiceClient testapi.GenericProvisionServiceClient
	KeyLocation                string
}

func NewCtrExecutor(ctr managers.ContainerManager) *CtrExecutor {
	return &CtrExecutor{Ctr: ctr}
}

func (ex *CtrExecutor) Execute(ctx context.Context, cmd string, resp *api.InternalTestplan) (*api.InternalTestplan, error) {
	if cmd == "run" {
		fmt.Println("CTR Run.")
		return nil, nil //ex.gcloudAuthCommandExecution(ctx)
	} else if cmd == "init" {
		fmt.Println("CTR init")
		ex.Ctr.StartManager(ctx, "foo")
		return nil, nil
	} else if cmd == "stop" {
		fmt.Println("CTR stop")
		ex.Ctr.StopManager(ctx, "foo")
		return nil, nil
	}
	return nil, fmt.Errorf("invalid command given: %s", cmd)
}
