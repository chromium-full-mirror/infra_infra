// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"go.chromium.org/chromiumos/test/provision/v2/foil-provision/service"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/chromiumos/test/util/adb"
)

var launchTargetPatterns = []*regexp.Regexp{
	regexp.MustCompile(`build_details/P?[0-9]+/([a-z_\-]+)`),
	regexp.MustCompile(`artifacts_list/P?[0-9]+/([a-z_\-]+)`)}

type Install struct {
	ctx            context.Context
	cs             *service.FoilService
	RebootRequired bool
	helperStatus   string
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
		return fmt.Errorf("Unable to pull image from cache %v", err)
	}
	defer os.Remove(localImagePath)
	log.Println("Image pulled from cache and stored at ", localImagePath)
	status, installErr := install(log, c.cs.UpdateEnginePid, c.cs.DutIp, localImagePath)

	if installErr != nil {
		c.helperStatus = fmt.Sprintf("OTA EXIT STATUS: %s", status)
		return fmt.Errorf("Unable to install over 1 atempts.")
	}
	return nil
}

// function to extract the launch from the android build path.
// Example it will return brya-trunk_staging-userdebug from
// android-build/build_explorer/build_details/P78687640/brya-trunk_staging-userdebug/android-desktop-ota-packages.zip
// android-build/build_explorer/artifacts_list/12330924/brya-trunk_staging-userdebug/brya-ota-12330924.zip
func extractTargetLaunch(path string) (string, error) {
	for _, re := range launchTargetPatterns {
		matches := re.FindStringSubmatch(path)
		if len(matches) == 2 {
			return matches[1], nil
		}
	}
	return "", fmt.Errorf("could not extract launch from %s", path)
}

// pullFromCache downloads the imge from cache server on cft container and
// returns the local path.
func (c *Install) pullFromCache(log *log.Logger, path string) (string, error) {
	targetLaunch, err := extractTargetLaunch(path)
	if err != nil {
		return "", err
	}
	var client http.Client
	cacheUrl := c.cs.CacheServerUrl
	cacheUrl.Path = filepath.Join("download", "android-build", "builds", c.cs.TargetBuild, targetLaunch, "attempts", "latest", "artifacts", filepath.Base(path))
	log.Println("pulling from cache url : ", cacheUrl.String())
	resp, err := client.Get(cacheUrl.String())
	if err != nil {
		c.helperStatus = fmt.Sprintf("failed to pull from cache: %s", err)
		return "", fmt.Errorf("FLEET: failed to pull from cache server %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.helperStatus = fmt.Sprintf("FLEET: error code while download: %v", resp.StatusCode)
		return "", fmt.Errorf("error %s to download %q", resp.Status, cacheUrl.String())
	}
	out, err := os.CreateTemp(os.TempDir(), "image-*")
	if err != nil {
		c.helperStatus = "INFRA: unable to make tempfile on host"
		return "", fmt.Errorf("failed to create a temp file %v", err)
	}
	log.Printf("created tmp file %v to store the pulled file", out.Name())

	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		c.helperStatus = "INFRA: unable to copy cache to tempfile"
		return "", fmt.Errorf("failed to copy request data to temp file %v", err)
	}
	return out.Name(), nil
}

func (c *Install) Revert() error {
	return nil
}

func (c *Install) GetErrorMessage() string {
	return c.helperStatus
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
	outStr, _ := adb.AdbCmd([]string{"-s", addr, "logcat", "-d", "--pid", pid}, log)

	log.Println("Finished co")
	log.Println("logcat out", outStr)

	return
}

// Helper function to read output from a pipe
func readOutput(r io.Reader, ch chan string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		ch <- scanner.Text()
	}
	close(ch)
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
		return "", fmt.Errorf("failed to run Cmd: %v", err)
	}
	var wg sync.WaitGroup
	wg.Add(2)

	const maxCapacity = 4096 * 1024
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

	if found1 == true || found2 == true {
		log.Println("Found success status!")
		return "", nil
	}

	return status, fmt.Errorf("unable to find successful status.")
}
