// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/servertemplate"
)

// FirmwareSpecs contains the flags necessary for the
// filter to know how to build out the provision request
// and populate the lookup table.
type FirmwareSpecs struct {
	Ro   string
	Rw   string
	ECRO string
	ECRW string
	// FirmwareBuilds is a map of board -> latest branch build
	FirmwareBuilds map[string]FirmwareBranchBuild
	// ECMilestoneBuilds is a map of board (nissa) -> milestone (123) -> latest build
	ECMilestoneBuilds map[string]map[int]FirmwareBranchBuild
	LatestMilestone   int
	FallbackToCros    bool
}

// Specification strings for the Ro, Rw, ECRO, and ECRW specs above.
const (
	// LatestFirmwareBranch pulls the firmware from the latest successful build of the board's firmware branch.
	LatestFirmwareBranch string = "firmwareBoardBranch"
	// OSSource pulls the firmware from the firmware that was build from the ChromeOS source, aka tip-of-tree.
	OSSource string = "cros"
	// ECMilestonePrefix followed by a number, pulls the firmware from the nth newest EC milestone branch.
	ECMilestonePrefix string = "M-"
)

// FirmwareBranchBuild holds information about a specific branch build.
type FirmwareBranchBuild struct {
	Builder         string    `bigquery:"builder"`
	FirmwareByBoard string    `bigquery:"firmware_by_board"`
	ArtifactLink    string    `bigquery:"artifact_link"`
	EndTime         time.Time `bigquery:"end_time"`
}

const saProject = "chromeos-bot"

var saFile string

func (specs *FirmwareSpecs) executor(req *api.InternalTestplan, log *log.Logger, commonParams *servertemplate.CommonFilterParams) (ret *api.InternalTestplan, retErr error) {
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			log.Printf("Panic detected: %+v stack: %s", r, stack)
			retErr = fmt.Errorf("panic: %+v stack: %s", r, stack)
		}
	}()

	log.Println("Executing firmware filter")
	if req.GetSuiteInfo().GetSuiteMetadata() == nil {
		return nil, fmt.Errorf("suite_info.suite_metadata is required")
	}

	if specs.FirmwareBuilds == nil {
		ctx := context.Background()

		c, err := bigquery.NewClient(ctx, saProject,
			option.WithCredentialsFile(saFile))
		if err != nil {
			return nil, fmt.Errorf("unable to make bq client %w", err)
		}
		defer c.Close()

		bqQ := c.Query(`SELECT
  *
FROM
  firmware-bigquery.builds.firmware_branch_builds
WHERE
  end_time > TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 180 DAY)
  AND builder NOT LIKE 'firmware-ti50-%'
  AND builder NOT LIKE 'firmware-quiche-%'
  AND builder NOT LIKE 'firmware-servo-%'
  AND builder NOT LIKE 'firmware-cr50-%'
  AND builder NOT LIKE 'firmware-hps-%'
  AND builder NOT LIKE 'firmware-android-%'
  AND NOT REGEXP_CONTAINS(builder, '^firmware-R[0-9]+-[0-9\\.]+\\.B-branch')
  AND NOT REGEXP_CONTAINS(builder, '^firmware-ec-R[0-9]+-[0-9\\.]+\\.B-branch')`)

		iter, err := bqQ.Read(ctx)
		if err != nil {
			return nil, fmt.Errorf("unable to make Bigquery call: %w", err)
		}

		boardMap := make(map[string]FirmwareBranchBuild)
		stableBranchRe := regexp.MustCompile(`-\d+\.\d+\.B-branch`)

		for {
			var r FirmwareBranchBuild
			err := iter.Next(&r)
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("iter.Next failed: %w", err)
			}
			tarPath := strings.SplitN(r.FirmwareByBoard, "/", 2)
			board := tarPath[0]
			// icarus is a terrible special case for 3 jacuzzi models: cozmo, pico, pico6
			if strings.HasPrefix(r.Builder, "firmware-icarus-") {
				board = "icarus"
			}
			if stableBranchRe.MatchString(r.Builder) {
				log.Printf("Skipping stabilization branch %q\n", r.Builder)
				continue
			}
			if existing, ok := boardMap[board]; ok {
				bestPrefix := fmt.Sprintf("firmware-%s-", board)
				// Keep the build that looks like firmware-$BOARD-*
				if strings.HasPrefix(r.Builder, bestPrefix) && !strings.HasPrefix(existing.Builder, bestPrefix) {
					boardMap[board] = r
					continue
				} else if !strings.HasPrefix(r.Builder, bestPrefix) && strings.HasPrefix(existing.Builder, bestPrefix) {
					continue
				}
				return nil, fmt.Errorf("ambiguous firmware branch build for %q: %+v != %+v", board, r.Builder, existing.Builder)
			}
			boardMap[board] = r
		}
		specs.FirmwareBuilds = boardMap
	}

	if specs.ECMilestoneBuilds == nil {
		ctx := context.Background()

		c, err := bigquery.NewClient(ctx, saProject, option.WithCredentialsFile(saFile))
		if err != nil {
			return nil, fmt.Errorf("unable to make bq client %w", err)
		}
		defer c.Close()

		bqQ := c.Query(`SELECT
  *
FROM
  firmware-bigquery.builds.firmware_branch_builds
WHERE
  end_time > TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 180 DAY)
  AND builder LIKE 'firmware-ec-R%'
  AND firmware_by_board LIKE '%/firmware_from_source.tar.bz2'`)

		iter, err := bqQ.Read(ctx)
		if err != nil {
			return nil, fmt.Errorf("unable to make Bigquery call: %w", err)
		}

		boardToMilestone := make(map[string]map[int]FirmwareBranchBuild)
		milestoneBranchRe := regexp.MustCompile(`firmware-ec-R(\d+)-(?:\d+\.)+B-branch`)
		largest := -1

		for {
			var r FirmwareBranchBuild
			err := iter.Next(&r)
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("iter.Next failed: %w", err)
			}
			tarPath := strings.SplitN(r.FirmwareByBoard, "/", 2)
			board := tarPath[0]
			groups := milestoneBranchRe.FindStringSubmatch(r.Builder)
			if groups == nil {
				log.Printf("Skipping unknown branch %q\n", r.Builder)
				continue
			}
			milestone, err := strconv.Atoi(groups[1])
			if err != nil {
				return nil, err
			}
			if milestone > largest {
				largest = milestone
			}
			milestoneToBuild, ok := boardToMilestone[board]
			if !ok {
				milestoneToBuild = make(map[int]FirmwareBranchBuild)
				boardToMilestone[board] = milestoneToBuild
			}
			if existing, ok := milestoneToBuild[milestone]; ok {
				return nil, fmt.Errorf("ambiguous ec milestone branch build for %q: %+v != %+v", board, r.Builder, existing.Builder)
			}
			milestoneToBuild[milestone] = r
		}
		specs.ECMilestoneBuilds = boardToMilestone
		specs.LatestMilestone = largest + 1
	}

	if err := GenerateDynamicInfo(req, specs, log); err != nil {
		log.Printf("Error while generating dynamic info, %s", err)
		return req, err
	}
	log.Println("Finished generating dynamic info")

	return req, nil
}

func main() {
	firmwareSpecs := &FirmwareSpecs{}
	fs := flag.NewFlagSet("Run firmware-provision-filter", flag.ExitOnError)
	fs.StringVar(&firmwareSpecs.Ro, "ro", "", "Comma separated list of specs for firmware RO")
	fs.StringVar(&firmwareSpecs.Rw, "rw", "", "Comma separated list of specs for firmware RW")
	fs.StringVar(&firmwareSpecs.ECRO, "ec-ro", "", "Comma separated list of specs for EC firmware RO")
	fs.StringVar(&firmwareSpecs.ECRW, "ec-rw", "", "Comma separated list of specs for EC firmware RW")
	fs.StringVar(&saFile, "serviceAccountCred", "/creds/service_accounts/service-account-chromeos.json", "Path to service account credential json file")
	fs.BoolVar(&firmwareSpecs.FallbackToCros, "fallbackToCros", false,
		"Fallback to the OS firmware_from_source archive if there is no branch build. (deprecated)")

	err := servertemplate.ServerWithFlagSet(fs, firmwareSpecs.executor, "fw_filter")
	if err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
