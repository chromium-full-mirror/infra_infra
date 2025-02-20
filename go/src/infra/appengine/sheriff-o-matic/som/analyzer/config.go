package analyzer

import (
	"context"
	"encoding/json"
	"path/filepath"

	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/appengine/sheriff-o-matic/som/client"
)

const configURL = "https://chromium.googlesource.com/infra/infra/+/HEAD/go/src/infra/appengine/sheriff-o-matic/config/config.json?format=text"

// ConfigRules is a parsed representation of the config.json file, which
// specifies builders and steps to exclude.
type ConfigRules struct {
	IgnoredSteps []string `json:"ignored_steps"`
}

// GetConfigRules fetches the latest version of the config from Gitiles.
func GetConfigRules(c context.Context) (*ConfigRules, error) {
	b, err := client.GetGitilesCached(c, configURL)
	if err != nil {
		return nil, err
	}

	return ParseConfigRules(b)
}

// ParseConfigRules parses the given byte array into a ConfigRules object.
// Public so that parse_config_test can use it.
func ParseConfigRules(cfgJSON []byte) (*ConfigRules, error) {
	cr := &ConfigRules{}
	if err := json.Unmarshal(cfgJSON, cr); err != nil {
		return nil, err
	}

	return cr, nil
}

// ExcludeFailure determines whether a particular failure should be ignored,
// according to the rules in the config.
func (r *ConfigRules) ExcludeFailure(ctx context.Context, builderGroup, builder, step string) bool {
	for _, stepPattern := range r.IgnoredSteps {
		matched, err := filepath.Match(stepPattern, step)
		if err != nil {
			logging.Errorf(ctx, "Malformed step pattern: %s", stepPattern)
		} else if matched {
			return true
		}
	}

	return false
}

func contains(arr []string, s string) bool {
	for _, itm := range arr {
		if itm == s {
			return true
		}
	}

	return false
}
