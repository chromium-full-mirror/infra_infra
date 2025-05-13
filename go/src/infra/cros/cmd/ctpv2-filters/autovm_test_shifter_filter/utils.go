// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"

	"github.com/google/uuid"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/cft/publish/commonutils/storage"
)

const (
	buildReportsDir = "/tmp/build_reports_"
)

// dutModelFromDut returns the dut model for a DUT.
func dutModelFromDut(dut *labapi.Dut) *labapi.DutModel {
	if dut == nil {
		return nil
	}

	switch hw := dut.GetDutType().(type) {
	case *labapi.Dut_Chromeos:
		return hw.Chromeos.GetDutModel()
	case *labapi.Dut_Android_:
		return hw.Android.GetDutModel()
	case *labapi.Dut_Devboard_:
		return hw.Devboard.GetDutModel()
	}
	return nil
}

// getBoard returns board value for a scheduling unit.
func getBoard(unit *api.SchedulingUnit) string {
	return dutModelFromDut(unit.GetPrimaryTarget().GetSwarmingDef().GetDutInfo()).GetBuildTarget()
}

func checkBoardInVMBoardsList(board string) bool {
	for _, vmBoard := range vmBoards {
		if board == vmBoard {
			return true
		}
	}
	return false
}

// createDirectory generates a unique directory for each request.
func createDirectory(log *log.Logger) (string, error) {

	dir := buildReportsDir + uuid.New().String()
	log.Printf("Creating dir ...")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	log.Printf("Dir created :%s", dir)
	return dir, nil
}

func getBuildReportJSON(ctx context.Context, board string, version string, buildReportJSONDir string, log *log.Logger) ([]byte, error) {
	gsClient, err := storage.NewGSClient(ctx, "")
	if err != nil {
		return nil, err
	}
	destFilePath := filepath.Join(buildReportJSONDir, board)

	// form the gcsImagePath for VM board to check image availability.
	gcsImagePath := gsBucketPath + board + "-" + release + version + "/" + buildReportJSON
	log.Printf("Checking for board %s: %s", board, gcsImagePath)

	if err := gsClient.DownloadFile(ctx, gcsImagePath, destFilePath); err != nil {
		return nil, err
	}
	content, err := os.ReadFile(destFilePath)
	if err != nil {
		return nil, err
	}

	return content, nil
}

// isVMImageAvailable reads build report in gs bucket and infers if the builder has successfully completed.
func isVMImageAvailable(ctx context.Context, board string, version string, buildReportJSONDir string, log *log.Logger) (bool, error) {
	buildReportJSON, err := getBuildReportJSON(ctx, board, version, buildReportJSONDir, log)
	if err != nil {
		return false, err
	}
	// Unmarshal the JSON string into the map
	var result map[string]any
	err = json.Unmarshal(buildReportJSON, &result)
	if err != nil {
		return false, err
	}

	// safely parse the nested structure
	status, ok := result["status"].(map[string]any)
	if !ok {
		return false, fmt.Errorf("status key not found or is not in expected format")
	}

	value, ok := status["value"].(string)
	if !ok {
		return false, fmt.Errorf("value key not found or is not in expected format")
	}

	// check the status value from build report
	log.Printf("Build report status: %s\n", value)
	if value == success {
		log.Printf("Image found available for AutoVM use")
		return true, nil
	}
	log.Printf("Image not available for AutoVM use")
	return false, nil
}

// getImageVersion returns the image version from a given image gs url(installPath) ex-`R128-15964.4.0`.
func getImageVersionFromInstallPath(installPath string) (string, error) {
	if installPath == "" {
		return "", errors.New("empty Installpath")
	}
	// Capturing group regex. match[0] contains everything including /R, match[1] contains everything captured as part of (.+)
	re := regexp.MustCompile(`release/R(.+)$`)

	match := re.FindStringSubmatch(installPath)
	if len(match) < 1 {
		return "", errors.New("not a release image")
	}
	if len(match) < 2 {
		return "", errors.New("milestone version not found in installPath")
	}

	milestone := match[1]
	return "R" + milestone, nil
}

// getTargetedImageVersion fetches the chromeos image version of boards on which tests are to be executed.
// Note: Is same for all boards for a single CTP request.
func getTargetedImageVersion(req *api.InternalTestplan, log *log.Logger) string {
	// find the image version from the first valid gcs installPath
	for _, tc := range req.TestCases {
		for _, units := range tc.GetSchedulingUnitOptions() {
			for _, unit := range units.GetSchedulingUnits() {
				if unit.GetDynamicUpdateLookupTable() != nil {
					gcsInstallPath, exists := unit.GetDynamicUpdateLookupTable()[installPath]
					if exists {
						log.Printf("Searching the release version in install path :%s", gcsInstallPath)
						imageVersion, err := getImageVersionFromInstallPath(gcsInstallPath)
						if err != nil {
							log.Printf("Release image version not found : %s", err)
							continue
						}
						log.Printf("Found release image version : %s", imageVersion)
						return imageVersion
					}
				}
			}
		}
	}
	return ""
}

// isAnyVMImageAvailable returns true if any corresponding VM image for same version is available. Availability
// means that the builder has successfully completed building image and should be available to lease a VM with it.
func isAnyVMImageAvailable(ctx context.Context, req *api.InternalTestplan, log *log.Logger) (bool, string, string) {
	// fetch image version of intended boards. Same version of VM boards will be used, if available.
	targetedVersion := getTargetedImageVersion(req, log)
	if targetedVersion == "" {
		return false, "", ""
	}
	// unique temp dir for downloading build_report.json. This is to avoid any race condition as multiple requests are processed by CTP on one machine.
	buildReportJSONdir, err := createDirectory(log)
	if err != nil {
		log.Printf("Error setting up parent directory for downloading files from gs: %s", err)
		return false, "", ""

	}

	// check for all valid VM boards for image availability. Stop as soon as any is found available.
	log.Printf("Checking availability for VM board images ...")
	for _, board := range vmBoards {

		exists, err := isVMImageAvailable(ctx, board, targetedVersion, buildReportJSONdir, log)
		if err != nil {
			log.Printf("Error while checking image availability: %s", err)
			continue
		}
		if exists {
			req.GetSuiteInfo().GetSuiteMetadata().SchedulingUnits = append(req.GetSuiteInfo().GetSuiteMetadata().SchedulingUnits, generateVMSchedulingUnit(board, targetedVersion))
			return exists, board, targetedVersion
		}
	}
	log.Printf("No images found available.")
	return false, "", ""
}
