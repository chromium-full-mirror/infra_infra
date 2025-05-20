// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/servertemplate"
)

func GenerateFilterExecutor() servertemplate.Filter {
	return &FirmwareSpecs{}
}

// FirmwareSpecs contains the flags necessary for the
// filter to know how to build out the provision request
// and populate the lookup table.
type FirmwareSpecs struct {
	servertemplate.FilterBase

	Ro   string
	Rw   string
	ECRO string
	ECRW string
	// FirmwareBuilds is a map of board -> latest branch build
	FirmwareBuilds map[string]FirmwareBranchBuild
	// ECMilestoneBuilds is a map of board (nissa) -> milestone (123) -> latest build
	ECMilestoneBuilds map[string]map[int]FirmwareBranchBuild
	LatestMilestone   int
	// SAFile is the path to the cloud credentials file.
	SAFile       string
	VersionCache map[string]string
	// TestArgReplacements are user requested test args that need resolved to paths.
	TestArgReplacements map[string]string
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

func (specs *FirmwareSpecs) Init(args []string) error {
	fs := flag.NewFlagSet("Run firmware-provision-filter", flag.ExitOnError)
	fs.StringVar(&specs.Ro, "ro", "", "Comma separated list of specs for firmware RO")
	fs.StringVar(&specs.Rw, "rw", "", "Comma separated list of specs for firmware RW")
	fs.StringVar(&specs.ECRO, "ec-ro", "", "Comma separated list of specs for EC firmware RO")
	fs.StringVar(&specs.ECRW, "ec-rw", "", "Comma separated list of specs for EC firmware RW")
	fs.StringVar(&specs.SAFile, "serviceAccountCred", "/creds/service_accounts/service-account-chromeos.json", "Path to service account credential json file")
	fs.Func("testarg", "key=SPEC to add to test arguments", func(s string) error {
		parts := strings.SplitN(s, "=", 2)
		if len(parts) != 2 {
			return errors.New("Invalid testarg, use -testarg key=SPEC")
		}
		key := parts[0]
		val := parts[1]
		if specs.TestArgReplacements == nil {
			specs.TestArgReplacements = make(map[string]string)
		}
		specs.TestArgReplacements[key] = val
		return nil
	})

	return fs.Parse(args)
}

func (specs *FirmwareSpecs) Executor(req *api.InternalTestplan, log *log.Logger, commonParams *common.CommonFilterParams) (ret *api.InternalTestplan, retErr error) {
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
			option.WithCredentialsFile(specs.SAFile))
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
			// trulo is also a terrible special case, maybe even worse that icarus, since it is on a stabilization branch
			if strings.HasPrefix(r.Builder, "firmware-trulo-") {
				board = "trulo"
			} else if stableBranchRe.MatchString(r.Builder) {
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

		c, err := bigquery.NewClient(ctx, saProject, option.WithCredentialsFile(specs.SAFile))
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

	firmwareImageArchiveBucket := "firmware-image-archive"
	ctx := context.Background()
	// gcsMatcher is a function that searches google cloud storage for a specific version artifact. It is injected for testing purposes.
	// bucketName is the storage bucket, i.e. chromeos-image-archive
	// branchPrefix is the branch up through the major version and dot, i.e. firmware-brya-14505.
	// release is the milestone of the firmware branch with trailing hyphen like R100-
	// version is the desired version, i.e. 14505.102.0
	// suffix is the path to the file, i.e. brya/firmware_from_source.tar.bz2
	searchGCS := func(ctx context.Context, bucketName, branchPrefix, release, version, suffix, board string) (string, error) {
		client, err := storage.NewClient(ctx, option.WithCredentialsFile(specs.SAFile))
		if err != nil {
			return "", fmt.Errorf("storage.NewClient: %w", err)
		}
		defer client.Close()

		verRe, err := regexp.Compile(`/(R\d+-)?([\d\.]+)(?:-[^/]*)?/$`)
		if err != nil {
			return "", fmt.Errorf("regexp.Compile: %w", err)
		}

		// Firmware images can live in a number of different places:

		// 1) The branch artifacts in firmware-image-archive
		// gs://firmware-image-archive/firmware-zork-13434.B/13434.891.0/

		// Find all the branch dirs in the bucket:
		bucket := client.Bucket(firmwareImageArchiveBucket)
		log.Printf("Listing gs://%s/%s", firmwareImageArchiveBucket, branchPrefix)
		dirIter := bucket.Objects(ctx, &storage.Query{Prefix: branchPrefix, Delimiter: "/"})
		for {
			attrs, err := dirIter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return "", fmt.Errorf("%s.Objects(%s): %w", firmwareImageArchiveBucket, branchPrefix, err)
			}
			log.Printf("Found directory gs://%s/%s", firmwareImageArchiveBucket, attrs.Prefix)
			// Find all the branch dirs in the bucket:
			verPrefix := attrs.Prefix + version
			log.Printf("Listing gs://%s/%s", firmwareImageArchiveBucket, verPrefix)
			verIter := bucket.Objects(ctx, &storage.Query{Prefix: verPrefix, Delimiter: "/"})
			for {
				verAttrs, err := verIter.Next()
				if err == iterator.Done {
					break
				}
				if err != nil {
					return "", fmt.Errorf("%s.Objects(%s): %w", firmwareImageArchiveBucket, verPrefix, err)
				}
				log.Printf("Found version dir gs://%s/%s", firmwareImageArchiveBucket, verAttrs.Prefix)
				matches := verRe.FindStringSubmatch(verAttrs.Prefix)
				if matches != nil && matches[2] == version {
					log.Printf("Found matching version gs://%s/%s", firmwareImageArchiveBucket, verAttrs.Prefix)
					return fmt.Sprintf("gs://%s/%s", firmwareImageArchiveBucket, verAttrs.Prefix), nil
				}
			}
		}

		// 2) The EC milestone artifacts in firmware-image-archive
		// gs://firmware-image-archive/firmware-ec-R135-16209.5.B/16209.5.25/

		bucket = client.Bucket(firmwareImageArchiveBucket)
		ecPrefix := "firmware-ec-R"
		log.Printf("Listing gs://%s/%s", firmwareImageArchiveBucket, ecPrefix)
		dirIter = bucket.Objects(ctx, &storage.Query{Prefix: ecPrefix, Delimiter: "/"})
		for {
			attrs, err := dirIter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return "", fmt.Errorf("%s.Objects(%s): %w", firmwareImageArchiveBucket, ecPrefix, err)
			}
			log.Printf("Found directory gs://%s/%s", firmwareImageArchiveBucket, attrs.Prefix)
			// Find all the branch dirs in the bucket:
			verPrefix := attrs.Prefix + version
			log.Printf("Listing gs://%s/%s", firmwareImageArchiveBucket, verPrefix)
			verIter := bucket.Objects(ctx, &storage.Query{Prefix: verPrefix, Delimiter: "/"})
			for {
				verAttrs, err := verIter.Next()
				if err == iterator.Done {
					break
				}
				if err != nil {
					return "", fmt.Errorf("%s.Objects(%s): %w", firmwareImageArchiveBucket, verPrefix, err)
				}
				log.Printf("Found version dir gs://%s/%s", firmwareImageArchiveBucket, verAttrs.Prefix)
				matches := verRe.FindStringSubmatch(verAttrs.Prefix)
				if matches != nil && matches[2] == version {
					log.Printf("Found matching version gs://%s/%s", firmwareImageArchiveBucket, verAttrs.Prefix)
					return fmt.Sprintf("gs://%s/%s", firmwareImageArchiveBucket, verAttrs.Prefix), nil
				}
			}
		}

		// 3) The branch artifacts in chromeos-image-archive
		// gs://chromeos-image-archive/firmware-zork-13434.B-branch/R87-13434.907.0-1-8719078507287215105/zork/firmware_from_source.tar.bz2
		// gs://chromeos-image-archive/firmware-zork-13434.B-branch-firmware/R87-13434.636.0/firmware_from_source.tar.bz2

		bucket = client.Bucket(bucketName)
		log.Printf("Listing gs://%s/%s", bucketName, branchPrefix)
		dirIter = bucket.Objects(ctx, &storage.Query{Prefix: branchPrefix, Delimiter: "/"})
		for {
			attrs, err := dirIter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return "", fmt.Errorf("%s.Objects(%s): %w", bucketName, branchPrefix, err)
			}
			log.Printf("Found directory gs://%s/%s", bucketName, attrs.Prefix)
			verPrefix := attrs.Prefix + release + version
			log.Printf("Listing gs://%s/%s", bucketName, verPrefix)
			verIter := bucket.Objects(ctx, &storage.Query{Prefix: verPrefix, Delimiter: "/"})
			for {
				verAttrs, err := verIter.Next()
				if err == iterator.Done {
					break
				}
				if err != nil {
					return "", fmt.Errorf("%s.Objects(%s): %w", bucketName, verPrefix, err)
				}
				log.Printf("Found version dir %s", verAttrs.Prefix)
				matches := verRe.FindStringSubmatch(verAttrs.Prefix)
				if matches != nil && matches[2] == version {
					log.Printf("Found matching version %s", verAttrs.Prefix)
					fileAttrs, err := bucket.Object(verAttrs.Prefix + suffix).Attrs(ctx)
					if err == nil {
						return fmt.Sprintf("gs://%s/%s", fileAttrs.Bucket, fileAttrs.Name), nil
					}
					if !errors.Is(err, storage.ErrObjectNotExist) {
						return "", fmt.Errorf("%s.Object(%s): %w", bucketName, verAttrs.Prefix+suffix, err)
					}
					fileAttrs, err = bucket.Object(verAttrs.Prefix + "firmware_from_source.tar.bz2").Attrs(ctx)
					if err == nil {
						return fmt.Sprintf("gs://%s/%s", fileAttrs.Bucket, fileAttrs.Name), nil
					}
					if !errors.Is(err, storage.ErrObjectNotExist) {
						return "", fmt.Errorf("%s.Object(%s): %w", bucketName, verAttrs.Prefix+"firmware_from_source.tar.bz2", err)
					}
				}
			}
		}

		// 4) Old firmware builds
		// gs://chromeos-image-archive/zork-firmware/R87-13434.283.0/firmware_from_source.tar.bz2
		bucket = client.Bucket(bucketName)
		log.Printf("Listing gs://%s/%s", bucketName, fmt.Sprintf("%s-firmware", board))
		dirIter = bucket.Objects(ctx, &storage.Query{Prefix: fmt.Sprintf("%s-firmware", board), Delimiter: "/"})
		for {
			attrs, err := dirIter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return "", fmt.Errorf("%s.Objects(%s): %w", bucketName, branchPrefix, err)
			}
			log.Printf("Found directory gs://%s/%s", bucketName, attrs.Prefix)
			verPrefix := attrs.Prefix + release + version
			log.Printf("Listing gs://%s/%s", bucketName, verPrefix)
			verIter := bucket.Objects(ctx, &storage.Query{Prefix: verPrefix, Delimiter: "/"})
			for {
				verAttrs, err := verIter.Next()
				if err == iterator.Done {
					break
				}
				if err != nil {
					return "", fmt.Errorf("%s.Objects(%s): %w", bucketName, verPrefix, err)
				}
				log.Printf("Found version dir gs://%s/%s", bucketName, verAttrs.Prefix)
				matches := verRe.FindStringSubmatch(verAttrs.Prefix)
				if matches != nil && matches[2] == version {
					log.Printf("Found matching version %s", verAttrs.Prefix)
					fileAttrs, err := bucket.Object(verAttrs.Prefix + suffix).Attrs(ctx)
					if err == nil {
						return fmt.Sprintf("gs://%s/%s", fileAttrs.Bucket, fileAttrs.Name), nil
					}
					if !errors.Is(err, storage.ErrObjectNotExist) {
						return "", fmt.Errorf("%s.Object(%s): %w", bucketName, verAttrs.Prefix+suffix, err)
					}
					fileAttrs, err = bucket.Object(verAttrs.Prefix + "firmware_from_source.tar.bz2").Attrs(ctx)
					if err == nil {
						return fmt.Sprintf("gs://%s/%s", fileAttrs.Bucket, fileAttrs.Name), nil
					}
					if !errors.Is(err, storage.ErrObjectNotExist) {
						return "", fmt.Errorf("%s.Object(%s): %w", bucketName, verAttrs.Prefix+"firmware_from_source.tar.bz2", err)
					}
				}
			}
		}

		// 5) ToT or release branch builds - board-release location
		// gs://chromeos-image-archive/zork-release/R104-14909.31.0/firmware_from_source.tar.bz2
		bucket = client.Bucket(bucketName)
		latestPath := fmt.Sprintf("%s-release/LATEST-%s", board, version)
		log.Printf("Reading gs://%s/%s", bucketName, latestPath)
		latest, err := bucket.Object(latestPath).NewReader(ctx)
		if err == nil {
			b, err := io.ReadAll(latest)
			if err != nil {
				return "", fmt.Errorf("io.ReadAll(gs://%s/%s): %w", bucketName, latestPath, err)
			}
			latestPath = fmt.Sprintf("%s-release/%s/", board, string(b))
			log.Printf("Found matching version gs://%s/%s", bucketName, latestPath)
			fileAttrs, err := bucket.Object(latestPath + "firmware_from_source.tar.bz2").Attrs(ctx)
			if err == nil {
				return fmt.Sprintf("gs://%s/%s", fileAttrs.Bucket, fileAttrs.Name), nil
			}
			if !errors.Is(err, storage.ErrObjectNotExist) {
				return "", fmt.Errorf("%s.Object(%s): %w", bucketName, latestPath+"firmware_from_source.tar.bz2", err)
			}
		} else if !errors.Is(err, storage.ErrObjectNotExist) {
			return "", fmt.Errorf("%s.Object(%s): %w", bucketName, latestPath, err)
		}

		// 6) ToT or release branch builds - canary-channel location
		// gs://chromeos-releases/canary-channel/zork/13433.0.0/ChromeOS-firmware-R87-13433.0.0-zork.tar.bz2
		chromeOSReleasesBucket := "chromeos-releases"
		bucket = client.Bucket(chromeOSReleasesBucket)
		firmwarePrefix := fmt.Sprintf("canary-channel/%s/%s/ChromeOS-firmware-", board, version)
		log.Printf("Listing gs://%s/%s", chromeOSReleasesBucket, firmwarePrefix)
		fileIter := bucket.Objects(ctx, &storage.Query{Prefix: firmwarePrefix, Delimiter: "/"})
		fileAttrs, err := fileIter.Next()
		if err == iterator.Done {
			return "", fmt.Errorf("artifacts for version %s not found for %s", version, board)
		}
		if err != nil {
			return "", fmt.Errorf("%s.Objects(%s): %w", chromeOSReleasesBucket, firmwarePrefix, err)
		}
		return fmt.Sprintf("gs://%s/%s", fileAttrs.Bucket, fileAttrs.Name), nil
	}

	if err := GenerateDynamicInfo(ctx, req, specs, log, searchGCS); err != nil {
		log.Printf("Error while generating dynamic info, %s", err)
		return req, err
	}
	log.Println("Finished generating dynamic info")

	return req, nil
}

func main() {
	err := servertemplate.Server(GenerateFilterExecutor, "fw_filter")
	if err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
