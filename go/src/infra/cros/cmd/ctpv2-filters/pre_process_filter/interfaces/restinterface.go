// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package interfaces

import (
	"errors"
)

type SensorTestStats struct {
	Stats          string
	First_row_seq  int
	First_run_time string
	Last_row_seq   int
	Last_run_time  string
}

type SensorFormat struct {
	Test_id        string
	Board          string
	Latest_results string
	Test_stats     SensorTestStats
}

// CallForStability return stability data in the SensorFormat from StabiitySensor rest endpoint
func CallForStability() ([]SensorFormat, error) {
	// TODO IMPLEMENT
	return nil, errors.New("STABILITY SENSOR INTERFACE NOT IMPLEMENTED")
}
