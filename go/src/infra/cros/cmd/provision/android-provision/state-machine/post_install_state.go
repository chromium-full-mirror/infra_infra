// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package state_machine

import (
	"context"
	"log"

	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/provision/android-provision/common"
	"go.chromium.org/infra/cros/cmd/provision/android-provision/service"
	"go.chromium.org/infra/cros/cmd/provision/android-provision/state-machine/commands"
	commonutils "go.chromium.org/infra/cros/cmd/provision/common-utils"
)

type PostInstallState struct {
	svc *service.AndroidService
}

func (s PostInstallState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	log.Println("State: Execute AndroidPostInstallState")
	ctx = context.WithValue(ctx, common.StageCtxKey, common.PostInstall)
	cmds := []commonutils.CommandInterface{
		commands.NewCleanupCommand(ctx, s.svc),
	}
	for _, c := range cmds {
		// Ignore errors. Don't fail provisioning due to cleanup errors.
		c.Execute(log)
	}
	log.Println("State: AndroidPostInstallState Completed")
	// Return metadata with provisioned OS and packages.
	resp, _ := s.svc.MarshalResponseMetadata()
	return resp, api.InstallResponse_STATUS_SUCCESS, nil
}

func (s PostInstallState) Next() commonutils.ServiceState {
	return nil
}

func (s PostInstallState) Name() string {
	return "Android Post Install State"
}
