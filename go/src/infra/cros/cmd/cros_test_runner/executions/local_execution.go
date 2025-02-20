// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executions

import (
	"context"
	"fmt"
	"log"
	"path"

	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/tools/crostoolrunner"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/internal/configs"
)

func LocalExecution(sk *data.LocalTestStateKeeper, ctrCipdVersion, pathToCipdBin, logPath string, noSudo bool) (*skylab_test_runner.ExecuteResponse, error) {
	common.GlobalTempDir = path.Join(logPath, common.CreateUniquePrefixedName("execution-logs"))
	emptyBuild := &buildbucketpb.Build{}
	build_state, ctx, err := build.Start(context.Background(), emptyBuild)

	logCfg := common.LoggerConfig{Out: log.Default().Writer()}
	ctx = logCfg.Use(ctx)
	defer func() {
		build_state.End(err)
		logCfg.DumpStepsToFolder(common.GlobalTempDir)
	}()

	sk.GcsPublishSrcDir = common.GlobalTempDir
	sk.CpconPublishSrcDir = common.GlobalTempDir
	sk.RdbPublishSrcDir = common.GlobalTempDir
	result, err := executeLocalTests(ctx, sk, ctrCipdVersion, pathToCipdBin, noSudo)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	return &skylab_test_runner.ExecuteResponse{Result: result}, nil
}

// executeLocalTests executes local tests
func executeLocalTests(
	ctx context.Context,
	sk *data.LocalTestStateKeeper,
	ctrCipdVersion, pathToCipdBin string, noSudo bool) (*skylab_test_runner.Result, error) {

	var err error
	step, ctx := build.StartStep(ctx, "Execute Local Tests")
	defer func() {
		if err != nil {
			logging.Infof(ctx, "%s", err)
		}
		step.End(err)
	}()

	// Validation
	if ctrCipdVersion == "" && pathToCipdBin == "" {
		err = fmt.Errorf("Must provide ctrCipdVersion or pathToCipdBin, both cannot be empty for local test execution.")
		return nil, err
	}
	if sk.CftTestRequest == nil {
		err = fmt.Errorf("CftTestRequest is missing from local execution call.")
	}

	var ctrCipdInfo crostoolrunner.CtrCipdInfo
	if pathToCipdBin != "" {
		ctrCipdInfo = crostoolrunner.CtrCipdInfo{
			IsInitialized: true,
			CtrPath:       pathToCipdBin,
		}
	} else {
		ctrCipdInfo = crostoolrunner.CtrCipdInfo{
			Version:        ctrCipdVersion,
			CtrCipdPackage: common.CtrCipdPackage,
		}
	}

	ctr := &crostoolrunner.CrosToolRunner{
		CtrCipdInfo:       ctrCipdInfo,
		EnvVarsToPreserve: common.DockerEnvVarsToPreserve(),
		NoSudo:            noSudo,
	}

	// Create configs
	// TODO: Implement path for PreLocal execution. Ignored for server implementation.
	metadataContainers := sk.CftTestRequest.GetContainerMetadata().GetContainers()
	metadataKey := sk.CftTestRequest.GetPrimaryDut().GetContainerMetadataKey()
	metadataMap, ok := metadataContainers[metadataKey]
	if !ok {
		err = fmt.Errorf("Provided key %q does not exist in provided container metadata.", metadataKey)
		return nil, err
	}
	// TODO: Add cacheserver to container image map. Ignored for server implementation.
	containerImagesMap := metadataMap.GetImages()
	containerCfg := configs.NewContainerConfig(ctr, containerImagesMap, false)
	executorCfg := configs.NewExecutorConfig(ctr, containerCfg)
	cmdCfg := configs.NewCommandConfig(executorCfg)

	if sk.CftTestRequest.AutotestKeyvals == nil {
		sk.CftTestRequest.AutotestKeyvals = map[string]string{}
	}

	if sk.CftTestRequest.GetPrimaryDut() != nil {
		sk.PrimaryDutModel = sk.CftTestRequest.GetPrimaryDut().GetDutModel()
	}
	for _, companion := range sk.CftTestRequest.GetCompanionDuts() {
		sk.CompanionDutModels = append(sk.CompanionDutModels, companion.GetDutModel())
	}

	// Generate config
	localTestConfig := configs.NewTrv2ExecutionConfig(configs.LocalTestExecutionConfigType, cmdCfg, sk, nil)
	err = localTestConfig.GenerateConfig(ctx)
	if err != nil {
		return sk.SkylabResult, errors.Annotate(err, "error during generating local test configs: ").Err()
	}

	// Execute config
	err = localTestConfig.Execute(ctx)
	if err != nil {
		return sk.SkylabResult, errors.Annotate(err, "error during executing local test configs: ").Err()
	}
	return sk.SkylabResult, nil
}
