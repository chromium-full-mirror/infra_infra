package common

import (
	"fmt"
	"os"
)

// GCE Metadata server environment variables.
const (
	GceMetadataHost = "GCE_METADATA_HOST"
	GceMetadataIP   = "GCE_METADATA_IP"
	GceMetadataRoot = "GCE_METADATA_ROOT"
)

// GceMetadataEnvVars returns environment variables related to the GCE
// Metadata server. These should be set when making GCP requests from
// containers.
func GceMetadataEnvVars() []string {
	// Get GCE Metadata Server env vars
	envVars := []string{}
	if host, present := os.LookupEnv(GceMetadataHost); present {
		envVars = append(envVars, fmt.Sprintf("%s=%s", GceMetadataHost, host))
	}
	if ip, present := os.LookupEnv(GceMetadataIP); present {
		envVars = append(envVars, fmt.Sprintf("%s=%s", GceMetadataIP, ip))
	}
	if root, present := os.LookupEnv(GceMetadataRoot); present {
		envVars = append(envVars, fmt.Sprintf("%s=%s", GceMetadataRoot, root))
	}
	return envVars
}
