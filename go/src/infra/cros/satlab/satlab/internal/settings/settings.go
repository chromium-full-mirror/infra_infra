// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package settings

import (
	"encoding/json"
	"fmt"
	"os"

	"go.chromium.org/infra/cros/satlab/common/paths"
)

type Settings map[string]any

// readSettingsFromFile reads settings from a JSON file.
func readSettingsFromFile() (Settings, error) {
	data, err := os.ReadFile(paths.UserSettingsPath)
	if err != nil {
		return nil, err
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}

	return settings, nil
}

// writeSettingsToFile writes settings to a JSON file.
func writeSettingsToFile(settings Settings) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(paths.UserSettingsPath, data, 0644)
}

func (c Settings) PrintValues() error {
	for key, value := range c {
		fmt.Printf("%s: %v\n", key, value)
	}
	return nil
}

func (c Settings) PrintValue(key string) error {
	value, ok := c[key]
	if !ok {
		return fmt.Errorf("key '%s' does not exist in settings", key)
	}
	fmt.Printf("%s: %v\n", key, value)
	return nil
}
