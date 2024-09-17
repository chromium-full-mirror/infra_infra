// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ctr contains functions with cros-tool-runner.
package ctr

import (
	"context"
	"fmt"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/ctr"
	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
)

const (
	networkPrefix      = "network-%s"
	adbContainerPrefix = "adb-%s"
)

func startADBContainer(ctx context.Context, info *execs.ExecInfo) error {
	ctrInfo, ok := ctr.Get(ctx)
	if !ok {
		return errors.Reason("start adb container").Err()
	}
	networkName := fmt.Sprintf(networkPrefix, info.GetDut().Name)
	containerName := fmt.Sprintf(adbContainerPrefix, info.GetDut().Name)
	if _, err := ctrInfo.GetNetwork(ctx, networkName); err != nil {
		return errors.Annotate(err, "start adb container").Err()
	}
	req := &api.StartTemplatedContainerRequest{
		Name:           containerName,
		ContainerImage: "us-docker.pkg.dev/cros-registry/test-services/adb-base:prod",
		Template: &api.Template{
			Container: &api.Template_Generic{
				Generic: &api.GenericTemplate{
					BinaryName: "tail",
					BinaryArgs: []string{
						"-f",
						"/dev/null",
					},
					AdditionalVolumes: []string{
						"/creds:/creds",
					},
					DockerArtifactDir: "/tmp/adb",
				},
			},
		},
		Network: networkName,
		// ArtifactDir: c.artifactsDir, defined below in the call.
	}
	if _, err := ctrInfo.CreateContainer(ctx, req); err != nil {
		return errors.Annotate(err, "start adb container").Err()
	}
	log.Infof(ctx, "Container %q started!", req.Name)
	return nil
}

func init() {
	execs.Register("ctr_start_adb_container", startADBContainer)
}
