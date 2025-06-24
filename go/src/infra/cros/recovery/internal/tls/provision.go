// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tls provides the canonical implementation of a common TLS server.
package tls

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

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/internal/env"
	access "go.chromium.org/infra/cros/recovery/internal/localtlw/ssh"
	tlw_server "go.chromium.org/infra/cros/recovery/internal/tlw"
	"go.chromium.org/infra/cros/recovery/tlw"
)

const (
	verificationTimeout = 5 * time.Minute
	rebootTimeout       = 10 * time.Minute
)

type runner struct {
	hostname string
	provider access.SSHProvider
}

func (r *runner) Run(ctx context.Context, timeout time.Duration, cmd string) (out string, err error) {
	newCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	res := access.Run(newCtx, r.provider, r.hostname, cmd)
	if res.GetExitCode() == 0 {
		return res.GetStdout(), nil
	}
	return "", errors.Reason("run on %q: failed %s", r.hostname, res.GetStderr()).Err()
}

func (r *runner) GetClient(ctx context.Context) (*ssh.Client, error) {
	c, err := r.provider.Get(ctx, r.hostname)
	if err != nil {
		return nil, errors.Annotate(err, "get client").Err()
	}
	return c.Client(), nil
}

func provision(ctx context.Context, tlw tlw_server.Server, run *runner, req *tlw.ProvisionRequest) (rErr error) {
	log.Printf("provision: started on hostname=%v", req.GetResource())

	// Set a timeout for provisioning.
	ctx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()

	startTime := time.Now()
	defer func() {
		log.Printf("provision: finished on hostname=%v", req.GetResource())
		log.Printf("provision: time to provision took %v", time.Since(startTime))
	}()

	var p *provisionState

	log.Printf("provision: Creating provision state.")
	p, err := newProvisionState(req, run, tlw)
	if err != nil {
		return errors.Annotate(err, "provision").Err()
	}

	if err := p.connect(ctx); err != nil {
		return errors.Annotate(err, "provision: connect provision state").Err()
	}

	log.Printf("provision: Verify that the DUT is reachable.")
	if _, err := run.Run(ctx, 120*time.Second, "true"); err != nil {
		return errors.Annotate(err, "provision: DUT un-reachable prior to provisioning").Err()
	}

	log.Printf("provision: Validate RVM on device.")
	if !checkIfLabstationDevice(p.c) {
		// Check if the DUT has KVM enabled.
		kvmEnabled, err := checkKvmEnabled(p.c)
		if err != nil {
			return errors.Annotate(err, "provision: failed to check if KVM enabled on this device").Err()
		}
		if !kvmEnabled {
			return errors.Annotate(err, "provision: KVM is not enabled on this device, provisioning it again will not help (repair needs to cold reboot this device)").Err()
		}
	}

	// Provision the OS.
	select {
	case <-ctx.Done():
		return errors.Fmt("provision: timed out before provisioning OS: %w", ctx.Err())
	default:
	}

	t := time.Now()
	if err := p.provisionOS(ctx); err != nil {
		return errors.Annotate(err, "provision: failed to provision OS").Err()
	}

	// Wait for DUT to come up after provisioning the OS.
	// Can continue as soon as a connection can be established.
	// Give extra time in case of firmware updates prior to UI spawning.
	rebootWaitCtx, cancel := context.WithTimeout(ctx, rebootTimeout)
	defer cancel()
	if err = p.connect(rebootWaitCtx); err != nil {
		return errors.Annotate(err, "provision: failed to wait for DUT to come up after provisioning the OS").Err()
	}
	log.Printf("DUT came up after provisioning the OS.")

	// Should shorten the time waiting for reboot to complete booting into the new OS.
	// Certain images will take significantly longer due to debug features being enabled.
	// Follow through subsequent reboots until "ui" job is running.
	uiStabilizeCtx, cancel := context.WithTimeout(ctx, rebootTimeout)
	defer cancel()
	if err := p.waitForUIToStabilize(uiStabilizeCtx); err != nil {
		return errors.Annotate(err, "provision: failed to wait for UI to stabilize").Err()
	}

	log.Printf("provision: time to provision OS took %v", time.Since(t))

	// To be safe, wait for kernel to be "sticky" right after installing the new partitions and booting into it.
	t = time.Now()
	// Timeout is determined by 2x delay to mark new kernel successful + 10 seconds fuzz.
	stickyKernelCtx, cancel := context.WithTimeout(ctx, 100*time.Second)
	defer cancel()
	if err := p.verifyKernelState(stickyKernelCtx); err != nil {
		return errors.Annotate(err, "provision: failed to wait for sticky kernel").Err()
	}
	log.Printf("provision: time to wait for sticky kernel %v", time.Since(t))

	t = time.Now()
	if err := p.provisionStateful(ctx); err != nil {
		return errors.Annotate(err, "provision: failed to provision stateful").Err()
	}
	log.Printf("provision: time to provision stateful took %v", time.Since(t))

	// After a reboot, need a new client connection.
	sshCtx, cancel := context.WithTimeout(ctx, rebootTimeout)
	defer cancel()
	if err := p.connect(sshCtx); err != nil {
		return errors.Annotate(err, "provision: failed to connect to DUT after stateful update and reboot").Err()
	}

	if !req.PreventReboot {
		t = time.Now()
		verifyCtx, cancel := context.WithTimeout(ctx, verificationTimeout)
		defer cancel()
		if err := p.verifyOSProvision(verifyCtx); err != nil {
			return errors.Annotate(err, "provision: failed to verify OS provision").Err()
		}
		log.Printf("provision: time to verify provision took %v", time.Since(t))
	}

	// Provision miniOS.
	select {
	case <-ctx.Done():
		return errors.Annotate(err, "provision: timed out before provisioning DLCs").Err()
	default:
	}

	if err := p.provisionMiniOS(ctx); err != nil {
		// Initially failing to provision miniOS partitions isn't a failure.
		log.Printf("provision: failed to provision miniOS partitions, check partition table, %s", err)
	}

	if bootID, err := getBootID(p.c); err != nil {
		log.Printf("provision: Warning, failed to get boot ID")
	} else {
		log.Printf("provision: boot ID is %s", bootID)
	}

	// Finish provisioning.
	return nil
}

// runCmd interprets the given string command in a shell and returns the error if any.
func runCmd(c *ssh.Client, cmd string) error {
	s, err := c.NewSession()
	if err != nil {
		return fmt.Errorf("runCmd: failed to create session, %w", err)
	}
	defer s.Close()

	s.Stdin = strings.NewReader(cmd)
	s.Stdout = os.Stdout
	s.Stderr = os.Stderr

	log.Printf("Running command: %s", cmd)
	// Always run commands under /bin/bash.
	err = s.Run("/bin/bash -")
	return errors.Annotate(err, "runCmd: failed to run command").Err()
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
			err = fmt.Errorf("%w, %w", err, retryErr)
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
		return "", fmt.Errorf("runCmdOutput: failed to run command, %w", err)
	}

	return stdoutStr, err
}

func pathExists(c *ssh.Client, path string) (bool, error) {
	exists, err := runCmdOutput(c, fmt.Sprintf("[ -e %s ] && echo -n 1 || echo -n 0", path))
	if err != nil {
		return false, fmt.Errorf("path exists: failed to check if %s exists, %w", path, err)
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
	wait := make(chan any)
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
