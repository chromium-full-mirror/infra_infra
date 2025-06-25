// Copyright 2024 The Chromium Authors
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

const (
	gscInfoFileName = "gsc_info.json"
)

// GscInfo packages GSC devboard info from GSC log files
func GscInfo(logger *log.Logger, testResult *artifact.TestResult) *api.GetGscInfoResponse {
	if testResult == nil {
		return &api.GetGscInfoResponse{}
	}

	ctx := context.Background()
	gscFiles := common.TestLevelFiles(logger, testResult, gscInfoFileName)
	gscInfos := gscInfos(ctx, logger, gscFiles)
	return &api.GetGscInfoResponse{GscInfos: gscInfos}
}

// gscInfos ingests the GSC devboard info from JSON file.
func gscInfos(ctx context.Context, logger *log.Logger, gscFiles map[string]string) map[string]*anypb.Any {
	gscInfos := map[string]*anypb.Any{}
	for test, gscFile := range gscFiles {
		logger.Printf("Fetch: %q for test: %q\n", gscFile, test)

		// Converts the GSC devboard info JSON file into AVL info proto.
		gscInfo := &artifact.GscInfo{}
		err := files.ReadProtoJSONFile(ctx, gscFile, gscInfo)
		if err != nil {
			logger.Printf("Failed to read GSC devboard info from file: %q for test: %q with err: %v\n", gscFile, test, err)
			gscInfos[test], _ = anypb.New(&artifact.GscInfo{})
			continue
		}

		gscInfos[test], _ = anypb.New(gscInfo)
	}

	return gscInfos
}
