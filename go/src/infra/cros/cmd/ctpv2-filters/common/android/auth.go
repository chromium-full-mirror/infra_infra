// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package android

import (
	"golang.org/x/oauth2/google"

	androidapi "go.chromium.org/infra/cros/cmd/common_lib/android_api"
	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

type CloudRunFilterAuthenticator struct {
	androidapi.RunType

	AuthHandler common.FilterAuthInterface
}

func (run *CloudRunFilterAuthenticator) FetchCredentials() (*google.Credentials, error) {
	return &google.Credentials{
		TokenSource: run.AuthHandler.GetTokenSource(common.CTPv2DockerKeyFileLocations, androidapi.CloudPlatformScope, androidapi.AndroidBuildInternalScope),
	}, nil
}

func (run *CloudRunFilterAuthenticator) String() string {
	return "cloudRunFilterAuthenticator"
}
