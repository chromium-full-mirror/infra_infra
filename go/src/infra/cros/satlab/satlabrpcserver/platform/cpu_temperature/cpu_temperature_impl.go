// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package cpu_temperature

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.chromium.org/infra/cros/satlab/satlabrpcserver/utils"
)

// ThermalZoneDir /* It is a platform related constants */
const ThermalZoneDir = "/sys/class/thermal"

// ThermalZonePrefix /* It is a platform related constants */
const ThermalZonePrefix = "thermal_zone"

// ThermalZoneType /* It is a platform related constants */
const ThermalZoneType = "type"

// ThermalZoneTemp /* It is a platform related constants */
const ThermalZoneTemp = "temp"

// ThermalZone /* It is a platform related constants */
const ThermalZone = "x86_pkg_temp"

type FilePathCPUTemperature struct {
	thermalZoneFilePath string
}

// NewCPUTemperature to create an instance that implement the ICPUTemperature interface.
func NewCPUTemperature() (ICPUTemperature, error) {
	path, err := thermalZoneFilePath(ThermalZoneDir, ThermalZonePrefix, ThermalZoneType, ThermalZone)
	if err != nil {
		return nil, err
	}

	return &FilePathCPUTemperature{
		thermalZoneFilePath: filepath.Join(path, ThermalZoneTemp),
	}, nil
}

// thermalZoneFilePath determine the correct thermal_zone file to read.
func thermalZoneFilePath(thermalZoneDir string, thermalZonePrefix string, thermalZoneType string, thermalZone string) (string, error) {
	subDirs, err := os.ReadDir(thermalZoneDir)
	if err != nil {
		return "", err
	}
	path, err := filepath.Abs(thermalZoneDir)
	if err != nil {
		return "", err
	}

	for _, entry := range subDirs {
		if strings.HasPrefix(entry.Name(), thermalZonePrefix) {
			tzf := filepath.Join(path, entry.Name(), thermalZoneType)
			file, err := os.Open(tzf)
			if err != nil {
				return "", err
			}

			r := bufio.NewReader(file)
			bytes, _, err := r.ReadLine()
			if err != nil {
				return "", err
			}

			content := string(bytes)
			find := content == thermalZone
			// If we can't close opened file.
			// We can't do anything here. We log the error message.
			err = file.Close()
			if err != nil {
				log.Printf("Can't close the file, got an error: %v", err)
			}

			if find {
				return filepath.Join(path, entry.Name()), nil
			}
		}
	}

	return "", utils.NotFound
}

// GetCurrentCPUTemperature Get the current temperature of CPU
func (c *FilePathCPUTemperature) GetCurrentCPUTemperature() (float32, error) {
	file, err := os.Open(c.thermalZoneFilePath)
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			// If we can't close opened file.
			// We can't do anything here. We log the error message.
			log.Printf("Can't close the file, got an error: %v", err)
		}
	}(file)
	if err != nil {
		return 0, err
	}

	reader := bufio.NewReader(file)
	bytes, _, err := reader.ReadLine()
	if err != nil {
		return 0, err
	}

	content, err := strconv.Atoi(string(bytes))
	if err != nil {
		return 0, err
	}

	return float32(content) / 1000.0, nil
}
