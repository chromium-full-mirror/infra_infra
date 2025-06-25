// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package commands

import (
	"context"
	"log"

	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/artifact"

	"go.chromium.org/infra/cros/cmd/cft/common/files"
	"go.chromium.org/infra/cros/cmd/cft/post-test/post-process/common"
)

const StressTestInfoFileName = "stress_info.json"

// GetStressTestInfo retrieves stress test info from stress info JSON file.
func GetStressTestInfo(logger *log.Logger, testResult *artifact.TestResult) *api.GetStressTestInfoResponse {
	if testResult == nil {
		return &api.GetStressTestInfoResponse{}
	}

	stressTestFiles := common.TestLevelFiles(logger, testResult, StressTestInfoFileName)
	ctx := context.Background()
	stressTestInfos := stressTestInfos(ctx, logger, stressTestFiles)
	return &api.GetStressTestInfoResponse{StressTestInfo: stressTestInfos}

}

// stressTestInfos ingests the stress info from the first successfully read
// JSON file. It is expected that all the files are identical. stressTestFiles
// is a map of test name to the path of the stress info JSON file.
func stressTestInfos(ctx context.Context, logger *log.Logger, stressTestFiles map[string]string) map[string]*anypb.Any {
	stressTestInfos := map[string]*anypb.Any{}
	for test, stressTestFile := range stressTestFiles {
		logger.Printf("Fetch: %q for test: %q\n", stressTestFile, test)

		// Converts the stress test info JSON file into stress test info proto.
		stressTestInfo := &artifact.StressTestInfo{}
		err := files.ReadProtoJSONFile(ctx, stressTestFile, stressTestInfo)
		if err != nil {
			logger.Printf("Failed to read stress test info from file: %q for test: %q with err: %v\n", stressTestFile, test, err)
			stressTestInfos[test], _ = anypb.New(&artifact.StressTestInfo{})
			continue
		}

		stressTestInfos[test], _ = anypb.New(stressTestInfo)
	}

	return stressTestInfos
}
