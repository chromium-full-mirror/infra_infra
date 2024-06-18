// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/maruel/subcommands"

	"infra/libs/vmlab"
	"infra/libs/vmlab/api"
)

var CleanImagesCmd = &subcommands.Command{
	UsageLine: "clean-images",
	ShortDesc: "clean up VM images in the GCP project",
	CommandRun: func() subcommands.CommandRun {
		c := &cleanImagesRun{}
		c.cleanImagesFlags.register(&c.Flags)
		return c
	},
}

type cleanImagesFlags struct {
	rate   int
	dryRun bool
	json   bool
}

func (c *cleanImagesFlags) register(f *flag.FlagSet) {
	f.IntVar(&c.rate, "rate", 1, "Rate limit for delete API calls in requests/second.")
	f.BoolVar(&c.dryRun, "dry-run", false, "Test run without really deleting images. Default is false.")
	f.BoolVar(&c.json, "json", false, "Output json result.")
}

type cleanImagesRun struct {
	subcommands.CommandRunBase
	cleanImagesFlags
}

type cleanImagesResult struct {
	Total   int
	Deleted []string
	Failed  []string
	Unknown []string
}

const (
	imageRetentionCQ         = time.Hour * 24 * 5
	imageRetentionPostsubmit = time.Hour * 24 * 5
	imageRetentionRelease    = time.Hour * 24 * 30
	imageRetentionDefault    = time.Hour * 24 * 3
)

func (c *cleanImagesRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	imageApi, err := vmlab.NewImageApi(api.ProviderId_CLOUDSDK)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot get image api provider: %v\n", err)
	}

	result, err := cleanUpImages(imageApi, c.cleanImagesFlags.rate, c.cleanImagesFlags.dryRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to clean up images: %v\n", err)
		return 1
	}

	if c.cleanImagesFlags.json {
		if jsonResult, err := json.Marshal(result); err != nil {
			fmt.Fprintf(os.Stderr, "cannot convert output to json: %v\n", err)
		} else {
			fmt.Println(string(jsonResult))
		}
	} else {
		fmt.Fprintf(os.Stdout, "total images: %d\n", result.Total)
		fmt.Fprintf(os.Stdout, "deleted images:\n")
		for _, imageName := range result.Deleted {
			fmt.Fprintf(os.Stdout, "\t%s\n", imageName)
		}
		fmt.Fprintf(os.Stdout, "failed to delete images:\n")
		for _, imageName := range result.Failed {
			fmt.Fprintf(os.Stdout, "\t%s\n", imageName)
		}
		fmt.Fprintf(os.Stdout, "unknown images:\n")
		for _, imageName := range result.Unknown {
			fmt.Fprintf(os.Stdout, "\t%s\n", imageName)
		}
	}

	if len(result.Failed) > 0 {
		return 1
	}

	return 0
}

// cleanUpImages cleans up images that exceed retention period. `rate` sets a
// limit on the number of delete image API requests per second. When `dryRun` is
// true it doesn't call image delete API. Returns deleted, failed to delete,
// unknown images in `cleanImagesResult`.
func cleanUpImages(imageApi api.ImageApi, rate int, dryRun bool) (cleanImagesResult, error) {
	result := cleanImagesResult{
		Total:   0,
		Deleted: []string{},
		Failed:  []string{},
		Unknown: []string{},
	}

	// Filter images created by vmlab CLI
	gceImages, err := imageApi.ListImages("labels.created-by:vmlab")
	if err != nil {
		return result, fmt.Errorf("failed to list image: %w", err)
	}
	result.Total = len(gceImages)

	var wg sync.WaitGroup
	var mu sync.Mutex
	limiter := time.NewTicker(time.Second / time.Duration(rate))

	for _, gceImage := range gceImages {
		// For unknown images, we still want to apply a retention period so that
		// they won't waste quota if they aren't investigated soon enough.
		buildType, ok := gceImage.Labels["build-type"]
		if !ok {
			fmt.Fprintf(os.Stderr, "failed to get build-type of image %s\n", gceImage.Name)
		}

		retention, err := getImageRetention(buildType)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to get retention period for %s: %v\n", gceImage.Name, err)
			result.Unknown = append(result.Unknown, gceImage.GetName())
		}

		if time.Now().Before(gceImage.TimeCreated.AsTime().Add(retention)) {
			continue
		}

		<-limiter.C
		if dryRun {
			result.Deleted = append(result.Deleted, gceImage.Name)
			continue
		}

		wg.Add(1)
		go func(imageName string) {
			defer wg.Done()
			if err := imageApi.DeleteImage(imageName, true); err != nil {
				fmt.Fprintf(os.Stderr, "failed to delete image %s: %v\n", imageName, err)
				mu.Lock()
				result.Failed = append(result.Failed, imageName)
				mu.Unlock()
				return
			}
			mu.Lock()
			result.Deleted = append(result.Deleted, imageName)
			mu.Unlock()
		}(gceImage.Name)
	}
	wg.Wait()

	return result, nil
}

// getImageRetention returns retention period for a known image build type. For
// unknown build type it returns an error alongside retention period.
func getImageRetention(buildType string) (time.Duration, error) {
	switch buildType {
	case "cq":
		return imageRetentionCQ, nil
	case "postsubmit":
		return imageRetentionPostsubmit, nil
	case "release":
		return imageRetentionRelease, nil
	case "snapshot":
		return imageRetentionDefault, nil
	default:
		return imageRetentionDefault, fmt.Errorf("unknown build type %s", buildType)
	}
}
