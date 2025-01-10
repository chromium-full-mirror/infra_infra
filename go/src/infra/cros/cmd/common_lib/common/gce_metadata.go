package common

import (
	"fmt"
	"os"
)

// GCE Metadata server environment variables.
const (
	GCE_METADATA_HOST = "GCE_METADATA_HOST"
	GCE_METADATA_IP   = "GCE_METADATA_IP"
	GCE_METADATA_ROOT = "GCE_METADATA_ROOT"
)

// gceMetadataEnvVars returns environment variables related to the GCE
// Metadata server. These should be set when making GCP requests from
// containers.
func GceMetadataEnvVars() []string {
	// Get GCE Metadata Server env vars
	envVars := []string{}
	if host, present := os.LookupEnv(GCE_METADATA_HOST); present == true {
		envVars = append(envVars, fmt.Sprintf("%s=%s", GCE_METADATA_HOST, host))
	}
	if ip, present := os.LookupEnv(GCE_METADATA_IP); present == true {
		envVars = append(envVars, fmt.Sprintf("%s=%s", GCE_METADATA_IP, ip))
	}
	if root, present := os.LookupEnv(GCE_METADATA_ROOT); present == true {
		envVars = append(envVars, fmt.Sprintf("%s=%s", GCE_METADATA_ROOT, root))
	}
	return envVars
}
