// Copyright 2022 The Chromium Authors
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

type PrepareState struct {
	svc *service.AndroidService
}

func NewPrepareState(s *service.AndroidService) commonutils.ServiceState {
	return PrepareState{
		svc: s,
	}
}

func (s PrepareState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	log.Println("State: Execute AndroidPrepareState")
	ctx = context.WithValue(ctx, common.StageCtxKey, common.Prepare)
	cmds := []commonutils.CommandInterface{
		commands.NewRestartADBCommand(ctx, s.svc),
		commands.NewFetchDutInfoCommand(ctx, s.svc),
	}
	for i, c := range cmds {
		if err := c.Execute(log); err != nil {
			log.Printf("State: Execute AndroidPrepareState failure %s\n", err)
			log.Println("State: Revert AndroidPrepareState")
			for ; i >= 0; i-- {
				if e := cmds[i].Revert(); e != nil {
					err = errors.Annotate(err, "failure while reverting %s", e).Err()
					break
				}
			}
			return nil, c.GetStatus(), fmt.Errorf("%s: %w", c.GetErrorMessage(), err)
		}
	}
	log.Println("State: AndroidPrepareState Completed")
	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

func (s PrepareState) Next() commonutils.ServiceState {
	return OSFetchState(s)
}

func (s PrepareState) Name() string {
	return "Android Prepare State"
}
