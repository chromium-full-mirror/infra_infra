// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package starfish contains utilities for interacting with starfish devices.
package starfish

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/components"
	"infra/cros/recovery/internal/components/cros"
	"infra/cros/recovery/internal/components/cros/cellular"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/internal/retry"
	"infra/cros/recovery/tlw"
)

// getDevice gets the starfish device path.
func getDevice(ctx context.Context, runner components.Runner) (string, error) {
	// Get the symlink to the starfish device by finding a device with Starfish in its ID.
	const idCmd = "find /dev/serial/by-id/ | grep Starfish"
	idDevPath, err := runner(ctx, 5*time.Second, idCmd)
	if err != nil {
		return "", errors.Annotate(err, "get device: failed to find device with starfish in id").Err()
	}
	idDevPath = strings.TrimSpace(idDevPath)

	if len(strings.Split(idDevPath, "\n")) != 1 {
		return "", errors.Reason("get device: more than one starfish found").Err()
	}

	// Get the underlying dev path.
	cmd := fmt.Sprintf("readlink -f %q", idDevPath)
	devPath, err := runner(ctx, 5*time.Second, cmd)
	if err != nil {
		return "", errors.Annotate(err, "get device: failed to find device with starfish in id").Err()
	}
	return strings.TrimSpace(devPath), nil
}

// EjectSIM ejects any connected SIM.
func EjectSIM(ctx context.Context, runner components.Runner) error {
	device, err := getDevice(ctx, runner)
	if err != nil {
		return errors.Annotate(err, "eject sim: failed to find starfish device").Err()
	}

	if _, err := sendCmd(ctx, runner, "sim eject", device); err != nil {
		return errors.Annotate(err, "eject sim: failed to send eject command").Err()
	}
	return nil
}

// InsertSIM inserts the SIM in the given slot.
func InsertSIM(ctx context.Context, runner components.Runner, slot int) error {
	device, err := getDevice(ctx, runner)
	if err != nil {
		return errors.Annotate(err, "insert sim: failed to find starfish device").Err()
	}
	cmd := fmt.Sprintf("sim connect -n %d", slot)
	if out, err := sendCmd(ctx, runner, cmd, device); err != nil {
		return errors.Annotate(err, "insert sim: failed to switch SIM slot").Err()
	} else if !strings.Contains(out, fmt.Sprintf("Mux set to %d", slot)) {
		return errors.Reason("insert sim: failed to verify that device was set to correct slot").Err()
	}

	// Normally, when you insert a SIM the device is able to detect the SIM being
	// physically inserted into the slot. Since this is an electronic switch the
	// device may not notice the new SIM. Power cycle it to ensure we have the correct SIM.
	if err := cros.Reboot(ctx, runner, 5*time.Second); err != nil {
		// cros reboot will always time out.
		log.Infof(ctx, "Error received during reboot: %v", err)
	}
	if err := cros.WaitUntilSSHable(ctx, 120*time.Second, 5*time.Second, runner); err != nil {
		return errors.Annotate(err, "insert sim: failed to connect to device after rebooting").Err()
	}

	// Make sure we're on a PSIM slot first.
	predicate := func(sim *tlw.Cellular_SIMInfo) bool {
		return sim.GetType() == tlw.Cellular_SIM_PHYSICAL
	}

	// Wait for the SIM to be detected.
	return retry.WithTimeout(ctx, time.Second, 60*time.Second, func() error {
		if err := cellular.SwitchToMatchingSIMSlot(ctx, runner, predicate); err != nil {
			return errors.Annotate(err, "failed to switch to psim slot").Err()
		}
		simInfo, err := cellular.GetSIMInfo(ctx, runner)
		if err != nil {
			return errors.Annotate(err, "failed to query info for sim slot: %d", slot).Err()
		}
		// If the SIM slot is empty, it may just not be detected yet.
		if simInfo == nil || len(simInfo.GetProfileInfos()) == 0 {
			return errors.Reason("sim info is empty").Err()
		}
		return nil
	}, "insert sim")
}

// GetAllSIMInfo switches to all occupied starfish slots and queries the SIM info.
func GetAllSIMInfo(ctx context.Context, runner components.Runner) ([]*tlw.Cellular_SIMInfo, error) {
	slots, err := GetOccupiedSlots(ctx, runner)
	if err != nil {
		return nil, errors.Annotate(err, "get all sim info: failed to determine occupied SIM slots").Err()
	}

	// Try to eject the SIM as cleanup.
	defer func() {
		if err := EjectSIM(ctx, runner); err != nil {
			log.Errorf(ctx, "Failed to eject SIM from starfish: %v", err)
		}
	}()

	infos := make([]*tlw.Cellular_SIMInfo, 0)
	for _, slot := range slots {
		if err := InsertSIM(ctx, runner, slot); err != nil {
			return nil, errors.Annotate(err, "get all sim info: failed to insert sim into starfish").Err()
		}

		simInfo, err := cellular.GetSIMInfo(ctx, runner)
		if err != nil {
			return nil, errors.Annotate(err, "get all sim info: failed to query info for sim slot: %d", slot).Err()
		}

		// Starfish 0 indexes while ModemManager 1 indexes SIM slots.
		simInfo.SlotId = int32(slot) + 1
		infos = append(infos, simInfo)
	}
	return infos, nil
}

// GetOccupiedSlots returns a list of the occupied SIM slots on the starfish.
func GetOccupiedSlots(ctx context.Context, runner components.Runner) ([]int, error) {
	device, err := getDevice(ctx, runner)
	if err != nil {
		return nil, errors.Annotate(err, "get occupied slots: failed to find starfish device").Err()
	}

	/*
		Get the status of the SIMs in the starfish.

		example output:
		Starfish:~$ sim status
		[0000000027318797] <inf> console: SIM 0 = Found
		[0000000027318797] <inf> console: SIM 1 = Found
		[0000000027318797] <inf> console: SIM 2 = Found
		[0000000027318797] <inf> console: SIM 3 = Found
		[0000000027318797] <inf> console: SIM 4 = None
		[0000000027318798] <inf> console: SIM 5 = None
		[0000000027318798] <inf> console: SIM 6 = None
		[0000000027318798] <inf> console: SIM 7 = None
		Starfish:~$
	*/
	out, err := sendCmd(ctx, runner, "sim status", device)
	if err != nil {
		return nil, errors.Annotate(err, "get occupied slots: failed to find starfish device").Err()
	}

	slots, err := parseActiveSIMSlots(out)
	if err != nil {
		return nil, errors.Annotate(err, "get occupied slots: failed to parse 'sim status' output: %s", out).Err()
	}
	log.Infof(ctx, "Found occupied slots: %v", slots)
	return slots, nil
}

// simStatusRegex returns a match on an occupied SIM slot in the starfish, the match
// is the integer slot of the occupied slot e.g.
//
//	[0000000027318797] <inf> console: SIM 0 = Found
//
// would return a match with value "0"
var simStatusRegex = regexp.MustCompile(`SIM\s?(\d+)\s?=\s?Found`)

// parseActiveSIMSlots parses the output of the starfish "sim status" command.
func parseActiveSIMSlots(out string) ([]int, error) {
	var res []int
	sims := make(map[int]bool)
	for _, line := range strings.Split(out, "\n") {
		m := simStatusRegex.FindStringSubmatch(line)
		if m == nil || len(m) != 2 {
			continue
		}

		i, err := strconv.Atoi(m[1])
		if err != nil {
			return nil, errors.Reason("parse occupied sim slots: failed to parse slot id: %v", m).Err()
		}
		if sims[i] {
			return nil, errors.Reason("parse occupied sim slots: duplicate SIM slot: %d", i).Err()
		}

		sims[i] = true
		res = append(res, i)
	}

	return res, nil
}

// sendCmd sends a command to the starfish device and returns the output.
//
// The starfish won't return the output of the command directly. Instead we read the
// starfish output for several seconds after sending the command. Since the starfish
// is a device and not a regular file there is no EOF in it's stdout for us to terminate
// on either so we force terminate the read using the "timeout" command.
func sendCmd(ctx context.Context, runner components.Runner, cmd, device string) (string, error) {
	// Configure tty for the device.
	configCmd := fmt.Sprintf("stty -F %q 115200 raw -echo", device)
	if _, err := runner(ctx, 5*time.Second, configCmd); err != nil {
		return "", errors.Annotate(err, "send cmd: failed to configure starfish tty").Err()
	}

	// Send command to the device.
	sendCmd := fmt.Sprintf("echo -ne %q > %q", "\r"+cmd+"\r", device)
	if _, err := runner(ctx, 15*time.Second, sendCmd); err != nil {
		return "", errors.Annotate(err, "send cmd: failed to run starfish command").Err()
	}

	// Read the current stdout buffer and all output for the next several seconds.
	// There is no EOF returned by the starfish device so we have no concrete way to know
	// when it is done outputting data, instead we will force terminate the command using "timeout."
	readCmd := fmt.Sprintf("timeout 5 cat %q || true", device)
	out, err := runner(ctx, 10*time.Second, readCmd)
	if err != nil {
		return "", errors.Annotate(err, "send cmd: failed to get starfish output").Err()
	}
	return strings.TrimSpace(out), nil
}
