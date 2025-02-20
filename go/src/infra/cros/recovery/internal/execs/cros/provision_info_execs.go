// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/cros"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// updateProvisionedInfoExec updated provision info.
// Data which included:
// 1) OS version from the DUT.
// 2) Job repo URL to download the packages matched to OS version.
func updateProvisionedInfoExec(ctx context.Context, info *execs.ExecInfo) error {
	if info.GetDut().ProvisionedInfo == nil {
		info.GetDut().ProvisionedInfo = &tlw.ProvisionedInfo{}
	}
	run := info.NewRunner(info.GetDut().Name)
	osVersion, err := cros.ReleaseBuildPath(ctx, run, info.NewLogger())
	if err != nil {
		return errors.Annotate(err, "update provision info").Err()
	}
	log.Debugf(ctx, "ChromeOS version on the dut: %s.", osVersion)
	// TODO: Remove this once confirmed new VersionInfo works from end-to-end.
	info.GetDut().ProvisionedInfo.CrosVersion = osVersion

	// Apply new UFS dut_state VersionInfo.
	if info.GetChromeos().GetIsAndroidBased() {
		info.GetDut().GetVersionInfo().OsType = tlw.VersionInfo_ANDROID
	} else if info.GetChromeos() != nil {
		info.GetDut().GetVersionInfo().OsType = tlw.VersionInfo_CHROMEOS
	} else {
		info.GetDut().GetVersionInfo().OsType = tlw.VersionInfo_UNKNOWN
	}
	info.GetDut().GetVersionInfo().Os = osVersion
	info.GetDut().GetVersionInfo().RwFirmware = ""
	info.GetDut().GetVersionInfo().RoFirmware = ""

	argsMap := info.GetActionArgs(ctx)
	if argsMap.AsBool(ctx, "update_job_repo_url", false) {
		log.Debugf(ctx, "Updating job repo URL ...")
		gsPath := fmt.Sprintf("%s/%s", gsCrOSImageBucket, osVersion)
		jobRepoURL, err := info.GetAccess().GetCacheUrl(ctx, info.GetDut().Name, gsPath)
		if err != nil {
			return errors.Annotate(err, "update provision info").Err()
		}
		log.Debugf(ctx, "New job repo URL: %s.", jobRepoURL)
		info.GetDut().ProvisionedInfo.JobRepoUrl = jobRepoURL
	}
	return nil
}

func resetProvisionedInfoExec(ctx context.Context, info *execs.ExecInfo) error {
	log.Debugf(ctx, "Provision info reseted!")
	// TODO: Remove old logic that uses ProvisionedInfo once confirmed new VersionInfo
	// workflow WAI.
	info.GetDut().ProvisionedInfo = &tlw.ProvisionedInfo{
		CrosVersion: "",
		JobRepoUrl:  "",
	}
	if info.GetChromeos().GetIsAndroidBased() {
		info.GetDut().GetVersionInfo().OsType = tlw.VersionInfo_ANDROID
	} else if info.GetChromeos() != nil {
		info.GetDut().GetVersionInfo().OsType = tlw.VersionInfo_CHROMEOS
	} else {
		info.GetDut().GetVersionInfo().OsType = tlw.VersionInfo_UNKNOWN
	}
	info.GetDut().GetVersionInfo().Os = ""
	info.GetDut().GetVersionInfo().RwFirmware = ""
	info.GetDut().GetVersionInfo().RoFirmware = ""
	return nil
}

// gsCrOSImageBucket is the base URL for the Google Storage bucket for
// ChromeOS image archives.
var gsCrOSImageBucket string

func init() {
	gcsBucket := os.Getenv("DRONE_AGENT_GCS_IMAGE_STORAGE_SERVER")
	if gcsBucket == "" {
		gcsBucket = "gs://chromeos-image-archive"
	}
	if strings.HasPrefix(gcsBucket, "gs://") {
		gsCrOSImageBucket = gcsBucket
	} else {
		gsCrOSImageBucket = fmt.Sprintf("gs://%s", gcsBucket)
	}
	execs.Register("cros_update_provision_info", updateProvisionedInfoExec)
	execs.Register("cros_reset_provision_info", resetProvisionedInfoExec)
}
