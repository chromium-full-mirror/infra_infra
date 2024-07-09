// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"go.chromium.org/chromiumos/config/go/test/api"
	server "go.chromium.org/chromiumos/test/ctpv2/common/server_template"
)

// FirmwareSpecs contains the flags necessary for the
// filter to know how to build out the provision request
// and populate the lookup table.
type FirmwareSpecs struct {
	Ro             string
	Rw             string
	FirmwareBuilds map[string]FirmwareBranchBuild
}

const (
	LATEST_FIRMWARE_BRANCH string = "firmwareBoardBranch"
)

type FirmwareBranchBuild struct {
	Builder         string    `bigquery:"builder"`
	FirmwareByBoard string    `bigquery:"firmware_by_board"`
	ArtifactLink    string    `bigquery:"artifact_link"`
	EndTime         time.Time `bigquery:"end_time"`
}

const saProject = "chromeos-bot"
const saFile = "/creds/service_accounts/service-account-chromeos.json"

func (specs *FirmwareSpecs) executor(req *api.InternalTestplan, log *log.Logger) (*api.InternalTestplan, error) {
	log.Println("Executing firmware filter")

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
		AND builder NOT LIKE 'firmware-hps-%'`)

		iter, err := bqQ.Read(ctx)
		if err != nil {
			return nil, fmt.Errorf("unable to make Bigquery call: %w", err)
		}

		boardMap := make(map[string]FirmwareBranchBuild)
		stableBranchRe := regexp.MustCompile(`-\d+\.\d+\.B-branch`)

		for {
			var r FirmwareBranchBuild
			err := iter.Next(&r)
			if err == iterator.Done {
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
			} else {
				boardMap[board] = r
			}
		}
		specs.FirmwareBuilds = boardMap
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
	fs.StringVar(&firmwareSpecs.Ro, "ro", "", "Spec for firmare RO")
	fs.StringVar(&firmwareSpecs.Rw, "rw", "", "Spec for firmare RW")

	err := server.ServerWithFlagSet(fs, firmwareSpecs.executor, "fw_filter")
	if err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
