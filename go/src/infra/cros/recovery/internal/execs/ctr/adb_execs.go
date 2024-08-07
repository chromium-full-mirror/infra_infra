// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stableversion

import (
	"context"
	"fmt"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/ctr"
	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
)

func startADBContainer(ctx context.Context, info *execs.ExecInfo) error {
	ctrInfo, ok := ctr.Get(ctx)
	if !ok {
		return errors.Reason("start adb container").Err()
	}
	req := &testapi.StartTemplatedContainerRequest{
		Name:           fmt.Sprintf("adb-%s", info.GetDut().Name),
		ContainerImage: "us-docker.pkg.dev/cros-registry/test-services/adb-base:prod",
		Template: &testapi.Template{
			Container: &testapi.Template_Generic{
				Generic: &testapi.GenericTemplate{
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
		Network: "", //common.ContainerDefaultNetwork,
		// ArtifactDir: c.artifactsDir,
	}
	if _, err := ctrInfo.GetContainer(ctx, req); err != nil {
		return errors.Annotate(err, "start adb container").Err()
	}
	log.Infof(ctx, "Container %q started!", req.Name)
	return nil
}

func init() {
	execs.Register("ctr_start_adb_container", startADBContainer)
}
