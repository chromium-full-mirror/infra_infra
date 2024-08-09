// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strconv"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"
	yaml "gopkg.in/yaml.v3"

	"infra/cros/recovery/internal/components"
	"infra/cros/recovery/internal/components/linux"
	"infra/cros/recovery/logger"
)

// configData hold data from '/usr/share/chromeos-config/yaml/config.yaml' file.
type configData struct {
	ChromeOS struct {
		Configs []struct {
			Firmware struct {
				BuildTargets struct {
					// AP image name, if missing fallback to ImageName
					Coreboot string `yaml:"coreboot"`
					EC       string `yaml:"ec"`
					ZephyrEC string `yaml:"zephyr-ec"`
				} `yaml:"build-targets"`
				ImageName string `yaml:"image-name"`
			} `yaml:"firmware"`
			Name     string `yaml:"name"`
			Identity struct {
				SKUID int `yaml:"sku-id"`
			} `yaml:"identity"`
		}
	}
}

// ReadConfigYAML read EC/AP fw targets from yaml file on the DUT.
//
// Copy from  http://cs/search?q=ReadConfigYAML%20file:cros-fw-provision&sq=
func ReadConfigYAML(ctx context.Context, model string, run components.Runner, log logger.Logger) (ecTarget string, apTarget string, err error) {
	// Set default sku in case we fail to read.
	sku := -1
	if crosidOut, err := readCrosID(ctx, run, nil); err != nil {
		return ecTarget, apTarget, errors.Annotate(err, "read config yaml").Err()
	} else if parts := crosIDSkuRegexp.FindStringSubmatch(crosidOut); len(parts) < 2 {
		log.Debugf("DUT SKU not found, will use %d", sku)
	} else if sku, err = strconv.Atoi(parts[1]); err != nil {
		return ecTarget, apTarget, errors.Annotate(err, "read config yaml: fail parse sku number").Err()
	} else {
		log.Debugf("DUT SKU = %d", sku)
	}

	const yamlPath = "/usr/share/chromeos-config/yaml/config.yaml"
	if err := linux.IsPathExist(ctx, run, yamlPath); err != nil {
		return ecTarget, apTarget, errors.Annotate(err, "read config yaml: not file found on the host").Err()
	}

	config, err := run(ctx, 30*time.Second, "cat", yamlPath)
	if err != nil {
		return ecTarget, apTarget, errors.Annotate(err, "read config yaml").Err()
	}
	configYaml := configData{}
	parser := yaml.NewDecoder(strings.NewReader(config))
	err = parser.Decode(&configYaml)
	if err != nil {
		return ecTarget, apTarget, errors.Annotate(err, "read config yaml: failed to parse config").Err()
	}
	for _, config := range configYaml.ChromeOS.Configs {
		if strings.EqualFold(config.Name, model) {
			if sku >= 0 && config.Identity.SKUID >= 0 && config.Identity.SKUID != sku {
				continue
			}
			thisAPName := config.Firmware.BuildTargets.Coreboot
			if thisAPName == "" {
				thisAPName = config.Firmware.ImageName
			}
			thisAPName = strings.TrimSpace(thisAPName)
			if thisAPName != "" {
				if apTarget != "" && apTarget != thisAPName {
					return "", "", errors.Reason("ambiguous AP name for model %q sku %d, could be %q or %q", model, sku, apTarget, thisAPName).Err()
				}
				apTarget = thisAPName
			}
			var thisECName string
			// Bizarrely, zephyr builders use the coreboot name for the ec.bin file.
			if config.Firmware.BuildTargets.ZephyrEC != "" {
				thisECName = thisAPName
			} else {
				thisECName = config.Firmware.BuildTargets.EC
			}
			thisECName = strings.TrimSpace(thisECName)
			if thisECName != "" {
				if ecTarget != "" && ecTarget != thisECName {
					return "", "", errors.Reason("ambiguous EC name for model %q sku %d, could be %q or %q", model, sku, ecTarget, thisECName).Err()
				}
				ecTarget = thisECName
			}
		}
	}
	log.Debugf("config.yaml image names AP: %s EC: %s", apTarget, ecTarget)
	return ecTarget, apTarget, nil
}
