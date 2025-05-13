// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package state_machine

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/provision/android-provision/common"
	"go.chromium.org/infra/cros/cmd/provision/android-provision/service"
	"go.chromium.org/infra/cros/cmd/provision/android-provision/state-machine/commands"
	commonutils "go.chromium.org/infra/cros/cmd/provision/common-utils"
)

type OSInstallState struct {
	svc *service.AndroidService
}

func (s OSInstallState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	log.Println("State: Execute AndroidOSInstallState")
	ctx = context.WithValue(ctx, common.StageCtxKey, common.OSInstall)
	cmds := []commonutils.CommandInterface{
		commands.NewRebootToBootloaderCommand(ctx, s.svc),
		commands.NewFlashOsCommand(ctx, s.svc),
		commands.NewCleanupCommand(ctx, s.svc),
	}
	for i, c := range cmds {
		if err := c.Execute(log); err != nil {
			log.Printf("State: Execute AndroidOSInstallState failure %s\n", err)
			log.Println("State: Revert AndroidOSInstallState")
			for ; i >= 0; i-- {
				if e := cmds[i].Revert(); e != nil {
					err = errors.Annotate(err, "failure while reverting %s", e).Err()
					break
				}
			}
			return nil, c.GetStatus(), fmt.Errorf("%s: %w", c.GetErrorMessage(), err)
		}
	}
	log.Println("State: AndroidOSInstallState Completed")
	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

func (s OSInstallState) Next() commonutils.ServiceState {
	return PackageFetchState(s)
}

func (s OSInstallState) Name() string {
	return "Android OS Install State"
}
