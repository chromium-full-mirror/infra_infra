// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package dut_services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"

	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/cros/satlab/common/enumeration"
	"go.chromium.org/infra/cros/satlab/common/paths"
	"go.chromium.org/infra/cros/satlab/common/utils/collection"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
	"go.chromium.org/infra/cros/satlab/satlabrpcserver/models"
	"go.chromium.org/infra/cros/satlab/satlabrpcserver/utils"
	"go.chromium.org/infra/cros/satlab/satlabrpcserver/utils/connector"
	"go.chromium.org/infra/cros/satlab/satlabrpcserver/utils/constants"
)

type ListFirmwareCommandResponse struct {
	FwId     string                         `json:"fwid"`
	Model    string                         `json:"model"`
	FwUpdate map[string]*ListFirmwareResult `json:"fw_update"`
}

type ListFirmwareResult struct {
	Host        *Host                  `json:"host"`
	Ec          map[string]interface{} `json:"ec"`
	SignatureId string                 `json:"signature_id"`
}

type Host struct {
	Versions *HostVersions `json:"versions"`
}

type HostVersions struct {
	RO string `json:"ro"`
	RW string `json:"rw"`
}

type GSCInfo struct {
	GSCSerial     string `json:"gsc_serial"`
	ServoUSBCount int    `json:"servo_usb_count"`
}

// DUTServicesImpl implement details of IDUTServices
type DUTServicesImpl struct {
	// config store the ssh configuration because we don't need
	// to create the config everytime.
	config ssh.ClientConfig
	// add this for testing
	port string
	// define a interface for how to connect to the host via ssh
	clientConnector connector.ISSHClientConnector
	// commandExecutor define a interface for executing a command
	commandExecutor executor.IExecCommander
	// subnetSearchRe the regex for parsing the `fping` command.
	// put it in here for testing
	subnetSearchRe *regexp.Regexp
}

func New() (IDUTServices, error) {
	// TODO we should read from file, but we don't know the path now.
	signer, err := utils.ReadSSHKey(constants.SSHKeyPath)
	if err != nil {
		return nil, err
	}
	config := ssh.ClientConfig{
		User: constants.SSHUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         constants.SSHConnectionTimeout,
	}
	sshConnector := connector.New(constants.SSHMaxRetry, constants.SSHRetryDelay)

	return &DUTServicesImpl{
		config:          config,
		port:            constants.SSHPort,
		clientConnector: sshConnector,
		commandExecutor: &executor.ExecCommander{},
		subnetSearchRe:  regexp.MustCompile(`(?P<IP>192\.168\.231\.[0-9][0-9]*[0-9]*).*`),
	}, nil
}

// RunCommandOnIP send the command to the DUT device and then get the result back
//
// ip which device ip want to execute the command.
// cmd which command want to be executed.
// TODO: consider one thing if the command was executed failed should be an error?
func (d *DUTServicesImpl) RunCommandOnIP(ctx context.Context, IP string, cmd string) (*models.SSHResult, error) {
	client, err := d.clientConnector.Connect(ctx, IP+":"+d.port, &d.config)
	if err != nil {
		logging.Infof(ctx, "Can't create a ssh client %v", err)
		return nil, err
	}
	defer func(client *ssh.Client) {
		err := client.Close()
		if err != nil {
			logging.Infof(ctx, "Can't close a ssh client, %v", err)
		}
	}(client)

	session, err := client.NewSession()
	if err != nil {
		logging.Infof(ctx, "Can't create a ssh session, %v", err)
		return nil, err
	}
	defer func(session *ssh.Session) {
		err := session.Close()
		// BUG: https://github.com/golang/go/issues/38115
		if err != nil && err != io.EOF {
			logging.Infof(ctx, "Can't close a ssh session, %v", err)
		}
	}(session)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		var out bytes.Buffer
		var outErr bytes.Buffer
		session.Stdout = &out
		session.Stderr = &outErr
		result := &models.SSHResult{IP: IP}

		err = session.Run(cmd)
		if err != nil {
			result.Error = errors.New(outErr.String())
			return result, nil
		}

		result.Value = out.String()
		return result, nil
	}
}

// RunCommandOnIPs send the command to DUT devices and then get the result back
//
// ips the list of ip which want to execute the command.
// cmd which command want to be executed.
func (d *DUTServicesImpl) RunCommandOnIPs(ctx context.Context, IPs []string, cmd string) []*models.SSHResult {
	ch := make(chan *models.SSHResult)

	var wg sync.WaitGroup

	for _, IP := range IPs {
		wg.Add(1)
		go func(IP string) {
			defer wg.Done()
			out, err := d.RunCommandOnIP(ctx, IP, cmd)
			// SSH connection error, we can't do anything here.
			// log the error message.
			if err != nil {
				logging.Infof(ctx, "Run command on IP: %s failed because the connection problem: %v", IP, err)
				ch <- &models.SSHResult{IP: IP, Error: err}
				return
			}
			ch <- out
		}(IP)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	var res []*models.SSHResult
	for data := range ch {
		res = append(res, data)
	}

	return res
}

func (d *DUTServicesImpl) fetchLeasesFile() (map[string]string, error) {
	// List all IPs that we applied.
	out, err := d.commandExecutor.CombinedOutput(exec.Command(
		paths.DockerPath,
		"exec",
		"dhcp",
		"/bin/cat",
		paths.LeasesPath,
	))

	if err != nil {
		return nil, err
	}

	rawData := strings.Split(string(out), "\n")
	ipToMAC := map[string]string{}

	dnsmasqIPIndex := 2
	dnsmasqMACAddressIndex := 1
	for _, row := range rawData {
		fields := strings.Fields(row)
		// Handle valid data
		if len(fields) == 5 {
			IP := fields[dnsmasqIPIndex]
			mac := fields[dnsmasqMACAddressIndex]
			ipToMAC[IP] = mac
		}
	}

	return ipToMAC, nil
}

func (d *DUTServicesImpl) pingDUTs(ctx context.Context, potentialIPs []string) ([]string, error) {
	// Use fping to figure out which IPs are active
	args := []string{"-a", "-t200", "-B1.0", "-r2"}
	args = append(args, potentialIPs...)

	// Use `Output` instead of `CombinedOutput` here
	// because we need to get error code from error.
	out, err := d.commandExecutor.Output(exec.Command(paths.Fping, args...))

	if err != nil {
		xerr, ok := err.(*exec.ExitError)
		// For reference:
		// fping will return exit status 1 if some hosts were unreachable.
		// https://fping.org/fping.1.html
		if !ok || xerr.ExitCode() != 1 {
			return []string{}, err
		}
	}

	rawData := strings.Split(string(out), "\n")
	activeIPs := []string{}

	for _, row := range rawData {
		if d.subnetSearchRe.MatchString(row) {
			matches := d.subnetSearchRe.FindStringSubmatch(row)
			IPIndex := d.subnetSearchRe.SubexpIndex("IP")
			activeIPs = append(activeIPs, matches[IPIndex])
		}
	}

	return activeIPs, nil
}

// GetConnectedIPs get the connected IPs from `dnsmasq.lease`
// and then check the IPs are alive.
func (d *DUTServicesImpl) GetConnectedIPs(ctx context.Context) ([]Device, error) {
	// This will list all IPs from a leases file
	ipToMACMap, err := d.fetchLeasesFile()
	if err != nil {
		return []Device{}, err
	}

	// Try to ping the IPs and get the active IPs
	potentialIPs := []string{}
	for IP := range ipToMACMap {
		potentialIPs = append(potentialIPs, IP)
	}
	activeIPs, err := d.pingDUTs(ctx, potentialIPs)
	if err != nil {
		return []Device{}, err
	}
	inactiveIPs := collection.Subtract(potentialIPs, activeIPs, func(a, b string) bool {
		return a == b
	})

	// We need to send a command to make sure ssh connection is avaliable.
	// Some DUTs can be pingable, but they can't establish the ssh connection.
	res := d.RunCommandOnIPs(ctx, activeIPs, constants.GrepLSBReleaseCommand)

	result := []Device{}
	for _, r := range res {
		macAddress := ipToMACMap[r.IP]
		hasTestImage := isTestImage(r.Value)
		// we check the some DUTs which install the stable image but they can
		// open the ssh connection.
		result = append(result, Device{IP: r.IP, IsPingable: true, HasTestImage: hasTestImage, MACAddress: macAddress})
	}

	for _, r := range inactiveIPs {
		macAddress := ipToMACMap[r]
		result = append(result, Device{IP: r, IsPingable: false, HasTestImage: false, MACAddress: macAddress})
	}

	return result, nil
}

// isTestImage checking the `lsp-release` contains test image.
func isTestImage(v string) bool {
	return strings.Contains(strings.ToLower(v), constants.ChromeosTestImageReleaseTrack)
}

// GetBoard get the DUT's board from `lsb-release`
func (d *DUTServicesImpl) GetBoard(ctx context.Context, IP string) (string, error) {
	res, err := d.RunCommandOnIP(ctx, IP, fmt.Sprintf(
		"%s | grep %s",
		constants.GrepLSBReleaseCommand,
		constants.ChromeosReleaseBoard,
	))
	if err != nil {
		return "", err
	}
	if res.Error != nil {
		return "", res.Error
	}

	if b, ok := strings.CutPrefix(res.Value, constants.ChromeosReleaseBoard); ok {
		return strings.TrimRight(b, "\n\t"), nil
	}

	return "", errors.New("can not find the board information in lsb release.")
}

// GetModel get the DUT's model from `cros_config / test-label` / `cros_config / name`
func (d *DUTServicesImpl) GetModel(ctx context.Context, IP string) (string, error) {
	for _, cmd := range constants.GetModelCommands {
		res, err := d.RunCommandOnIP(ctx, IP, cmd)
		if err != nil || res.Error != nil {
			// Skip if we run a command failed.
			continue
		}

		if res.Value != "" {
			// If we find the model isn't empty then we return it.
			return strings.TrimRight(res.Value, "\n\t"), nil
		}
	}
	return "", errors.New("can not get the model information")
}

// GetGSCSerialAndServoUSBCount returns the cr50/ti50 usb connector serial number for the given IP
func (d *DUTServicesImpl) GetGSCSerialAndServoUSBCount(ctx context.Context, IP string) (*GSCInfo, error) {
	res, err := d.RunCommandOnIP(ctx, IP, constants.GetGSCSerialAndServoUSB)
	if err != nil {
		logging.Infof(ctx, "command '%s'to get gsc serial and servo usb connector failed on %s: %v", constants.GetGSCSerialAndServoUSB, IP, err)
		return nil, err
	}

	if res.Error != nil {
		logging.Infof(ctx, "command '%s'to get gsc serial and servo usb connector failed on %s: %v", constants.GetGSCSerialAndServoUSB, IP, res.Error)
		return nil, res.Error
	}

	var gscInfo GSCInfo
	err = json.Unmarshal([]byte(res.Value), &gscInfo)
	if err != nil {
		logging.Infof(ctx, "Json decode error while processing gsc serial: %v", err)
		return nil, err
	}
	return &gscInfo, nil
}

// GetServoSerial returns the Servo serial number for the given IP
func (d *DUTServicesImpl) GetServoSerial(ctx context.Context, IP string, usbDevices []enumeration.USBDevice) (bool, string, error) {

	gscServoInfo, err := d.GetGSCSerialAndServoUSBCount(ctx, IP)
	if err != nil {
		logging.Infof(ctx, "unable to get gsc serial and servo usb count: %v", err)
		return false, "", err
	}

	if gscServoInfo.GSCSerial == "" {
		logging.Infof(ctx, "gsc serial is empty, cannot determine servo serial: %v", err)
		return false, "", nil
	}

	// Check if there are any servo connections found
	if gscServoInfo.ServoUSBCount > 0 {
		device, err := enumeration.FindServoFromDUT(gscServoInfo.GSCSerial, usbDevices)
		if err != nil {
			logging.Infof(ctx, "found servo connection but not detected on cr50/ti50 (serial:%s) port for %s : %v", gscServoInfo.GSCSerial, IP, err)
			return true, "", nil
		}
		logging.Infof(ctx, "detected servo connection with serial %s: cr50/ti50 (serial:%s) port for %s ", device.Serial, gscServoInfo.GSCSerial, IP)
		return true, device.Serial, nil
	}

	logging.Infof(ctx, "No Servo connected or detected for %s", IP)
	return false, "", nil
}

// GetUSBDevicePaths returns all the USBDevices instance of plugged devices
func (d *DUTServicesImpl) GetUSBDevicePaths(ctx context.Context) ([]enumeration.USBDevice, error) {
	return enumeration.GetAllServoUSBDevices()
}

// GetCCDStatus gets the CCD status from the given IP address. If it the command return empty string,
// we set it `Unknown` status.
func (d *DUTServicesImpl) GetCCDStatus(ctx context.Context, address string) (string, error) {
	res, err := d.RunCommandOnIP(ctx, address, fmt.Sprintf(
		"%s | grep State | awk '{print $2}'",
		constants.CCDStatusCommand,
	))
	if err != nil {
		return "", err
	}
	if res.Error != nil {
		return "", res.Error
	}

	status := strings.TrimSpace(res.Value)
	if status == "" {
		return "Unknown", nil
	} else {
		return status, nil
	}
}
