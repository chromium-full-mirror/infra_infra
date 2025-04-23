// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tlslib provides the canonical implementation of a common TLS server.
package tlslib

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"golang.org/x/crypto/ssh"
)

const (
	firmwareUpdaterPath = "/usr/sbin/chromeos-firmwareupdate"
)

type FirmwareManifest map[string]FirmwareManifestData

type FirmwareManifestData struct {
	Host struct {
		Versions struct {
			Ro string `json:"ro"`
			Rw string `json:"rw"`
		} `json:"versions"`
		Keys struct {
			Root     string `json:"root"`
			Recovery string `json:"recovery"`
		} `json:"keys"`
		Image string `json:"image"`
	} `json:"host"`
	Ec struct {
		Versions struct {
			Ro string `json:"ro"`
			Rw string `json:"rw"`
		} `json:"versions"`
		Image string `json:"image"`
	} `json:"ec"`
	SignatureId string `json:"signature_id"`
}

var firmwareManifestRegexp = regexp.MustCompile("FIRMWARE_MANIFEST_KEY='(.*)'")

// getAvailableFirmwareVersion read firmware manifest from current OS and extract available firmware version based on model.
func getAvailableFirmwareVersion(c *ssh.Client) (string, error) {
	out, err := runCmdOutput(c, fmt.Sprintf("%s --manifest", firmwareUpdaterPath))
	if err != nil {
		return "", fmt.Errorf("getAvailableFirmwareVersion: failed to get firmware manifest, %w", err)
	}
	var manifest FirmwareManifest
	if err := json.Unmarshal([]byte(out), &manifest); err != nil {
		return "", fmt.Errorf("getAvailableFirmwareVersion: failed to unmarshal firmware manifest, %w", err)
	}
	fwModel, err := getFirmwareTarget(c)
	if err != nil {
		return "", fmt.Errorf("getAvailableFirmwareVersion: failed to get firmware target %w", err)
	}
	if data, ok := manifest[fwModel]; ok {
		log.Printf("Available firmware from the new OS: %s.", data.Host.Versions.Rw)
		return data.Host.Versions.Rw, nil
	}
	return "", fmt.Errorf("getAvailableFirmwareVersion: failed to get firmware data of key %s from manifest, %w", fwModel, err)
}

// getFirmwareTarget returns firmware target of the DUT, which will be used to as key to fetch expected firmware from manifest.
func getFirmwareTarget(c *ssh.Client) (string, error) {
	out, err := runCmdOutput(c, "crosid")
	if err != nil {
		return "", err
	}
	fwLine := firmwareManifestRegexp.FindString(out)
	if fwLine != "" {
		return strings.TrimPrefix(strings.TrimSuffix(fwLine, "'"), "FIRMWARE_MANIFEST_KEY='"), nil
	}
	return "", fmt.Errorf("getFirmwareTarget: unable to parse FIRMWARE_MANIFEST_KEY from crosid.")
}

// getCurrentFirmwareVersion read current system firmware version on the DUT.
func getCurrentFirmwareVersion(c *ssh.Client) (string, error) {
	out, err := runCmdOutput(c, "crossystem fwid")
	if err != nil {
		return "", fmt.Errorf("getCurrentFirmwareVersion: failed to read current system firmware, %w", err)
	}
	log.Printf("Current firmware on DUT: %s.", out)
	return out, nil
}

// updateFirmware update DUT's firmware(RW) to current available version from OS image.
func (p *provisionState) updateFirmware(ctx context.Context) (bool, error) {
	if err := runCmd(p.c, fmt.Sprintf("%s --wp=1 --mode=autoupdate", firmwareUpdaterPath)); err != nil {
		return false, fmt.Errorf("updateFirmware: failed to execute chromeos-firmwareupdate, %w", err)
	}
	fwChanged, err := isFirmwareSlotChanged(p.c)
	if err != nil {
		return false, err
	}
	if p.preventReboot {
		log.Printf("updateFirmware: reboot prevented by request")
		return fwChanged, nil
	}
	if fwChanged {
		log.Printf("Firmware slot changed on next boot, rebooting the DUT.")
		if err := rebootDUT(ctx, p.c); err != nil {
			return fwChanged, fmt.Errorf("updateFirmware: failed to reboot DUT, %w", err)
		}
	}
	return fwChanged, nil
}

func isFirmwareSlotChanged(c *ssh.Client) (bool, error) {
	current, err := runCmdOutput(c, "crossystem mainfw_act")
	if err != nil {
		return false, fmt.Errorf("isFirmwareSlotChanged: failed to get current active main firmware slot, %w", err)
	}
	next, err := runCmdOutput(c, "crossystem fw_try_next")
	if err != nil {
		return false, fmt.Errorf("isFirmwareSlotChanged: failed to get next main firmware slot, %w", err)
	}
	log.Printf("Current active firmware slot: %s, next boot firmware slot: %s", current, next)
	return current != next, nil
}

func checkFirmwareUpdaterExist(c *ssh.Client) error {
	return runCmd(c, fmt.Sprintf("test -f %s", firmwareUpdaterPath))
}
