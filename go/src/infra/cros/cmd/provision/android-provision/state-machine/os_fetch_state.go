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

type OSFetchState struct {
	svc *service.AndroidService
}

func (s OSFetchState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	log.Println("State: Execute AndroidOSFetchState")
	ctx = context.WithValue(ctx, common.StageCtxKey, common.OSFetch)
	cmds := []commonutils.CommandInterface{
		commands.NewResolveImagePathCommand(ctx, s.svc),
		commands.NewCopyDataCommand(ctx, s.svc),
	}
	for i, c := range cmds {
		if err := c.Execute(log); err != nil {
			log.Printf("State: Execute AndroidOSFetchState failure %s\n", err)
			log.Println("State: Revert AndroidOSFetchState")
			for ; i >= 0; i-- {
				if e := cmds[i].Revert(); e != nil {
					err = errors.Annotate(err, "failure while reverting %s", e).Err()
					break
				}
			}
			return nil, c.GetStatus(), fmt.Errorf("%s: %w", c.GetErrorMessage(), err)
		}
	}
	log.Println("State: AndroidOSFetchState Completed")
	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

func (s OSFetchState) Next() commonutils.ServiceState {
	return OSInstallState(s)
}

func (s OSFetchState) Name() string {
	return "Android OS Fetch State"
}
