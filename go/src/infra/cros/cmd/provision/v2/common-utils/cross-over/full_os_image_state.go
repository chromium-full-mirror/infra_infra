// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// DLC constants and helpers
package cross_over

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	common_utils "go.chromium.org/chromiumos/test/provision/v2/common-utils"
	"golang.org/x/crypto/ssh"

	"go.chromium.org/chromiumos/config/go/test/api"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	// sshMsgIgnore is the SSH global message sent to ping the host.
	// See RFC 4253 11.2, "Ignored Data Message".
	sshMsgIgnore    = "SSH_MSG_IGNORE"
	pingTimeout     = time.Second
	pingRetryDelay  = 2 * time.Second
	pingRetryCount  = 20
	networkWaitTime = 1 * time.Minute
	powerResetDelay = 10 * time.Second
	networkErrMsg   = "Network is unreachable"
	cpuUartCapture  = "cpu_uart_capture"
	engImg          = "-eng"
	androidBuild    = "android-build"
)

var launchTargetPatterns = []*regexp.Regexp{
	regexp.MustCompile(`build_details/P?[0-9]+/([a-z_\-]+)`),
	regexp.MustCompile(`artifacts_list/P?[0-9]+/([a-z_\-]+)`)}

var buildIdPatterns = []*regexp.Regexp{
	regexp.MustCompile(`build_details/(P?[0-9]+)/`),
	regexp.MustCompile(`artifacts_list/(P?[0-9]+)/`)}

// FullOSImageState copies the full Android/Cros image onto DUT disk.
type FullOSImageState struct {
	params *CrossOverParameters
}

func NewFullOSImageState(params *CrossOverParameters) common_utils.ServiceState {
	return &FullOSImageState{
		params: params,
	}
}

func (s FullOSImageState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	log.Println("Executing " + s.Name())
	dutAddress := fmt.Sprintf("%s:%v", s.params.Dut.GetChromeos().GetSsh().GetAddress(), s.params.Dut.GetChromeos().GetSsh().GetPort())
	var client *ssh.Client
	if client = checkSSH(log, dutAddress, bootWaitRetryCount, bootWaitRetryInterval); client == nil {
		setPDRole(ctx, log, "src", s.params)
		errMsg := "cannot SSH onto DUT booted from USB common provision image"
		return common_utils.WrapStringInAny(s.errStatus(errMsg)), api.InstallResponse_STATUS_PRE_PROVISION_SETUP_FAILED, errors.New(errMsg)
	}
	defer client.Close()
	setPDRole(ctx, log, "src", s.params)

	cacheServer := s.params.Dut.GetCacheServer().GetAddress()
	ipFunc := func() {
		log.Println("IP info")
		var session *ssh.Session
		session, err := client.NewSession()
		if err != nil {
			log.Println("Failed to create session: ", err)
			return
		}

		// Jumble the command, we don't care what fails or not, any output, simply print it.
		const ipCmd = "set -e; ip rule show; ip route show table all; route; ifconfig eth0; echo 'All commands succeeded'"
		if output, err := session.CombinedOutput(ipCmd); err != nil {
			log.Println("Warning: failed to run IP info commands: ", ipCmd, err)
			log.Println(string(output))
		} else {
			log.Println(string(output))
		}
	}
	networkWaitFunc := func() error {
		shorterCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		networkCheckCmd := fmt.Sprintf("curl --max-time 5 -s -o/dev/null http://%s:%v/check_health", cacheServer.GetAddress(), cacheServer.GetPort())
		for {
			var session *ssh.Session
			session, err := client.NewSession()
			if err != nil {
				log.Println("Failed to create session: ", err)
				return err
			}

			select {
			case <-shorterCtx.Done():
				return errors.New("Failed while waiting for network connection.")
			default:
				if err := session.Run(networkCheckCmd); err != nil {
					ipFunc()
					// Retry w/ delay.
					time.Sleep(time.Second)
					continue
				}
				return nil
			}
		}
	}
	if err := networkWaitFunc(); err != nil {
		log.Println(err)
	}

	installCommand := ""
	targetImgPath := s.params.TargetImagePath.GetPath()
	if strings.Contains(targetImgPath, androidBuild) {
		fwBuildPath := ""
		if partnerGCSBucket := s.params.PartnerMetadata.GetPartnerGcsBucket(); partnerGCSBucket != "" {
			// Set the Firmware blob path for the given model (b/379953281)
			fwBuildPath = fmt.Sprintf("%s/provision_images/al-fw-blobs", partnerGCSBucket)
		}
		buildId, err := targetBuild(targetImgPath)
		if err != nil {
			return common_utils.WrapStringInAny(s.errStatus("INFRA: unable to parse image path from target")), api.InstallResponse_STATUS_INVALID_REQUEST, err
		}
		launchTarget, err := extractTargetLaunch(targetImgPath)
		if err != nil {
			return common_utils.WrapStringInAny(s.errStatus("INFRA: unable to parse image path from target")), api.InstallResponse_STATUS_INVALID_REQUEST, err
		}
		if strings.Contains(launchTarget, engImg) {
			// Uart logs are only enabled in -eng img.
			callServodSET(ctx, log, cpuUartCapture, on, s.params)
		}
		installCommand = fmt.Sprintf("al-install android-build/builds/%s/%s/attempts/latest/artifacts/android-desktop_image.bin.gz %s %s", buildId, launchTarget, cacheServer.GetAddress(), fwBuildPath)
	} else {
		installCommand = fmt.Sprintf("cros-install %s %s", targetImgPath[5:], cacheServer.GetAddress())
	}
	if err := installCommandWithRetry(client, installCommand, log); err != nil {
		log.Println("Failed to run installCommand: ", installCommand, err)
		collectUARTLogs(ctx, log, s.params)
		return common_utils.WrapStringInAny(s.errStatus("FAILED TO RUN CROSOVER INSTALL COMMAND")), api.InstallResponse_STATUS_DOWNLOADING_IMAGE_FAILED, err
	}
	errStatus, responseStatus, err := s.handleReboot(ctx, client, log)
	if err != nil {
		collectUARTLogs(ctx, log, s.params)
	}
	return errStatus, responseStatus, err
}

func (s FullOSImageState) errStatus(curErr string) string {
	if s.params.PrevError != "" {
		return fmt.Sprintf("Flash and OTA failed: %s", s.params.PrevError)
	}
	return curErr
}

func (s FullOSImageState) Next() common_utils.ServiceState {
	return NewValidateState(s.params)
}

func (s FullOSImageState) Name() string {
	return "Full OS Image State"
}

func (s FullOSImageState) handleReboot(ctx context.Context, sshClient *ssh.Client, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	if err := callServodRetry(ctx, log, "power_state", "off", s.params); err != nil {
		return common_utils.WrapStringInAny("INFRA: unable to set turn off power via servo"), api.InstallResponse_STATUS_PROVISIONING_FAILED, err
	}
	if err := sshFailWait(sshClient, log); err != nil {
		return common_utils.WrapStringInAny("INFRA: dut did not power off post provision image installation"), api.InstallResponse_STATUS_PROVISIONING_FAILED, err
	}
	log.Println("Device successfully powered off.")
	if err := callServodRetry(ctx, log, "image_usbkey_direction", "servo_sees_usbkey", s.params); err != nil {
		return common_utils.WrapStringInAny("INFRA: unable to set usb direction via servo"), api.InstallResponse_STATUS_PROVISIONING_FAILED, err
	}
	log.Printf("\nWaiting for the image_usbkey_direction.")
	// TODO: Instead of fixed wait time, use a poll based status checker. Refer WaitForPowerStates and GetECSystemPowerState in firmware code.
	time.Sleep(10 * time.Second)

	if err := callServodRetry(ctx, log, "power_state", "on", s.params); err != nil {
		// reset might be needed if power on failed: b/381982169
		log.Println("Warning: unable to set turn on power via servo ", err)
		log.Println("Waiting for 10 seconds before trying power resetting.")
		time.Sleep(powerResetDelay)
		if err := callServodRetry(ctx, log, "power_state", "reset", s.params); err != nil {
			return common_utils.WrapStringInAny("INFRA: unable to reset power via servo"), api.InstallResponse_STATUS_PROVISIONING_FAILED, err
		}
	}
	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

func installCommandWithRetry(client *ssh.Client, installCommand string, log *log.Logger) error {
	log.Println("Running command on host now ", installCommand)
	retryCount := 3
	for ; retryCount >= 0; retryCount-- {
		var session *ssh.Session
		session, err := client.NewSession()
		if err != nil {
			log.Println("Failed to create session: ", err)
			return err
		}
		output, err := session.CombinedOutput(installCommand)
		log.Println(string(output))
		if err == nil {
			return nil
		}
		if !strings.Contains(string(output), networkErrMsg) {
			return err
		}
		log.Println("Got error " + networkErrMsg + " while running install command. Will Retry...")
	}
	return fmt.Errorf("Failed to run installCommand")
}

// sshFailWait return nil, if the device cannot be sshed within 40 seconds.
// If the device is alive even after 40 seconds, it returns error.
func sshFailWait(sshClient *ssh.Client, log *log.Logger) error {
	for attemp := 1; attemp < pingRetryCount; attemp++ {
		log.Println("pinging device to check if its offline...")
		if err := ping(sshClient, pingTimeout); err != nil {
			return nil
		}
		time.Sleep(pingRetryDelay)
	}
	return errors.New("dut failed to become unreachable")
}

func ping(sshClient *ssh.Client, timeout time.Duration) error {
	ch := make(chan error, 1)
	go func() {
		_, _, err := sshClient.SendRequest(sshMsgIgnore, true, []byte{})
		ch <- err
	}()

	select {
	case err := <-ch:
		return err
	case <-time.After(timeout):
		return errors.New("timed out")
	}
}

func extractTargetLaunch(path string) (string, error) {
	for _, re := range launchTargetPatterns {
		matches := re.FindStringSubmatch(path)
		if len(matches) == 2 {
			return matches[1], nil
		}
	}
	return "", fmt.Errorf("could not extract launch from %s", path)
}

func targetBuild(imagePath string) (string, error) {
	for _, re := range buildIdPatterns {
		matches := re.FindStringSubmatch(imagePath)
		if len(matches) == 2 {
			return matches[1], nil
		}
	}
	return "", fmt.Errorf("could not extract buildId from %s", imagePath)
}
