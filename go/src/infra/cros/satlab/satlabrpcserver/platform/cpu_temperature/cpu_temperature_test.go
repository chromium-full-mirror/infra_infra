// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package cpu_temperature

import (
	"os"
	"path/filepath"
	"testing"

	"go.chromium.org/infra/cros/satlab/satlabrpcserver/utils"
)

func TestGetCurrentTemperature(t *testing.T) {
	// Create a file
	fp, err := os.Create("test.data")
	if err != nil {
		t.Fatalf("Can't create a file")
	}
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			t.Fatalf("Can't delete a file")
		}
	}(fp.Name())

	// Write some data to the file
	_, err = fp.WriteString("6000")
	if err != nil {
		t.Fatalf("Can't close file")
	}

	// Close the file
	err = fp.Close()
	if err != nil {
		t.Fatalf("Can't close file")
	}

	// Get the path
	path, err := filepath.Abs("./test.data")

	// Init the FilePathCPUTemperature
	cpu := FilePathCPUTemperature{thermalZoneFilePath: path}

	// Get current temperature
	temperature, err := cpu.GetCurrentCPUTemperature()

	// Assert
	if err != nil {
		t.Errorf("Got an error %v", err)
	}

	if !utils.NearlyEqual(float64(temperature), 6.0) {
		t.Errorf("Not expected %v", temperature)
	}
}
