// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build !windows

// Package ccd contains functionality around management of the GSC console.
package ccd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"

	"infra/cros/satlab/common/utils/executor"
	"infra/cros/satlab/common/utils/expect"
	"infra/cros/satlab/common/utils/misc"
)

// CCDOpenCmd contains the usage and implementation for the ccd open command
var CCDOpenCmd = &subcommands.Command{
	UsageLine: "open <args>",
	ShortDesc: `Opens CCD`,
	LongDesc: `Opens CCD, enables testlab mode and optionally resets CCD to factory settings.
	Providing servo serial or USB device is optional if only one device is connected to the host.
	Servo V4, V4p1 and SuzyQ cable are supported. You can provide either a Servo serial number
	or a path to a Cr50/Ti50 serial console USB device (/dev/ttyUSBy).`,
	CommandRun: func() subcommands.CommandRun {
		c := &ccdOpenRun{}
		c.Flags.StringVar(&c.servoSerial, "servo-serial", "", "Servo serial number")
		c.Flags.StringVar(&c.usbDevice, "usb-device", "", "Cr50/Ti50 serial console USB device")
		c.Flags.BoolVar(&c.useRmaAuth, "rma-auth", false, "Use rma_auth to open CCD")
		c.Flags.BoolVar(&c.resetFactory, "reset-factory", false, "Reset CCD to factory settings")
		return c
	},
}

// ccdOpenRun struct contains the arguments needed to run CCDOpenCmd and common fields for the methods
type ccdOpenRun struct {
	subcommands.CommandRunBase
	servoSerial  string
	usbDevice    string
	useRmaAuth   bool
	resetFactory bool

	containerName   string
	dockerPath      string
	goexpectSession *expect.GExpect
}

const (
	iterationsLimit       = 50
	consoleConnIterations = 3
	timeout               = 5 * time.Second
)

// reCcd is a regular expression for the GSC console prompt
var reCcd = regexp.MustCompile(`> `)

// Run is what is called when a user inputs the ccd open command
func (c *ccdOpenRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

// innerRun handles all orchestration needed to pass appropriate clients, contexts and commands into the application
func (c *ccdOpenRun) innerRun(a subcommands.Application, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)
	err := c.TriggerRun(ctx, &executor.ExecCommander{})
	return err
}

// TriggerRun triggers the Run with the given information
func (c *ccdOpenRun) TriggerRun(
	ctx context.Context,
	executor executor.IExecCommander,
) error {
	cancelChannel := make(chan os.Signal, 1)
	// catch SIGETRM or SIGINTERRUPT
	signal.Notify(cancelChannel, syscall.SIGTERM, syscall.SIGINT)
	// Handle the signals.
	go func() {
		for {
			<-cancelChannel
			fmt.Println("Docker container cleanup...")
			if err := c.killDockerContainerForGSCSerial(ctx, executor); err != nil {
				fmt.Println(err)
			}
			fmt.Println("Exiting on an interrupt.")
			os.Exit(1)
		}
	}()
	if c.servoSerial != "" && c.usbDevice != "" {
		return errors.New("Provide only one of the parameters -servo-serial or -usb-device.")
	}
	if err := c.setDockerPath(); err != nil {
		return errors.Annotate(err, "set docker executable path").Err()
	}
	c.containerName = "gscserial"
	if c.servoSerial != "" {
		c.containerName = "gscserial-" + c.servoSerial
	}
	if err := c.startDockerContainerForGSCSerial(ctx, executor); err != nil {
		return errors.Annotate(err, "start docker container for gsc serial").Err()
	}
	defer func() {
		if err := c.killDockerContainerForGSCSerial(ctx, executor); err != nil {
			fmt.Println(err)
		}
	}()
	if err := c.prepareCCD(ctx, executor); err != nil {
		return errors.Annotate(err, "prepare ccd").Err()
	}
	return nil
}

func (c *ccdOpenRun) setDockerPath() error {
	var err error
	c.dockerPath, err = exec.LookPath("docker")
	if err != nil {
		return errors.Annotate(err, "find the docker executable").Err()
	}
	return nil
}

func dockerSerialImageName() string {
	label := misc.GetEnv("GSC_SERIAL_CONTAINER_LABEL", "release")
	registry := misc.GetEnv("REGISTRY_URI", "gcr.io/chromeos-partner-moblab")
	return fmt.Sprintf("%s/satlab-serial:%s", registry, label)
}

func (c *ccdOpenRun) startDockerContainerForGSCSerial(
	ctx context.Context,
	executor executor.IExecCommander,
) error {
	args := []string{
		c.dockerPath,
		"run",
		"-dit",
		"--name",
		c.containerName,
		"--privileged",
		"--rm",
		"-v",
		"/dev/serial:/dev/serial:ro",
		dockerSerialImageName(),
	}
	if _, err := executor.CombinedOutput(exec.CommandContext(ctx, args[0], args[1:]...)); err != nil {
		return errors.Annotate(err, "start docker container for gsc serial").Err()
	}
	return nil
}

func (c *ccdOpenRun) killDockerContainerForGSCSerial(
	ctx context.Context,
	executor executor.IExecCommander,
) error {
	args := []string{
		c.dockerPath,
		"kill",
		c.containerName,
	}
	if _, err := executor.CombinedOutput(exec.CommandContext(ctx, args[0], args[1:]...)); err != nil {
		return errors.Annotate(err, "kill docker container for gsc serial").Err()
	}
	return nil
}

func (c *ccdOpenRun) getSerialDevice(
	ctx context.Context,
	executor executor.IExecCommander,
) (string, error) {
	if c.usbDevice != "" {
		return c.usbDevice, nil
	}
	args := []string{
		c.dockerPath,
		"exec",
		c.containerName,
		"ls",
		"/dev/serial/by-id/",
	}
	out, err := executor.CombinedOutput(exec.CommandContext(ctx, args[0], args[1:]...))
	if err != nil {
		return "", errors.Annotate(err, "get serial devices by id").Err()
	}
	serialDevices := strings.TrimRight(string(out), "\n")
	var relatedUSBHubID string
	m := make(map[string]map[string]string)
	for _, line := range strings.Split(serialDevices, "\n") {
		if strings.Contains(line, "-if00-") {
			args = []string{
				c.dockerPath,
				"exec",
				c.containerName,
				"readlink",
				"-f",
				"/dev/serial/by-id/" + line,
			}
			out, err := executor.CombinedOutput(exec.CommandContext(ctx, args[0], args[1:]...))
			if err != nil {
				return "", errors.Annotate(err, "readlink").Err()
			}
			ttyUSB := strings.TrimRight(string(out), "\n\t")

			args = []string{
				c.dockerPath,
				"exec",
				c.containerName,
				"udevadm",
				"info",
				"-q",
				"path",
				"-n",
				ttyUSB,
			}
			out, err = executor.CombinedOutput(exec.CommandContext(ctx, args[0], args[1:]...))
			if err != nil {
				return "", errors.Annotate(err, "udevadm").Err()
			}
			reUdevadm := regexp.MustCompile(`usb\d+/(\d+-\d+)/`)
			match := reUdevadm.FindStringSubmatch(strings.TrimRight(string(out), "\n\t"))
			if match == nil {
				return "", errors.New("USB hub id does not match.")
			}
			usbHubID := match[1]

			if m[usbHubID] == nil {
				m[usbHubID] = make(map[string]string)
			}
			reServo := regexp.MustCompile(`Servo_V\w+_(.+)-if00-`)
			reGSC := regexp.MustCompile(`Cr50|Ti50`)
			if match := reServo.FindStringSubmatch(line); match != nil {
				m[usbHubID]["servo_serial"] = match[1]
				if c.servoSerial != "" && match[1] == c.servoSerial {
					relatedUSBHubID = usbHubID
				}
			} else if match = reGSC.FindStringSubmatch(line); match != nil {
				m[usbHubID]["gsc_serial_dev"] = ttyUSB
			}
		}
	}
	serialInfo := ""
	servoSerialMatch := false
	for k, v := range m {
		if relatedUSBHubID == "" && len(m) == 1 {
			relatedUSBHubID = k
		}
		if v["servo_serial"] == c.servoSerial {
			servoSerialMatch = true
		}
		serialInfo = serialInfo + fmt.Sprintf("USB hub %s: servo serial: %q, GSC serial device: %s\n", k, v["servo_serial"], v["gsc_serial_dev"])
	}
	if c.servoSerial != "" && !servoSerialMatch {
		return "", errors.New("Provided servo serial not found. Detected devices:\n" + serialInfo)
	}
	if relatedUSBHubID == "" && len(m) > 1 {
		return "", errors.New("More than one supported device detected. Please provide -servo-serial or -usb-device parameter:\n" + serialInfo)
	}
	if gscSerialDev, ok := m[relatedUSBHubID]["gsc_serial_dev"]; ok {
		return gscSerialDev, nil
	} else {
		return "", errors.New("No GSC serial device detected.")
	}
}

func (c *ccdOpenRun) prepareCCD(
	ctx context.Context,
	executor executor.IExecCommander,
) error {
	var err error
	c.goexpectSession, _, err = expect.Spawn("docker attach "+c.containerName, -1)
	if err != nil {
		return errors.Annotate(err, "spawn expect session").Err()
	}
	defer func() {
		if err := c.goexpectSession.Close(); err != nil {
			fmt.Println(err)
		}
	}()
	port, err := c.getSerialDevice(ctx, executor)
	if err != nil {
		return errors.Annotate(err, "get serial device").Err()
	}
	if err := c.prepareTerminal(port); err != nil {
		return errors.Annotate(err, "prepare terminal").Err()
	}
	ccdOpened, err := c.checkIfCCDOpened()
	if err != nil {
		return errors.Annotate(err, "check if ccd is opened").Err()
	}
	rmaAuthOpen := false
	if !ccdOpened {
		if c.useRmaAuth {
			if err := c.getRMAAuthChallenge(); err != nil {
				return errors.Annotate(err, "get rma_auth challenge").Err()
			}
			unlockCode, err := c.provideUnlockCode()
			if err != nil {
				return errors.Annotate(err, "check if ccd opened").Err()
			}
			err = c.openCCDWithRMAAuth(unlockCode)
			if err != nil {
				return errors.Annotate(err, "run rma_auth").Err()
			}
			rmaAuthOpen = true
		} else {
			err = c.runCCDOpen()
			if err != nil {
				return errors.Annotate(err, "run ccd open").Err()
			}
		}
	}
	testlabEnabled, err := c.checkIfTestlabEnabled()
	if err != nil {
		return errors.Annotate(err, "check if testlab mode is enabled").Err()
	}
	if !testlabEnabled {
		if err := c.runCCDTestlabEnable(); err != nil {
			return errors.Annotate(err, "enable testlab mode").Err()
		}
	}
	if c.resetFactory {
		if err := c.runCCDResetFactory(); err != nil {
			return errors.Annotate(err, "reset ccd to factory settings").Err()
		}
	}
	if rmaAuthOpen {
		// GSC disables the TPM after RMA open until the AP resets.
		// Sending ecrst pulse to reboot the AP.
		fmt.Println("Rebooting to enable TPM after RMA open...")
		if err := c.goexpectSession.Send("ecrst pulse\n"); err != nil {
			return errors.Annotate(err, "send ecrst pulse command").Err()
		}
		// Wait for the command execution
		time.Sleep(time.Second)
	}
	return nil
}

func tryConsole(f func() error) error {
	var err error
	for i := 0; i < consoleConnIterations; i++ {
		if err = f(); err == nil {
			return nil
		}
	}
	return err
}

func (c *ccdOpenRun) prepareTerminal(port string) error {
	if _, _, err := c.goexpectSession.Expect(regexp.MustCompile(`/ # `), timeout); err != nil {
		return errors.Annotate(err, "wait for sh prompt").Err()
	}
	if err := c.goexpectSession.Send("picocom " + port + "\n"); err != nil {
		return errors.Annotate(err, "send picocom command").Err()
	}
	checkTerminalReady := func() error {
		re := regexp.MustCompile(`Terminal ready`)
		_, _, err := c.goexpectSession.Expect(re, 1*time.Second)
		return err
	}
	checkGSCPrompt := func() error {
		if err := c.goexpectSession.Send("\n"); err != nil {
			return errors.Annotate(err, "send a new line").Err()
		}
		_, _, err := c.goexpectSession.Expect(reCcd, 1*time.Second)
		return err
	}
	if err := tryConsole(checkTerminalReady); err == nil {
		return errors.Annotate(err, "wait for terminal ready").Err()
	}
	if err := tryConsole(checkGSCPrompt); err == nil {
		return errors.Annotate(err, "wait for gsc serial prompt").Err()
	}
	return nil
}

func (c *ccdOpenRun) checkIfCCDOpened() (bool, error) {
	if err := c.goexpectSession.Send("ccd\n"); err != nil {
		return false, errors.Annotate(err, "send ccd command").Err()
	}
	reCcdState := regexp.MustCompile(`State: (Opened|Locked|Unlocked)`)
	_, match, err := c.goexpectSession.Expect(reCcdState, timeout)
	if err != nil {
		return false, errors.Annotate(err, "expect output from ccd command").Err()
	}
	fmt.Println("CCD state: " + match[1])
	if match[1] == "Opened" {
		return true, nil
	}
	return false, nil
}

func (c *ccdOpenRun) getRMAAuthChallenge() error {
	if err := c.goexpectSession.Send("rma_auth\n"); err != nil {
		return errors.Annotate(err, "send rma_auth command").Err()
	}
	reChallenge := regexp.MustCompile(`generated challenge:\s+([\s0-9A-Z]{40,})\s`)
	_, match, err := c.goexpectSession.Expect(reChallenge, timeout)
	if err != nil {
		return errors.Annotate(err, "expect output from rma_auth command").Err()
	}
	challenge := strings.ReplaceAll(match[1], " ", "")
	fmt.Println("Follow this link to generate unlock code:")
	url := "https://www.google.com/chromeos/partner/console/cr50reset?challenge=" + challenge
	fmt.Println(url)
	return nil
}

func (c *ccdOpenRun) provideUnlockCode() (string, error) {
	var unlockCode string
	fmt.Print("Enter the unlock code: ")
	fmt.Scanln(&unlockCode)
	if !regexp.MustCompile(`[A-Z0-9]{8}`).MatchString(unlockCode) {
		return "", errors.New("Invalid unlock code.")
	}
	return unlockCode, nil
}

func (c *ccdOpenRun) openCCDWithRMAAuth(unlockCode string) error {
	if err := c.goexpectSession.Send("rma_auth " + unlockCode + "\n"); err != nil {
		return errors.Annotate(err, "send rma_auth command with unlock code").Err()
	}
	if _, _, err := c.goexpectSession.Expect(reCcd, timeout); err != nil {
		return errors.Annotate(err, "expect output from rma_auth command").Err()
	}
	// Time for GSC to reboot
	time.Sleep(1 * time.Second)
	ccdOpened, err := c.checkIfCCDOpened()
	if err != nil {
		return errors.Annotate(err, "check if ccd is opened").Err()
	}
	if !ccdOpened {
		if err := c.runCCDOpen(); err != nil {
			return errors.Annotate(err, "run ccd open").Err()
		}
	}
	return nil
}

func powerButtonAlert() {
	fmt.Print("You may be asked to press the physical power button multiple times. Follow the instructions.")
	for i := 0; i < 4; i++ {
		fmt.Print(".")
		time.Sleep(time.Second)
	}
	fmt.Print("\n")
}

func (c *ccdOpenRun) runCCDOpen() error {
	powerButtonAlert()
	if err := c.goexpectSession.Send("ccd open\n"); err != nil {
		return errors.Annotate(err, "send ccd open command").Err()
	}
	waitTimeout := 20 * time.Second
	ccdOpened := false
	reCcdOpen := regexp.MustCompile(`(Press the physical button now|PP press counted|[Tt]imeout|CCD [Oo]pened|Busy|Access Denied)`)
	for i := 0; i < iterationsLimit; i++ {
		output, _, err := c.goexpectSession.Expect(reCcdOpen, waitTimeout)
		if err != nil {
			return errors.Annotate(err, "wait for output from ccd open command").Err()
		} else if regexp.MustCompile(`CCD [Oo]pened`).MatchString(string(output)) {
			fmt.Println("CCD opened.")
			ccdOpened = true
			break
		} else if regexp.MustCompile(`PP press counted`).MatchString(string(output)) {
			fmt.Println("PP press counted.")
			waitTimeout = 120 * time.Second
		} else if regexp.MustCompile(`Press the physical button now`).MatchString(string(output)) {
			fmt.Println("Press the physical button now!")
			waitTimeout = 20 * time.Second
		} else if regexp.MustCompile(`[Tt]imeout`).MatchString(string(output)) {
			return errors.New("Timeout waiting for power button!")
		} else if regexp.MustCompile(`Busy`).MatchString(string(output)) {
			return errors.New("CCD is busy.")
		} else if regexp.MustCompile(`Access Denied`).MatchString(string(output)) {
			return errors.New("Access Denied. You need to use -rma-auth option or run \"gsctool -a -o\" on the DUT in ChromeOS dev console.")
		}
	}
	if !ccdOpened {
		return errors.New("Opening CCD failed.")
	}
	return nil
}

func (c *ccdOpenRun) checkIfTestlabEnabled() (bool, error) {
	if err := c.goexpectSession.Send("ccd testlab\n"); err != nil {
		return false, errors.Annotate(err, "send ccd testlab command").Err()
	}
	reTestlabMode := regexp.MustCompile(`mode\s(enabled|disabled)`)
	_, match, err := c.goexpectSession.Expect(reTestlabMode, timeout)
	if err != nil {
		return false, errors.Annotate(err, "expect output from ccd testlab command").Err()
	}
	fmt.Println("CCD testlab mode: " + match[1])
	if match[1] == "enabled" {
		return true, nil
	}
	return false, nil
}

func (c *ccdOpenRun) runCCDTestlabEnable() error {
	powerButtonAlert()
	testlabEnabled := false
	if err := c.goexpectSession.Send("ccd testlab enable\n"); err != nil {
		return errors.Annotate(err, "send ccd testlab enable command").Err()
	}
	reTestlab := regexp.MustCompile(`(Press the physical button now|PP press counted|Updating testlab to true|CCD test lab mode enabled)`)
	for i := 0; i < iterationsLimit; i++ {
		output, _, err := c.goexpectSession.Expect(reTestlab, timeout)
		if err != nil {
			return errors.Annotate(err, "expect output from ccd testlab enable command").Err()
		} else if regexp.MustCompile(`Updating testlab to true|CCD test lab mode enabled`).MatchString(string(output)) {
			fmt.Println("CCD testlab mode enabled.")
			testlabEnabled = true
			break
		} else if regexp.MustCompile(`PP press counted`).MatchString(string(output)) {
			fmt.Println("PP press counted.")
		} else if regexp.MustCompile(`Press the physical button now`).MatchString(string(output)) {
			fmt.Println("Press the physical button now!")
		}
	}
	if !testlabEnabled {
		return errors.New("Enabling CCD testlab mode failed.")
	}
	return nil
}

func (c *ccdOpenRun) runCCDResetFactory() error {
	if err := c.goexpectSession.Send("ccd reset factory\n"); err != nil {
		return errors.Annotate(err, "send ccd reset factory command").Err()
	}
	if _, _, err := c.goexpectSession.Expect(reCcd, timeout); err != nil {
		return errors.Annotate(err, "expect output from ccd reset factory command").Err()
	} else if err := c.goexpectSession.Send("ccd\n"); err != nil {
		return errors.Annotate(err, "send ccd command").Err()
	}
	output, _, err := c.goexpectSession.Expect(reCcd, timeout)
	if err != nil {
		return errors.Annotate(err, "expect output from ccd command").Err()
	} else if strings.Contains(string(output), "IfOpened") {
		return errors.New("ccd reset factory command failed.")
	}
	fmt.Println("Resetting CCD to factory settings successful.")
	return nil
}
