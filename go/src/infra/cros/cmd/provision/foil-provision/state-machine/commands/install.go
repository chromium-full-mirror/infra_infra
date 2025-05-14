// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/cft/common/adb"
	"go.chromium.org/infra/cros/cmd/provision/common-utils/cache"
	"go.chromium.org/infra/cros/cmd/provision/foil-provision/service"
)

var lunchTargetPatterns = []*regexp.Regexp{
	regexp.MustCompile(`build_details/P?[0-9]+/([a-z_\-]+)`),
	regexp.MustCompile(`artifacts_list/P?[0-9]+/([a-z_\-]+)`)}

type Install struct {
	ctx            context.Context
	cs             *service.FoilService
	RebootRequired bool
	errorReason    string
}

func NewInstall(ctx context.Context, cs *service.FoilService) *Install {
	return &Install{
		ctx: ctx,
		cs:  cs,
	}
}

func (c *Install) Execute(log *log.Logger) error {

	log.Printf("Start Install Execute, with reboot changed")
	localImagePath, err := c.pullFromCache(log, c.cs.ImagePath.GetPath())
	if err != nil {
		return fmt.Errorf("Unable to pull image from cache %w", err)
	}
	defer os.Remove(localImagePath)
	log.Println("Image pulled from cache and stored at ", localImagePath)
	status, installErr := install(log, c.cs.UpdateEnginePid, c.cs.DutIp, localImagePath)

	if installErr != nil {
		c.errorReason = fmt.Sprintf("OTA EXIT STATUS: %s", status)
		return fmt.Errorf("unable to install over 1 attempt (exit status=%s): %w", status, installErr)
	}
	return nil
}

// extractLunchTarget extracts the lunch target from the Android Build artifact path.
// For example, it will return "brya-trunk_staging-userdebug" from any of the following:
// android-build/build_explorer/build_details/P78687640/brya-trunk_staging-userdebug/android-desktop-ota-packages.zip
// android-build/build_explorer/artifacts_list/12330924/brya-trunk_staging-userdebug/brya-ota-12330924.zip
func extractLunchTarget(path string) (string, error) {
	for _, re := range lunchTargetPatterns {
		matches := re.FindStringSubmatch(path)
		if len(matches) == 2 {
			return matches[1], nil
		}
	}
	return "", fmt.Errorf("could not extract lunch target from %s", path)
}

// pullFromCache downloads an image from the cache server onto the cft container,
// and returns the local path to the downloaded image.
func (c *Install) pullFromCache(log *log.Logger, path string) (string, error) {
	lunchTarget, err := extractLunchTarget(path)
	if err != nil {
		return "", err
	}
	cacheClient, err := cache.NewClient(c.cs.CacheServerURL)
	if err != nil {
		return "", err
	}
	localPath, errorReason, err := cacheClient.DownloadABArtifact(c.cs.TargetBuild, lunchTarget, filepath.Base(path))
	c.errorReason = errorReason
	return localPath, err
}

func (c *Install) Revert() error {
	return nil
}

func (c *Install) GetErrorMessage() string {
	return c.errorReason
}

func (c *Install) GetStatus() api.InstallResponse_Status {
	return api.InstallResponse_STATUS_PROVISIONING_FAILED
}

func TestScanner(stream io.Reader, logger *log.Logger, harness string) (bool, string) {
	const maxCapacity = 4096 * 1024
	scanner := bufio.NewScanner(stream)
	completion := false
	errStr := "OTA completed"
	// Expand the buffer size to avoid deadlocks on heavy logs
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {

		txt := scanner.Text()
		logger.Printf("[%v] %v", harness, txt)
		if strings.Contains(txt, "onPayloadApplicationComplete(ErrorCode::kSuccess") {
			logger.Println("found status in: ", txt)
			completion = true
		} else if strings.Contains(txt, "onPayloadApplicationComplete(ErrorCode::") {
			logger.Println("found status in2: ", txt)
			completion = false
			re := regexp.MustCompile(`ErrorCode::(.*?) `)
			// Find the matching substring
			match := re.FindStringSubmatch(txt)
			if len(match) > 1 {
				errStr = match[1]
			} else {
				errStr = "unknown error during OTA"
			}
		}
	}
	if scanner.Err() != nil {
		logger.Println("Failed to read pipe: ", scanner.Err())
	}
	logger.Println("Final completion status: ", completion)
	return completion, errStr
}

func logcat(log *log.Logger, pid string, addr string) {
	outStr, _ := adb.AdbCmd([]string{"-s", addr, "logcat", "-d", "--pid", pid}, log, adb.DefaultRetryAttempts, adb.DefaultCommandSeconds)

	log.Println("Finished co")
	log.Println("logcat out", outStr)
}

func install(log *log.Logger, lcpid string, addr, localImagePath string) (string, error) {
	log.Printf("Install start")
	// logcmd, logchan, err := logcat()

	cmd := exec.Command("python3", []string{"/usr/local/update_device.py", localImagePath, "-s", adb.FmtAddr(addr)}...)

	log.Println("Running ADB OTA: ", cmd.String())

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("StderrPipe failed")
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("StdoutPipe failed")
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to run Cmd: %w", err)
	}
	var wg sync.WaitGroup
	wg.Add(2)

	found1 := false
	found2 := false
	status := ""
	go func() {
		defer wg.Done()
		found1, status = TestScanner(stderr, log, "foil-prov:")
	}()

	go func() {
		defer wg.Done()
		found2, status = TestScanner(stdout, log, "foil-prov")
	}()
	wg.Wait()

	if err := cmd.Wait(); err != nil {
		log.Println("Failed to run ADB UPDATE: ", err)
	}
	log.Printf("DONE. Checking completion status")

	logcat(log, lcpid, addr)
	log.Println("LOGCAT CLOSED!")

	if found1 || found2 {
		log.Println("Found success status!")
		return "", nil
	}

	return status, fmt.Errorf("unable to find successful status.")
}

func (c *Install) Retry() bool {
	return false
}
