// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

// checkFingerprintInfo confirms that fingerprint mcu and sensor from probing the fpmcu match the fingerprint board found in cros_config.
func checkFingerprintInfo(board, mcu, sensor string) bool {
	// TODO: This table must be updated when new boards are added to match go/cros-fingerprint-docs#mcu-sensor-table.
	// Please use go/cros-fp-bug template and assign to chromeos-fingerprint@google.com
	validCombinations := map[string]map[string]string{
		"bloonchipper": {"stm32f412": "210"},
		"dartmonkey":   {"stm32h7x3": "1400"},
		"nami_fp":      {"stm32h7x3": "1400"},
		"nocturne_fp":  {"stm32h7x3": "1400"},
		"helipilot":    {"NPCX99FP": "210"},
		"buccaneer":    {"NPCX99FP": "4f4f"},
		"None":         {"None": "None"},
	}

	if mcus, ok := validCombinations[board]; ok {
		if sensors, ok := mcus[mcu]; ok {
			return sensors == sensor
		}
	}

	return false

}

// collectFingerprintExec read fingerprint_mcu, fingerprint_board, and fingerprint_sensor from dut to inventory.
//
// Find the fingerprint info with commands and set it on ChromeOS struct
func collectFingerprintExec(ctx context.Context, info *execs.ExecInfo) error {
	r := info.DefaultRunner()
	cros := info.GetChromeos()
	if cros == nil {
		return errors.Reason("collect fingerprint: only for chromeos devices").Err()
	}
	log.Debugf(ctx, "Fingerprint info before update: %s", cros.GetFingerprint())

	// command to grab the fingerprint_board from dut
	const boardCmd = `cros_config /fingerprint board`
	board, err := r(ctx, info.GetExecTimeout(), boardCmd)
	if err != nil {
		// In the error case, we expect this was a device without fingerprint.
		board = "None"
		log.Debugf(ctx, "collect fingerprint_board returned error: ", err)
	}
	if board == "" {
		// Nami devices without fingerprint return blank string.
		board = "None"
	}

	// command to grab the fingerprint_mcu from dut
	const mcuCmd = `ectool --name=cros_fp chipinfo | awk '/name:/{print $2}'`
	mcu, err := r(ctx, info.GetExecTimeout(), mcuCmd)
	if err != nil {
		return errors.Annotate(err, "collect fingerprint_mcu").Err()
	}
	if mcu == "" {
		mcu = "None"
	} else {
		// handle mcu running zephyr which may have more specific name.
		if strings.HasPrefix(mcu, "stm32f412") {
			mcu = "stm32f412"
		} else if strings.HasPrefix(mcu, "stm32h7") {
			mcu = "stm32h7x3"
		}
	}

	// command to grab the fingerprint sensor and vendor from dut
	const sensorCmd = `ectool --name=cros_fp fpinfo`
	const fpcVendorID = `20435046`
	reVendor := regexp.MustCompile(`^Fingerprint sensor:[^\S\r\n]*vendor\s+(\S+)\s+product`)
	reSensor := regexp.MustCompile(`^Fingerprint sensor:[^\S\r\n]*vendor.+model\s+(\S+)\s+version`)
	sensor := ""
	sensorCmdOut, _ := r(ctx, info.GetExecTimeout(), sensorCmd)
	if sensorCmdOut == "" {
		sensor = "None"
	} else {
		sensorVendor := reVendor.FindStringSubmatch(sensorCmdOut)[1]
		sensor = reSensor.FindStringSubmatch(sensorCmdOut)[1]
		if sensorVendor == fpcVendorID {
			intsensorMasked, err := strconv.ParseInt(sensor, 16, 64)
			if err != nil {
				return errors.Reason("Probed FPC vendor, but model is not hex string: %s", sensor).Err()
			} else {
				intsensorMasked &= ^0xf // Mask off the last four bits
				sensor = fmt.Sprintf("%x", intsensorMasked)
			}
		}
	}

	if checkFingerprintInfo(board, mcu, sensor) {
		cros.Fingerprint.Board = strings.ToUpper(board)
		cros.Fingerprint.Mcu = strings.ToUpper(mcu)
		cros.Fingerprint.Sensor = strings.ToUpper(sensor)
		log.Debugf(ctx, "Fingerprint data: %s", cros.GetFingerprint())
		return nil
	}
	return errors.Reason("Unexpected fingerprint hw/sw combo. Board: %s, Mcu:%s, Sensor: %s", board, mcu, sensor).Err()
}

func isValidFingerprintBoardExec(ctx context.Context, info *execs.ExecInfo) error {
	res, err := info.DefaultHostAccess().Run(ctx, info.GetExecTimeout(), `cros_config /fingerprint board`)
	if err != nil {
		return errors.Annotate(err, "is fingerprint board specified").Err()
	}
	board := res.GetStdout()
	log.Debugf(ctx, "Fingerprint board: %q", board)
	if board == "" {
		return errors.Reason("is fingerprint board specified: invalid board value").Err()
	}
	return nil
}

func init() {
	execs.Register("cros_collect_fingerprint", collectFingerprintExec)
	execs.Register("cros_is_valid_fingerprint_board", isValidFingerprintBoardExec)
}
