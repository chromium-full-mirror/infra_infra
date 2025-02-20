// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"go.chromium.org/infra/libs/vmlab/api"
)

var ConfigMapping = map[string]*BuiltinConfig{
	"cts-prototype": {
		ProviderId: api.ProviderId_GCLOUD,
		GcloudConfig: api.Config_GCloudBackend{
			Project:        "betty-cloud-prototype",
			Zone:           "us-west2-a",
			MachineType:    "n2-standard-4",
			InstancePrefix: "ctsprototype-",
			PublicIp:       true,
		},
	},
}
