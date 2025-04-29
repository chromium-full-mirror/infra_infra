// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"

	"google.golang.org/api/option"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/servertemplate"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/partner-staging/site"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/partner-staging/tools"
)

const (
	BinName                   = "partner-staging"
	ChromeOSBuildKey          = "chromeos_build"
	ChromeOSBuildGCSBucketKey = "chromeos_build_gcs_bucket"
	DefaultBucket             = "chromeos-image-archive"
)

func main() {
	if err := servertemplate.Server(executor, BinName); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

// executor is the main function that processes the InternalTestplan request.
// It extracts the builds to stage, creates a Moblab client, and stages the builds.
func executor(req *api.InternalTestplan, log *log.Logger, commonParams *common.CommonFilterParams) (*api.InternalTestplan, error) {
	buildsToStage, cft := extractBuildsToStage(req)
	log.Printf("Found builds to stage: %d\n", len(buildsToStage))
	ctx := context.Background()
	options := make([]option.ClientOption, 0, 1)
	saPath := site.ServiceAccountPath()
	if saPath != "" {
		log.Printf("Using service account key path: %s\n", saPath)
		options = append(options, option.WithCredentialsFile(saPath))
	}
	moblabClient, err := tools.NewMobLabClient(ctx, options...)
	if err != nil {
		return req, err
	}
	err = stageBuilds(ctx, moblabClient, buildsToStage, cft, log)
	return req, err
}

// extractBuildsToStage extracts builds to stage from the InternalTestplan request.
// It iterates through the scheduling units and extracts the build and bucket information from the swarming definition key values.
// It returns a map of build to bucket and cft flag.  Only builds with buckets other than DefaultBucket are included.
func extractBuildsToStage(req *api.InternalTestplan) (map[string]string, bool) {
	buildsToStage := mapBuildsToBucket(req)
	cft := isCFTenabled(req)
	return buildsToStage, cft
}

// mapBuildsToBucket maps builds to their corresponding GCS buckets.
func mapBuildsToBucket(req *api.InternalTestplan) map[string]string {
	buildsToStage := make(map[string]string)
	if req.SuiteInfo == nil || req.SuiteInfo.SuiteMetadata == nil {
		return buildsToStage
	}
	for _, schedulingUnit := range req.SuiteInfo.SuiteMetadata.SchedulingUnits {
		if schedulingUnit.PrimaryTarget == nil || schedulingUnit.PrimaryTarget.SwReq == nil {
			continue
		}
		var (
			build  *string
			bucket *string
		)
		for _, swKeyValues := range schedulingUnit.PrimaryTarget.SwReq.KeyValues {
			if swKeyValues.Key == ChromeOSBuildKey {
				build = &swKeyValues.Value
			}
			if swKeyValues.Key == ChromeOSBuildGCSBucketKey {
				bucket = &swKeyValues.Value
			}
		}
		if build != nil && bucket != nil && *bucket != DefaultBucket {
			buildsToStage[*build] = *bucket
		}
	}
	return buildsToStage
}

// isCFTenabled checks if CFT is enabled in the InternalTestplan request.
func isCFTenabled(_ *api.InternalTestplan) bool {
	// now we can't verify if cft is enabled in the request
	// ToDo: need to wait until client library will be updated: cl/725213532
	return true
}

// stageBuilds stages the specified Chrome OS builds to the user GCS buckets.
// It takes a map of build to bucket and stages each build to the specified bucket.
// It uses a wait group to wait for all builds to be staged before returning.
func stageBuilds(ctx context.Context, moblabClient tools.MobLabAPI, buildsToStage map[string]string, cft bool, log *log.Logger) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(buildsToStage))

	for build, bucket := range buildsToStage {
		wg.Add(1)
		go func(build, bucket string) {
			defer wg.Done()
			if err := stageChromeOSBuild(ctx, moblabClient, build, bucket, cft, log); err != nil {
				errChan <- err
			}
		}(build, bucket)
	}

	wg.Wait()
	close(errChan)

	var errorList []string
	for err := range errChan {
		errorList = append(errorList, err.Error())
	}

	if errLen := len(errorList); errLen > 0 {
		return fmt.Errorf("Failed staging %d out of %d builds; Errors:\n%s", len(errorList), len(buildsToStage), strings.Join(errorList, "\n"))
	}
	return nil
}

// stageChromeOSBuild stages the specified Chrome OS build to the user GCS bucket.
// It parses the build string to extract the board and version, and then calls StageImageToBucket to stage the build.
func stageChromeOSBuild(ctx context.Context, moblabClient tools.MobLabAPI, build string, bucket string, cft bool, log *log.Logger) error {
	board, version, err := parseChromeOSBuild(build)
	if err != nil {
		return err
	}
	stageParams := &tools.StageImageParams{
		Bucket:       bucket,
		Board:        board,
		Model:        "foo", // CTPV1 invoke stage image with `foo` placeholder: go/stage-foo
		BuildVersion: version,
		BuildType:    tools.BuildTypeReleas,
	}
	err = tools.StageImageToBucket(ctx, moblabClient, stageParams, cft, log)
	return err
}

// parseChromeOSBuild parses the Chrome OS build string to extract the board and version.
// The build string is expected to be in the format "{board}-release/R{milestone}-{release}".
// It returns the board and release, and an error if the build string is not formatted correctly.
func parseChromeOSBuild(build string) (string, string, error) {
	re := regexp.MustCompile(`(.*)-release/R?(?:\d+-)?(\d+.*)`)
	matches := re.FindStringSubmatch(build)

	if len(matches) < 3 {
		return "", "", fmt.Errorf("Build not formatted correctly: %s", build)
	}

	return matches[1], matches[2], nil
}
