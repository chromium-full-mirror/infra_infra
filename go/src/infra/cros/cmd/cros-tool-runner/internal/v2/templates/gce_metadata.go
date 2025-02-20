// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package templates

import (
	"fmt"
	"os"
)

// HostServiceAcctCredsDir is the path on the host where service account
// credentials are stored. This must be mounted into containers which
// are making requests to GCP.
const HostServiceAcctCredsDir = "/creds/service_accounts"

// GCE Metadata server environment variables.
const (
	gceMetadataHost = "GCE_METADATA_HOST"
	gceMetadataIP   = "GCE_METADATA_IP"
	gceMetadataRoot = "GCE_METADATA_ROOT"
)

// gceMetadataEnvVars returns environment variables related to the GCE
// Metadata server. These should be set when making GCP requests from
// containers.
func gceMetadataEnvVars() []string {
	// Get GCE Metadata Server env vars
	envVars := []string{}
	if host, present := os.LookupEnv(gceMetadataHost); present {
		envVars = append(envVars, fmt.Sprintf("%s=%s", gceMetadataHost, host))
	}
	if ip, present := os.LookupEnv(gceMetadataIP); present {
		envVars = append(envVars, fmt.Sprintf("%s=%s", gceMetadataIP, ip))
	}
	if root, present := os.LookupEnv(gceMetadataRoot); present {
		envVars = append(envVars, fmt.Sprintf("%s=%s", gceMetadataRoot, root))
	}
	return envVars
}
