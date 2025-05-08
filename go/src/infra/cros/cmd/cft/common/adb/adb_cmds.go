// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package adb

import (
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const (
	retryInterval         = 1 * time.Second
	DefaultCommandSeconds = 15
	DefaultRetryAttempts  = 1
)

func TeardownAdb(logger *log.Logger, addr string) error {
	if out, err := AdbCmd([]string{"-s", FmtAddr(addr), "disconnect"}, logger, DefaultRetryAttempts, DefaultCommandSeconds); err != nil {
		return fmt.Errorf("ADB disconnect failed. %w: %s", err, string(out))
	}
	return nil
}

func AdbConnect(logger *log.Logger, addr string) (err error) {
	// Assumes the ADB port on the CrOS device is visible directly.
	outStr, err := AdbCmd([]string{"connect", FmtAddr(addr)}, logger, DefaultRetryAttempts, DefaultCommandSeconds)
	if err != nil {
		return fmt.Errorf("ADB Start failed. %w: %s", err, outStr)
	}

	if !strings.Contains(outStr, fmt.Sprintf("connected to %s", FmtAddr(addr))) {
		return fmt.Errorf("ADB Start failed")
	}

	logger.Println("adb connected!:", outStr)

	return err
}

func AdbCmd(args []string, log *log.Logger, retryCount, deadlineSeconds int) (string, error) {
	// Define a function to run in the retry loop below.
	f := func(deadlineSeconds int, args []string) (string, string, error) {
		// Add a ctx timeout to prevent cmds from hanging.
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(deadlineSeconds)*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "adb", args...)
		log.Println("Running ADB SHELL CMD: ", cmd.String())

		out, err := cmd.CombinedOutput()
		if ctx.Err() == context.DeadlineExceeded {
			return "", "", fmt.Errorf("cmd %s exceeded deadline", cmd.String())
		}
		if err != nil {
			return "", "", err
		}

		outStr := string(out)
		return cmd.String(), outStr, nil
	}

	// Run the adb command in a retry loop.
	var outStr, cmd string
	var err error
	for range retryCount {
		cmd, outStr, err = f(deadlineSeconds, args)
		if err != nil {
			log.Printf("ADB command %s failed: %s", cmd, err.Error())
			continue
		}

		// If the command succeeded then break the loop and return the results.
		break
	}

	return outStr, nil
}

func RetrySetupAdb(log *log.Logger, addr string, timeout time.Duration) error {
	st := time.Now()
	for time.Since(st) < timeout {
		err := SetupAdb(log, addr)
		if err == nil {
			log.Println("Device online")

			return nil
		}
		time.Sleep(retryInterval)
	}
	log.Println("Device offline")

	return fmt.Errorf("device not online within timeout")

}

func SetupAdbConnections(logger *log.Logger, addrs []string) (err error) {
	for range 3 {
		for _, addr := range addrs {
			innerErr := AdbConnect(logger, addr)
			if innerErr != nil {
				logger.Printf("failed to connect to %s, %s", addr, innerErr)
				err = fmt.Errorf("%w\n%w", err, innerErr)
			}
		}

		allFound := true
		for _, addr := range addrs {
			found, innerErr := AdbDeviceFound(addr, logger)
			allFound = allFound && found
			if innerErr != nil {
				logger.Printf("Failed to find device %s, %s", addr, innerErr)
			}
		}

		if allFound {
			err = nil
			return
		}
	}

	return
}

func SetupAdb(logger *log.Logger, addr string) (err error) {
	err = AdbConnect(logger, addr)
	if err != nil {
		logger.Println("setupAdb failed. Trying again: ", err)
		// ADB can be in akward state requiring a retry often.
		err = AdbConnect(logger, addr)
		if err != nil {
			logger.Println("setupAdb failed Attempt 2. Exiting: ", err)

			return fmt.Errorf("setupAdb failed")

		}
	}
	found, err := AdbDeviceFound(addr, logger)
	if !found {
		return fmt.Errorf("ADB Start failed: %w", err)
	}

	return nil
}

// KeepAdbAlive spawns a go routine that will continuously check the
// adb connection to ensure the device is found. If not found, try
// to setup the adb connection again. Useful for reboot cycles.
func KeepAdbAlive(logger *log.Logger, addrs []string, exit chan struct{}) {
	go func(logger *log.Logger, addrs []string, exit chan struct{}) {
		// Constantly checking the adb connection can be quite loud in the logs.
		// Suppress logging through io.Discard.
		disabledLogger := log.New(io.Discard, "", 0)
		logger.Println("Starting adb connection check loop")
		for {
			select {
			case <-exit:
				logger.Println("Exiting adb connection check loop")
				return
			default:
				for _, addr := range addrs {
					found, _ := AdbDeviceFound(addr, disabledLogger)
					if !found {
						SetupAdb(disabledLogger, addr)
					}
				}
			}

			// Sleep to make checks more periodic rather than constant.
			time.Sleep(10 * time.Second)
		}
	}(logger, addrs, exit)
}

func AdbDeviceFound(addr string, logger *log.Logger) (bool, error) {
	outStr, err := AdbCmd([]string{"devices"}, logger, DefaultRetryAttempts, DefaultCommandSeconds)
	deviceRegex := regexp.MustCompile(fmt.Sprintf(`%s\sdevice`, FmtAddr(addr)))
	if !deviceRegex.MatchString(outStr) {

		return false, fmt.Errorf("ADB Devices CMd failed: %w, %s", err, outStr)
	}
	return true, nil
}

func FmtAddr(addr string) string {
	if strings.Contains(addr, ":") {
		// The adb port specified. No formatting is needed
		return addr
	}
	return fmt.Sprintf("%s:5555", addr)
}

func AdbShellCmd(cmdStr []string, addr string, log *log.Logger, retryCount, deadlineSeconds int) (string, error) {
	return AdbCmd(append([]string{"-s", FmtAddr(addr), "shell"}, cmdStr...), log, retryCount, deadlineSeconds)
}

func GetBuildVersion(addr string, log *log.Logger) (string, error) {
	s, err := AdbShellCmd([]string{"getprop", "ro.system.build.version.incremental"}, addr, log, DefaultRetryAttempts, DefaultCommandSeconds)
	s = strings.ReplaceAll(s, "\n", "")
	return s, err

}
