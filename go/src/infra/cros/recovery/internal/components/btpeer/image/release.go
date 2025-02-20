// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package image

import (
	"context"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/components/linux"
	"go.chromium.org/infra/cros/recovery/internal/execs/wifirouter/ssh"
)

const (
	// btpeerImageConfigProdGCSPublicURL is the public URL of the production image
	// config in GCS.
	//
	// The format of this file is defined as labapi.RaspiosCrosBtpeerImageConfig.
	btpeerImageConfigProdGCSPublicURL = "https://storage.googleapis.com/chromeos-connectivity-test-artifacts/btpeer/raspios_cros_btpeer_image_config_prod.json"

	// imageBuildInfoFilePath is the path to the build info file placed on
	// btpeer images created with chromiumos/third_party/pi-gen-btpeer.
	//
	// The format of this file is defined as labapi.RaspiosCrosBtpeerImageBuildInfo.
	//
	// If this file does not exist on the btpeer, it is assumed that the btpeer
	// does not have a custom image installed.
	imageBuildInfoFilePath = "/etc/chromiumos/raspios_cros_btpeer_build_info.json"
)

// FetchBtpeerImageReleaseConfig downloads the production
// RaspiosCrosBtpeerImageConfig JSON file from GCS via its public URL through
// the runner host and returns its unmarshalled contents.
//
// Note: We use the public URL here rather than the cache to ensure we always
// use the latest version of the config file from GCS.
func FetchBtpeerImageReleaseConfig(ctx context.Context, runner components.Runner) (*labapi.RaspiosCrosBtpeerImageConfig, error) {
	configJSON, _, err := linux.CurlURL(ctx, runner, 30*time.Second, btpeerImageConfigProdGCSPublicURL, nil)
	if err != nil {
		return nil, errors.Annotate(err, "failed to curl %q on the host", btpeerImageConfigProdGCSPublicURL).Err()
	}
	config := &labapi.RaspiosCrosBtpeerImageConfig{}
	if err := protojson.Unmarshal([]byte(configJSON), config); err != nil {
		return nil, errors.Annotate(err, "failed to unmarshal RaspiosCrosBtpeerImageConfig from %q", btpeerImageConfigProdGCSPublicURL).Err()
	}
	return config, nil
}

// MarshalBtpeerImageReleaseConfig marshals the config into JSON using
// the same settings as btpeer_manager, which is what is used to create the
// config JSON that this would be parsed from, so that the look is consistent.
func MarshalBtpeerImageReleaseConfig(config *labapi.RaspiosCrosBtpeerImageConfig) (string, error) {
	marshaller := protojson.MarshalOptions{
		Multiline:       true,
		Indent:          "  ",
		EmitUnpopulated: true,
	}
	configJSON, err := marshaller.Marshal(config)
	if err != nil {
		return "", errors.Annotate(err, "marshal btpeer image release config").Err()
	}
	return string(configJSON), nil
}

// BtpeerHasImageBuildInfoFile checks the btpeer host via the provided runner
// to determine if the build info file exists.
func BtpeerHasImageBuildInfoFile(ctx context.Context, sshRunner ssh.Runner) (bool, error) {
	exists, err := ssh.TestFileExists(ctx, sshRunner, imageBuildInfoFilePath)
	if err != nil {
		return false, errors.Annotate(err, "btpeer has image build info file: failed to check if file %q exists on host", imageBuildInfoFilePath).Err()
	}
	return exists, nil
}

// FetchBtpeerImageBuildInfo reads and parses the image build info file on the
// host using the provided runner and returns the unmarshalled result.
func FetchBtpeerImageBuildInfo(ctx context.Context, sshRunner ssh.Runner) (*labapi.RaspiosCrosBtpeerImageBuildInfo, error) {
	buildInfoJSON, err := ssh.CatFile(ctx, sshRunner, imageBuildInfoFilePath)
	if err != nil {
		return nil, errors.Annotate(err, "fetch btpeer image build info: failed to read file %q on host", imageBuildInfoFilePath).Err()
	}
	buildInfo := &labapi.RaspiosCrosBtpeerImageBuildInfo{}
	if err := protojson.Unmarshal([]byte(buildInfoJSON), buildInfo); err != nil {
		return nil, errors.Annotate(err, "failed to unmarshal RaspiosCrosBtpeerImageBuildInfo from contents of %q on host", imageBuildInfoFilePath).Err()
	}
	return buildInfo, nil
}
