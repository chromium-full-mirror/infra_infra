// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tlslib provides the canonical implementation of a common TLS server.
package tlslib

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/chromiumos/config/go/api/test/tls"

	"infra/cros/internal/env"
)

const (
	// provisionFailed - A flag file to indicate provision failures.
	// The file's location in stateful means that on successful update
	// it will be removed.  Thus, if this file exists, it indicates that
	// we've tried and failed in a previous attempt to update.
	// The file will be created every time a OS provision is kicked off.
	// TODO(b/229309510): Remove when lab uses the latter marker.
	provisionFailed       = "/var/tmp/provision_failed"
	provisionFailedMarker = "/mnt/stateful_partition/unencrypted/provision_failed"

	verificationTimeout = 5 * time.Minute
	rebootTimeout       = 10 * time.Minute
)

func (s *Server) provision(req *tls.ProvisionDutRequest, opName string) {
	log.Printf("provision: started %v on hostname=%v", opName, req.GetName())

	// Set a timeout for provisioning.
	// TODO(kimjae): Tie the context with timeout to op by passing to lroMgr.
	ctx, cancel := context.WithTimeout(s.ctx, time.Hour)
	defer cancel()

	startTime := time.Now()
	defer func() {
		provisionDutCounter.Add(ctx, 1)
		log.Printf("provision: finished %v on hostname=%v", opName, req.GetName())
		log.Printf("provision: time to provision took %v", time.Since(startTime))
	}()

	var p *provisionState
	createProvisionFailedMarker := func() {
		if p == nil || p.c == nil {
			return
		}
		if err := runCmd(p.c, fmt.Sprintf("touch %s %s", provisionFailed, provisionFailedMarker)); err != nil {
			log.Printf("createProvisionFailedMarker: Warning, failed to create provision failed marker, %s", err)
		}
	}

	setError := func(opErr *status.Status) {
		createProvisionFailedMarker()
		if err := s.lroMgr.SetError(opName, opErr); err != nil {
			log.Printf("provision: failed to set Operation error, %s", err)
		}
	}

	p, err := newProvisionState(s, req)
	if err != nil {
		setError(newOperationError(
			codes.InvalidArgument,
			fmt.Sprintf("provision: failed to create provisionState, %s", err),
			tls.ProvisionDutResponse_REASON_INVALID_REQUEST.String()))
		return
	}

	// Verify that the DUT is reachable.
	addr, err := s.getSSHAddr(ctx, req.GetName())
	if err != nil {
		setError(newOperationError(
			codes.InvalidArgument,
			fmt.Sprintf("provision: DUT SSH address unattainable prior to provisioning, %s", err),
			tls.ProvisionDutResponse_REASON_INVALID_REQUEST.String()))
		return
	}

	// Connect to the DUT.
	initialSSHCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	disconnect, err := p.connect(initialSSHCtx, addr)
	if err != nil {
		setError(newOperationError(
			codes.FailedPrecondition,
			fmt.Sprintf("provision: DUT unreachable prior to provisioning (SSH client), %s", err),
			tls.ProvisionDutResponse_REASON_DUT_UNREACHABLE_PRE_PROVISION.String()))
		return
	}
	defer disconnect()

	// Create a marker so the lab knows to repair the device on failure.
	createProvisionFailedMarker()

	if !checkIfLabstationDevice(p.c) {
		// Check if the DUT has KVM enabled.
		kvmEnabled, err := checkKvmEnabled(p.c)
		if err != nil {
			setError(newOperationError(
				codes.FailedPrecondition,
				"provision: failed to check if KVM enabled on this device",
				tls.ProvisionDutResponse_REASON_PROVISIONING_FAILED.String()))
			return
		}
		if !kvmEnabled {
			setError(newOperationError(
				codes.FailedPrecondition,
				"provision: KVM is not enabled on this device, provisioning it again will not help (repair needs to cold reboot this device)",
				tls.ProvisionDutResponse_REASON_PROVISIONING_FAILED.String()))
			return
		}
	}

	// Provision the OS.
	select {
	case <-ctx.Done():
		setError(newOperationError(
			codes.DeadlineExceeded,
			"provision: timed out before provisioning OS",
			tls.ProvisionDutResponse_REASON_PROVISIONING_TIMEDOUT.String()))
		return
	default:
	}

	if p.shouldProvisionOS() {
		t := time.Now()
		if err := p.provisionOS(ctx); err != nil {
			setError(newOperationError(
				codes.Aborted,
				fmt.Sprintf("provision: failed to provision OS, %s", err),
				tls.ProvisionDutResponse_REASON_PROVISIONING_FAILED.String()))
			return
		}

		// Wait for DUT to come up after provisioning the OS.
		// Can continue as soon as a connection can be established.
		// Give extra time in case of firmware updates prior to UI spawning.
		rebootWaitCtx, cancel := context.WithTimeout(ctx, rebootTimeout)
		defer cancel()
		disconnect, err = p.connect(rebootWaitCtx, addr)
		if err != nil {
			setError(newOperationError(
				codes.Aborted,
				fmt.Sprintf("provision: failed to wait for DUT to come up after provisioning the OS, %s", err),
				tls.ProvisionDutResponse_REASON_PROVISIONING_FAILED.String()))
			return
		}
		defer disconnect()
		log.Printf("DUT came up after provisioning the OS.")

		// Should shorten the time waiting for reboot to complete booting into the new OS.
		// Certain images will take significantly longer due to debug features being enabled.
		// Follow through subsequent reboots until "ui" job is running.
		uiStabilizeCtx, cancel := context.WithTimeout(ctx, rebootTimeout)
		defer cancel()
		if err := p.waitForUIToStabilize(uiStabilizeCtx, addr); err != nil {
			setError(newOperationError(
				codes.Aborted,
				fmt.Sprintf("provision: failed to wait for UI to stabilize, %s", err),
				tls.ProvisionDutResponse_REASON_PROVISIONING_FAILED.String()))
			return
		}

		shouldUpdateFirmware := req.UpdateFirmware
		if shouldUpdateFirmware {
			// Firmware updater may not exist on some builds(e.g. chromiumos build).
			if err := checkFirmwareUpdaterExist(p.c); err != nil {
				log.Printf("Firmware updater does not exist, skipping firmware update.")
				shouldUpdateFirmware = false
			}
		} else {
			log.Printf("Skipping firmware update by request.")
		}

		if shouldUpdateFirmware {
			fwChanged, err := p.updateFirmware(ctx)
			if err != nil {
				setError(newOperationError(
					codes.Aborted,
					fmt.Sprintf("provision: failed to update firmware, %s", err),
					tls.ProvisionDutResponse_REASON_UPDATE_FIRMWARE_FAILED.String()))
				return
			}
			// If firmware changed and reboot doesn't get blocked by request, make sure DUT comes back after post update reboot.
			if fwChanged && !req.PreventReboot {
				fwRebootWaitCtx, cancel := context.WithTimeout(ctx, rebootTimeout)
				defer cancel()
				disconnect, err = p.connect(fwRebootWaitCtx, addr)
				if err != nil {
					setError(newOperationError(
						codes.Aborted,
						fmt.Sprintf("provision: failed to wait for DUT to come up after firmware update, %s", err),
						tls.ProvisionDutResponse_REASON_UPDATE_FIRMWARE_FAILED.String()))
					return
				}
				defer disconnect()
				log.Printf("DUT came up after updated firmware from the new OS.")
			}
			// Next we need to make sure firmware is updated to expected version when possible to prevent false positive from updater.
			curFW, err := getCurrentFirmwareVersion(p.c)
			if err != nil {
				setError(newOperationError(
					codes.Aborted,
					fmt.Sprintf("provision: failed to get current system firmware info, %s", err),
					tls.ProvisionDutResponse_REASON_UPDATE_FIRMWARE_FAILED.String()))
				return
			}
			expectedFw, err := getAvailableFirmwareVersion(p.c)
			if err != nil {
				// Forgive the this error since it's not a firmware or updater issue.
				log.Printf("(Non-critical) Cannot get available firmware version from the DUT, %s", err.Error())
			}

			if fwChanged {
				if expectedFw == "" {
					log.Printf("(Non-critical) Firmware updater ran success, but unable get expected firmware to verify against.")
				} else if p.preventReboot {
					log.Printf("Firmware updated but prevent reboot is requested, expected firmware(in the next reboot): %s", expectedFw)
				} else if expectedFw != curFW {
					setError(newOperationError(
						codes.Aborted,
						fmt.Sprintf("provision: firmware didn't updated to expected version, current version: %s, expected version: %s", curFW, expectedFw),
						tls.ProvisionDutResponse_REASON_UPDATE_FIRMWARE_FAILED.String()))
					return
				} else {
					log.Printf("Firmware update completed successfully, it has been updated to %s.", curFW)
				}
			} else {
				log.Printf("Current system firmware: %s is already matched with OS image.", curFW)
			}
		}
		log.Printf("provision: time to provision OS took %v", time.Since(t))

		// To be safe, wait for kernel to be "sticky" right after installing the new partitions and booting into it.
		t = time.Now()
		// Timeout is determined by 2x delay to mark new kernel successful + 10 seconds fuzz.
		stickyKernelCtx, cancel := context.WithTimeout(ctx, 100*time.Second)
		defer cancel()
		if err := p.verifyKernelState(stickyKernelCtx); err != nil {
			setError(newOperationError(
				codes.Aborted,
				fmt.Sprintf("provision: failed to wait for sticky kernel, %s", err),
				tls.ProvisionDutResponse_REASON_PROVISIONING_FAILED.String()))
			return
		}
		log.Printf("provision: time to wait for sticky kernel %v", time.Since(t))

		t = time.Now()
		if err := p.provisionStateful(ctx); err != nil {
			setError(newOperationError(
				codes.Aborted,
				fmt.Sprintf("provision: failed to provision stateful, %s", err),
				tls.ProvisionDutResponse_REASON_PROVISIONING_FAILED.String()))
			return
		}
		log.Printf("provision: time to provision stateful took %v", time.Since(t))

		// After a reboot, need a new client connection.
		sshCtx, cancel := context.WithTimeout(ctx, rebootTimeout)
		defer cancel()

		disconnect, err := p.connect(sshCtx, addr)
		if err != nil {
			setError(newOperationError(
				codes.Aborted,
				fmt.Sprintf("provision: failed to connect to DUT after stateful update and reboot, %s", err),
				tls.ProvisionDutResponse_REASON_PROVISIONING_FAILED.String()))
			return
		}
		defer disconnect()

		if !req.PreventReboot {
			t = time.Now()
			verifyCtx, cancel := context.WithTimeout(ctx, verificationTimeout)
			defer cancel()
			if err := p.verifyOSProvision(verifyCtx); err != nil {
				setError(newOperationError(
					codes.Aborted,
					fmt.Sprintf("provision: failed to verify OS provision, %s", err),
					tls.ProvisionDutResponse_REASON_PROVISIONING_FAILED.String()))
				return
			}
			log.Printf("provision: time to verify provision took %v", time.Since(t))
		}
	} else if isStatefulCorrupt(p.c) || mismatchStatefulCheck(p.c) {
		log.Printf("provision: Stateful is corrupt, provisioning stateful partition.")
		t := time.Now()
		if !req.GetPreserveStateful() && !req.PreventReboot {
			if err := p.wipeStateful(ctx); err != nil {
				setError(newOperationError(
					codes.Aborted,
					fmt.Sprintf("provision: failed to wipe stateful, %s", err),
					tls.ProvisionDutResponse_REASON_PROVISIONING_FAILED.String()))
				return
			}
			// After a reboot, need a new client connection.
			sshCtx, cancel := context.WithTimeout(ctx, rebootTimeout)
			defer cancel()

			disconnect, err := p.connect(sshCtx, addr)
			if err != nil {
				setError(newOperationError(
					codes.Aborted,
					fmt.Sprintf("provision: failed to connect to DUT after wipe reboot, %s", err),
					tls.ProvisionDutResponse_REASON_PROVISIONING_FAILED.String()))
				return
			}
			defer disconnect()
		}
		log.Printf("provision: time to wipe stateful took %v", time.Since(t))

		t = time.Now()
		if err := p.provisionStateful(ctx); err != nil {
			setError(newOperationError(
				codes.Aborted,
				fmt.Sprintf("provision: failed to provision stateful, %s", err),
				tls.ProvisionDutResponse_REASON_PROVISIONING_FAILED.String()))
			return
		}

		// After a reboot, need a new client connection.
		sshCtx, cancel := context.WithTimeout(ctx, rebootTimeout)
		defer cancel()

		disconnect, err := p.connect(sshCtx, addr)
		if err != nil {
			setError(newOperationError(
				codes.Aborted,
				fmt.Sprintf("provision: failed to connect to DUT after mismatching/corrupt stateful fix and reboot, %s", err),
				tls.ProvisionDutResponse_REASON_PROVISIONING_FAILED.String()))
			return
		}
		defer disconnect()

		log.Printf("provision: time to provision stateful took %v", time.Since(t))
	} else {
		log.Printf("provision: Operation=%s skipped as DUT is already on builder path %s", opName, p.targetBuilderPath)
	}

	// Provision DLCs.
	select {
	case <-ctx.Done():
		setError(newOperationError(
			codes.DeadlineExceeded,
			"provision: timed out before provisioning DLCs",
			tls.ProvisionDutResponse_REASON_PROVISIONING_TIMEDOUT.String()))
		return
	default:
	}
	if err := p.provisionDLCs(ctx, req.GetDlcSpecs()); err != nil {
		setError(newOperationError(
			codes.Aborted,
			fmt.Sprintf("provision: failed to provision DLCs, %s", err),
			tls.ProvisionDutResponse_REASON_PROVISIONING_FAILED.String()))
		return
	}

	// Provision miniOS.
	select {
	case <-ctx.Done():
		setError(newOperationError(
			codes.DeadlineExceeded,
			"provision: timed out before provisioning miniOS",
			tls.ProvisionDutResponse_REASON_PROVISIONING_TIMEDOUT.String()))
		return
	default:
	}
	if err := p.provisionMiniOS(ctx); err != nil {
		// Initially failing to provision miniOS partitions isn't a failure.
		log.Printf("provision: failed to provision miniOS partitions, check partition table, %s", err)
	}

	// Remove the provisionFailed marker as provisioning stateful is skipped if OS
	// is already on the requested version.
	if err := runCmd(p.c, fmt.Sprintf("rm %s %s", provisionFailed, provisionFailedMarker)); err != nil {
		log.Printf("provision: Warning, failed to remove provision failed marker, %s", err)
	}

	if bootID, err := getBootID(p.c); err != nil {
		log.Printf("provision: Warning, failed to get boot ID")
	} else {
		log.Printf("provision: boot ID is %s", bootID)
	}

	// Finish provisioning.
	if err := s.lroMgr.SetResult(opName, &tls.ProvisionDutResponse{}); err != nil {
		log.Printf("provision: failed to set Operation result, %s", err)
	}
}

func (s *Server) provisionLacros(req *tls.ProvisionLacrosRequest, opName string) {
	log.Printf("provisionLacros: started %v on hostname=%v", opName, req.GetName())
	defer log.Printf("provisionLacros: finished %v on hostname=%v", opName, req.GetName())

	// Set a timeout for provisioning Lacros.
	// TODO(kimjae): Tie the context with timeout to op by passing to lroMgr.
	ctx, cancel := context.WithTimeout(s.ctx, time.Hour)
	defer cancel()

	setError := func(opErr *status.Status) {
		if err := s.lroMgr.SetError(opName, opErr); err != nil {
			log.Printf("provision: failed to set Operation error, %s", err)
		}
	}

	p, err := newProvisionLacrosState(s, req)
	if err != nil {
		setError(newOperationError(
			codes.InvalidArgument,
			fmt.Sprintf("provisionLacros: failed to create provisionLacrosState, %s", err),
			tls.ProvisionLacrosResponse_REASON_INVALID_REQUEST.String()))
		return
	}

	// Verify that the DUT is reachable.
	addr, err := s.getSSHAddr(ctx, req.GetName())
	if err != nil {
		setError(newOperationError(
			codes.InvalidArgument,
			fmt.Sprintf("provisionLacros: DUT SSH address unattainable prior to provisioning Lacros, %s", err),
			tls.ProvisionLacrosResponse_REASON_INVALID_REQUEST.String()))
		return
	}

	// Connect to the DUT.
	disconnect, err := p.connect(ctx, addr)
	if err != nil {
		setError(newOperationError(
			codes.FailedPrecondition,
			fmt.Sprintf("provisionLacros: DUT unreachable prior to provisioning Lacros (SSH client), %s", err),
			tls.ProvisionLacrosResponse_REASON_DUT_UNREACHABLE_PRE_PROVISION.String()))
		return
	}
	defer disconnect()

	// Provision Lacros onto the DUT.
	if err := p.provisionLacros(ctx); err != nil {
		setError(newOperationError(
			codes.Aborted,
			fmt.Sprintf("provisionLacros: failed to provision Lacros onto the DUT, %s", err),
			tls.ProvisionLacrosResponse_REASON_PROVISIONING_FAILED.String()))
		return
	}

	// Lacros provisioning was successful!
	if err := s.lroMgr.SetResult(opName, &tls.ProvisionLacrosResponse{}); err != nil {
		log.Printf("provisionLacros: failed to set Opertion result for %v, %s", opName, err)
	}
}

// runCmd interprets the given string command in a shell and returns the error if any.
func runCmd(c *ssh.Client, cmd string) error {
	s, err := c.NewSession()
	if err != nil {
		return fmt.Errorf("runCmd: failed to create session, %v", err)
	}
	defer s.Close()

	s.Stdin = strings.NewReader(cmd)
	s.Stdout = os.Stdout
	s.Stderr = os.Stderr

	log.Printf("Running command: %s", cmd)
	// Always run commands under /bin/bash.
	err = s.Run("/bin/bash -")

	if err != nil {
		return fmt.Errorf("runCmd: failed to run command, %v", err)
	}

	return nil
}

// runCmdRetry is runCmd with retries with context.
func runCmdRetry(ctx context.Context, c *ssh.Client, retryLimit uint, cmd string) error {
	var err error
	for ; retryLimit != 0; retryLimit-- {
		select {
		case <-ctx.Done():
			return fmt.Errorf("runCmdRetry: timeout reached, %w", err)
		default:
		}
		retryErr := runCmd(c, cmd)
		if retryErr == nil {
			return nil
		}

		// Wrap the retry errors.
		if err == nil {
			err = retryErr
		} else {
			err = fmt.Errorf("%s, %w", err, retryErr)
		}
		time.Sleep(2 * time.Second)
	}
	return err
}

// runCmdOutput interprets the given string command in a shell and returns stdout.
func runCmdOutput(c *ssh.Client, cmd string) (string, error) {
	s, err := c.NewSession()
	if err != nil {
		return "", err
	}
	defer s.Close()

	stdoutBuf := new(strings.Builder)

	s.Stdin = strings.NewReader(cmd)
	s.Stdout = io.MultiWriter(os.Stdout, stdoutBuf)
	s.Stderr = os.Stderr

	log.Printf("Running command with output: %s", cmd)
	err = s.Run("/bin/bash -")

	stdoutStr := stdoutBuf.String()
	if err != nil {
		return "", fmt.Errorf("runCmdOutput: failed to run command, %v", err)
	}

	return stdoutStr, err
}

// newOperationError is a helper in creating Operation_Error and marshals ErrorInfo.
func newOperationError(c codes.Code, msg, reason string) *status.Status {
	s := status.New(c, msg)
	s, err := s.WithDetails(&errdetails.ErrorInfo{
		Reason: reason,
	})
	if err != nil {
		panic("Failed to set status details")
	}
	return s
}

func pathExists(c *ssh.Client, path string) (bool, error) {
	exists, err := runCmdOutput(c, fmt.Sprintf("[ -e %s ] && echo -n 1 || echo -n 0", path))
	if err != nil {
		return false, fmt.Errorf("path exists: failed to check if %s exists, %s", path, err)
	}
	return exists == "1", nil
}

// stopSystemDaemon stops system daemons than can interfere with provisioning.
func stopSystemDaemons(c *ssh.Client) {
	if err := runCmd(c, "stop ui"); err != nil {
		log.Printf("Stop system daemon: failed to stop UI daemon, %s", err)
	}
	if err := runCmd(c, "stop update-engine"); err != nil {
		log.Printf("Stop system daemon: failed to stop update-engine daemon, %s", err)
	}
}

func clearTPM(c *ssh.Client) error {
	return runCmd(c, "crossystem clear_tpm_owner_request=1")
}

func wait(ctx context.Context, c *ssh.Client) error {
	// Wait so following commands don't run before an actual reboot has kicked off
	// by waiting for the client connection to shutdown or a timeout.
	wait := make(chan interface{})
	go func() {
		_ = c.Wait()
		close(wait)
	}()
	select {
	case <-wait:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("rebootDUT: timeout waiting waiting for reboot")
	}
}

func keepalive(ctx context.Context, c *ssh.Client) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_, _, err := c.SendRequest("keepalive@openssh.org", true, nil)
			log.Printf("keepalive connection alive=%v", err == nil)
		case <-ctx.Done():
			return
		}
	}
}

func rebootDUT(ctx context.Context, c *ssh.Client) error {
	// Reboot, ignoring the SSH disconnection.

	// On cloudbots, reboot never returned even after the DUT has been rebooted.
	// It does return by checking client connection periodically.
	if env.IsCloudBot() {
		log.Printf("Running hard reboot on cloudbots")
		go keepalive(ctx, c)
	}
	_ = runCmd(c, "reboot")
	return wait(ctx, c)
}

func hardRebootDUT(ctx context.Context, c *ssh.Client) error {
	// Hard reboot, ignoring the SSH disconnection.

	// On cloudbots, reboot never returned even after the DUT has been rebooted.
	// It does return by checking client connection periodically.
	if env.IsCloudBot() {
		log.Printf("Running hard reboot on cloudbots")
		go keepalive(ctx, c)
	}
	_ = runCmd(c, "/bin/echo \"b\" > /proc/sysrq-trigger")
	return wait(ctx, c)
}

func runLabMachineAutoReboot(c *ssh.Client) {
	const (
		labMachineFile = statefulPath + "/.labmachine"
	)
	err := runCmd(c, fmt.Sprintf("FILE=%s ; [ -f $FILE ] || ( touch $FILE ; start autoreboot )", labMachineFile))
	if err != nil {
		log.Printf("run lab machine autoreboot: failed to run autoreboot, %s", err)
	}
}

var reBuilderPath = regexp.MustCompile(`CHROMEOS_RELEASE_BUILDER_PATH=(.*)`)

func getBuilderPath(c *ssh.Client) (string, error) {
	return readLsbRelease(c, reBuilderPath)
}

var reBoard = regexp.MustCompile(`CHROMEOS_RELEASE_BOARD=(.*)`)

func getBoard(c *ssh.Client) (string, error) {
	return readLsbRelease(c, reBoard)
}

func readLsbRelease(c *ssh.Client, r *regexp.Regexp) (string, error) {
	lsbRelease, err := runCmdOutput(c, "cat /etc/lsb-release")
	if err != nil {
		return "", fmt.Errorf("read lsb release: failed to read lsb-release")
	}

	match := r.FindStringSubmatch(lsbRelease)
	if match == nil {
		return "", fmt.Errorf("read lsb release: no match found in lsb-release for %s", r.String())
	}
	return match[1], nil
}

func isStatefulCorrupt(c *ssh.Client) bool {
	return fileExists(c, "/mnt/stateful_partition/.corrupt_stateful")
}

func mismatchStatefulCheck(c *ssh.Client) bool {
	// Check the preloaded DLC mismatch.
	// NOTE: Verify one for now, if flakes in lab still occur, expand to entire supported DLC list.
	if err := runCmd(c, "which dlcservice_util"); err != nil {
		if _, err = runCmdOutput(c, "dlcservice_util --install --id=sample-dlc"); err != nil {
			log.Printf("mismatch stateful check: stateful resident preloaded DLC corrupt, %s", err)
			return true
		}
	}
	return false
}

func shouldForceProvision(c *ssh.Client) bool {
	return fileExists(c, "/mnt/stateful_partition/.force_provision")
}

func isRootfsVerificationOn(c *ssh.Client) (bool, error) {
	cmdline, err := runCmdOutput(c, "cat /proc/cmdline")
	if err != nil {
		return false, fmt.Errorf("is rootfs verification on: failed to read kernel cmdline")
	}
	return strings.Contains(cmdline, "dm_verity.dev_wait=1"), nil
}

// fileExists finds the file on the DUT, failures are treated as the file missing.
// Note: path must be escaped.
func fileExists(c *ssh.Client, path string) bool {
	exists, err := runCmdOutput(c,
		fmt.Sprintf("if [ -f %s ]; then echo found; fi", path))
	// Treat failure as file missing.
	return err != nil || exists != ""
}

func getBootID(c *ssh.Client) (string, error) {
	return runCmdOutput(c,
		fmt.Sprintf(
			"if [ -f '%[1]s' ]; then cat '%[1]s'; else echo 'no boot_id available'; fi",
			"/proc/sys/kernel/random/boot_id"))
}

func checkKvmEnabled(c *ssh.Client) (bool, error) {
	return pathExists(c, "/dev/kvm")
}

func checkIfLabstationDevice(c *ssh.Client) bool {
	board, err := getBoard(c)
	// Treat failure as non-labstation device.
	return err == nil && strings.Contains(board, "labstation")
}
