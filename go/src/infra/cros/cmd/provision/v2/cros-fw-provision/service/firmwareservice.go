// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmwareservice

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"go.chromium.org/chromiumos/test/provision/lib/servo_lib"
	"go.chromium.org/chromiumos/test/provision/lib/servoadapter"
	common_utils "go.chromium.org/chromiumos/test/provision/v2/common-utils"

	"github.com/pkg/errors"
	conf "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"gopkg.in/yaml.v3"
)

// FirmwareVersions holds the ro and rw versions read from a firmware binary.
type FirmwareVersions struct {
	AP struct {
		Versions struct {
			RO string `yaml:"ro"`
			RW string `yaml:"rw"`
			// The EC version embedded within the AP-RW image.
			ECRW string `yaml:"ecrw"`
		} `yaml:"versions"`
	} `yaml:"host"`
	EC struct {
		Versions struct {
			RO     string `yaml:"ro"`
			RW     string `yaml:"rw"`
			RWHash string
		} `yaml:"versions"`
	} `yaml:"ec"`
}

// FirmwareService implements ServiceInterface
type FirmwareService struct {
	// In case of flashing over SSH, |connection| connects to the DUT.
	// In case of flashing over Servo, |connection| connects to the ServoHost.
	connection common_utils.ServiceAdapterInterface

	// DetailedRequest fields
	mainRwPath *conf.StoragePath
	mainRoPath *conf.StoragePath
	ecRoPath   *conf.StoragePath
	ecRwPath   *conf.StoragePath

	board, model string
	// The name of the model's fw binary from config.yaml
	CorebootName string
	// StandaloneECName is the name of the ec binary for standalone (i.e. firmware-ec-Rxxx branches) builds.
	StandaloneECName string
	// LegacyECName is the name of the ec binary for legacy_fw (i.e. firmware-board branches) builds.
	LegacyECName string

	force bool

	ecChip string

	// imagesMetadata is a map from gspath -> ImageArchiveMetadata.
	// Allows to avoid redownloading/reprocessing archives.
	imagesMetadata map[string]ImageArchiveMetadata

	// DUTServer is the gRPC connection to the DUT.
	DUTServer api.DutServiceClient
	// servoClient is the gRPC connection to the Servo Nexus.
	servoClient api.ServodServiceClient
	servoConfig *labapi.Servo

	useServo bool
	// wrapper around servoClient connection with extra servod-related functions
	servoConnection servoadapter.ServoHostInterface
	servoPort       int

	CacheServer      url.URL
	ExpectedVersions FirmwareVersions

	// RestartRequired indicates that a firmware update was performed, but the DUT has not yet rebooted.
	RestartRequired bool
}

type versionJSON struct {
	Default FirmwareVersions `yaml:"default"`
}

// NewFirmwareService initializes a FirmwareService.
func NewFirmwareService(ctx context.Context, dutServer api.DutServiceClient,
	servoClient api.ServodServiceClient, cacheServer url.URL, board, model string,
	useServo bool, req *api.InstallRequest, servoConfig *labapi.Servo) (*FirmwareService, error) {
	metadata := new(api.FirmwareProvisionInstallMetadata)
	if req.GetMetadata().MessageIs(metadata) {
		if err := req.GetMetadata().UnmarshalTo(metadata); err != nil {
			return nil, InvalidRequestErr(errors.Wrap(err, "unmarshalling metadata"))
		}
	} else {
		return nil, InvalidRequestErr(errors.Errorf("FirmwareProvisionInstallMetadata is required, got %s", req.String()))
	}
	detailedRequest := metadata.FirmwareConfig
	dutAdapter := common_utils.NewServiceAdapter(dutServer, false /*noReboot*/)

	fws := FirmwareService{
		connection:     dutAdapter,
		DUTServer:      dutServer,
		servoClient:    servoClient,
		servoConfig:    servoConfig,
		board:          board,
		model:          model,
		force:          false,
		useServo:       useServo,
		imagesMetadata: make(map[string]ImageArchiveMetadata),
		CacheServer:    cacheServer,
	}

	// Firmware may be updated in write-protected mode, where only 'rw' regions
	// would be update, or write-protection may be disabled (dangerous) in order
	// to update 'ro' regions.
	//
	// The only 'rw' firmware is the main one, aka AP firmware, which also syncs
	// EC 'rw'. It will be flashed with write protection turned on.
	fws.mainRwPath = detailedRequest.MainRwPayload.GetFirmwareImagePath()

	// Read-only firmware images include AP, EC, and PD firmware, and will be
	// flashed with write protection turned off.
	fws.mainRoPath = detailedRequest.MainRoPayload.GetFirmwareImagePath()
	fws.ecRoPath = detailedRequest.EcRoPayload.GetFirmwareImagePath()
	fws.ecRwPath = detailedRequest.EcRwPayload.GetFirmwareImagePath()

	if useServo {
		fws.prepareServoConnection(ctx, servoClient)
	}

	fws.PrintRequestInfo()

	if !fws.UpdateRo() && !fws.UpdateRw() {
		return nil, InvalidRequestErr(errors.New("no paths to images specified"))
	}

	return &fws, nil
}

// Confirms that cros-servod connection is functional, and fills the following
// fields in FirmwareService:
//   - servoConnection
//   - ecChip
//   - servoPort
func (fws *FirmwareService) prepareServoConnection(ctx context.Context, servoClient api.ServodServiceClient) error {
	if servoClient == nil {
		return InvalidRequestErr(errors.New("servo use is requested, but servo client not provided"))
	}

	// Note: dut.GetChromeos().Servo.ServodAddress.Port is the port of cros-servod
	// service, we seem to be missing port of servod itself.
	// Always use default servod port 9999 for now.
	if fws.servoPort == 0 {
		fws.servoPort = 9999
	}

	fws.servoConnection = servoadapter.NewServoHostAdapterFromExecCmder(fws.board,
		fws.model,
		fws.servoPort,
		servoClient)

	// Ask servod for servo_type. This implicitly checks if servod is running
	// and servo connection is working.
	servoTypeStr, err := fws.servoConnection.GetVariable(ctx, "servo_type")
	if err != nil {
		return UnreachablePreProvisionErr(errors.Wrapf(err, "failed to get servo_type. "+
			"Is servod running on port %v and connected to the DUT?",
			fws.servoPort)) // TODO(sfrolov): add UnreachableCrosServodErr
	}

	// ask servod for serial number of the connected DUT.
	servoType := servo_lib.NewServoType(servoTypeStr)

	if servoType.IsMultipleServos() {
		// Handle dual servo.
		// We need CCD if ec_ro is set, otherwise, servo_micro will be faster.
		preferCCD := false
		if fws.ecRoPath != nil {
			preferCCD = true
		}
		servoSubtypeStr := servoType.PickServoSubtype(preferCCD)
		servoType = servo_lib.NewServoType(servoSubtypeStr)
	}

	fws.ecChip, err = fws.servoConnection.GetVariable(ctx, "ec_chip")
	if err != nil {
		return UnreachablePreProvisionErr(errors.Wrap(err, "failed to get ec_chip variable")) // TODO: cros-servod is unreachable
	}
	return nil
}

// PrintRequestInfo logs details of the provisioning operation.
func (fws *FirmwareService) PrintRequestInfo() {
	informationString := "provisioning "

	images := []string{}
	if fws.mainRwPath != nil {
		images = append(images, "AP(RW)")
	}
	if fws.mainRoPath != nil {
		images = append(images, "AP(RO)")
	}
	if fws.ecRoPath != nil {
		images = append(images, "EC(RO)")
	}
	if fws.ecRwPath != nil {
		images = append(images, "EC(RW)")
	}
	informationString += strings.Join(images, " and ") + " firmware"

	flashMode := "SSH"
	informationString += " over " + flashMode + ". "

	informationString += "Board: " + fws.board + ". "

	informationString += "Model: " + fws.model + ". "

	informationString += fmt.Sprintf("Force: %v.", fws.force)

	log.Println("[FW Provisioning]", informationString)
}

// UpdateRw returns whether the read/write firmware is being operated on.
func (fws *FirmwareService) UpdateRw() bool {
	// If AP RW was specified, or EC RW without AP RO
	return fws.mainRwPath != nil || (fws.ecRwPath != nil && fws.mainRoPath == nil)
}

// UpdateRo returns whether the read-only firmware is being operated on.
func (fws *FirmwareService) UpdateRo() bool {
	return (fws.mainRoPath != nil) || (fws.ecRoPath != nil)
}

// GetBoard returns board of the DUT to provision. Returns empty string if board is not known.
func (fws *FirmwareService) GetBoard() string {
	return fws.board
}

// WaitForReconnect attempts to ssh to the DUT until it connects
func (fws *FirmwareService) WaitForReconnect(ctx context.Context) error {
	// Attempts to run `true` on the DUT over SSH until |reconnectRetries| attempts.
	const reconnectRetries = 10
	const reconnectAttemptWait = 10 * time.Second
	const reconnectFailPause = 10 * time.Second
	var connectErr error
	for i := 0; i < reconnectRetries; i++ {
		reconnectCtx, reconnCancel := context.WithTimeout(ctx, reconnectAttemptWait)
		defer reconnCancel()
		_, connectErr = fws.connection.RunCmd(reconnectCtx, "true", nil)
		if connectErr == nil {
			return nil
		}
		time.Sleep(reconnectFailPause)
	}
	log.Printf("Timed out waiting for DUT to connect: %v\n", connectErr)
	return connectErr
}

// RestartDut restarts the DUT using one of the available mechanisms.
// Preferred restart method is to send "power_state:reset" command to servod.
// If servod restarting failed/not available in the environment,
// then this function will try to SSH to the DUT and run `restart`,
// if |requireServoReset| is false. If |requireServoReset| is True, restart over
// SSH will not be attempted, and the function will return an error.
func (fws *FirmwareService) RestartDut(ctx context.Context, requireServoReset bool) error {
	if requireServoReset && fws.servoConnection == nil {
		return errors.New("servo restart is required but servo connection not available")
	}
	// over Servo first
	if fws.servoConnection != nil {
		log.Printf("[FW Provisioning: Restart DUT] restarting DUT with \"dut-control power_state:reset\" over servo.\n")
		servoRestartErr := fws.servoConnection.RunDutControl(ctx, []string{"power_state:reset"})
		if servoRestartErr == nil {
			if fws.connection != nil {
				// If SSH connection to DUT was available, wait until it's back up again.
				return fws.WaitForReconnect(ctx)
			}
			waitDuration := 30 * time.Second
			log.Printf("[FW Provisioning: Restart DUT] waiting for %v for DUT to finish rebooting.\n", waitDuration.String())
			time.Sleep(waitDuration)
			powerState, getPowerStateErr := fws.servoConnection.GetVariable(ctx, "ec_system_powerstate")
			if getPowerStateErr != nil {
				log.Printf("[FW Provisioning: Restart DUT] failed to get power state after reboot: %v\n", getPowerStateErr)
			} else {
				log.Printf("[FW Provisioning: Restart DUT] DUT power state after reboot: %v\n", powerState)
			}
			return getPowerStateErr
		}
		log.Printf("[FW Provisioning: Restart DUT] failed to restart DUT via Servo: %v.\n", servoRestartErr)
		if requireServoReset {
			return servoRestartErr
		}
	}

	// over SSH if allowed by |requireServoReset|
	if fws.connection != nil {
		log.Printf("[FW Provisioning: Restart DUT] restarting DUT over SSH.")
		fws.connection.Restart(ctx)
		return fws.WaitForReconnect(ctx)
	}
	return errors.New("failed to restart: no SSH connection to the DUT")
}

// GetModel returns model of the DUT to provision. Returns empty string if model is not known.
func (fws *FirmwareService) GetModel() string {
	return fws.model
}

// DeleteArchiveDirectories deletes files on the servo host or DUT.
func (fws *FirmwareService) DeleteArchiveDirectories() error {
	var cleanedDevice common_utils.ServiceAdapterInterface
	if fws.useServo {
		// If servo is used, the files will be located on the ServoHost.
		cleanedDevice = fws.servoConnection
	} else {
		// If SSH is used, the files will be located on the DUT in /tmp/
		// It's not strictly necessary to delete them before reboot.
		cleanedDevice = fws.connection
	}

	var allErrors []string
	for _, imgMetadata := range fws.imagesMetadata {
		err := cleanedDevice.DeleteDirectory(context.Background(), imgMetadata.ArchiveDir)
		if err != nil {
			allErrors = append(allErrors, fmt.Sprintf("failed to delete %v: %v", imgMetadata.ArchiveDir, err))
		}
	}

	if len(allErrors) > 0 {
		return errors.New(strings.Join(allErrors, ". "))
	}

	return nil
}

// ServoHostPath returns the ServoHostPath to pass to servoClient
func (fws *FirmwareService) ServoHostPath() string {
	host := fws.servoConfig.GetServodAddress().GetAddress()
	if strings.Contains(host, ":") {
		return host
	}
	if strings.HasSuffix(host, "-docker_servod") {
		return ""
	}
	return fmt.Sprintf("%s:22", host)
}

// ServodDockerContainerName returns the ServodDockerContainerName to pass to servoClient
func (fws *FirmwareService) ServodDockerContainerName() string {
	host := fws.servoConfig.GetServodAddress().GetAddress()
	if strings.HasSuffix(host, "-docker_servod") {
		return host
	}
	return ""
}

// FlashWithFutility flashes the DUT using "futility" tool.
// futility will be run with "--mode=recovery".
// if |rwOnly| is true, futility will flash only RW regions.
// if |rwOnly| is false, futility will flash both RW and RO regions.
// futilityArgs must include argument(s) that provide path(s) to the images.
// apImagePath is the path to the AP image.
// ecImagePath is the path to the EC image.
//
// If flashing over ssh, simply calls runFutility().
// If flashing over servo, also runs pre- and post-flashing dut-controls.
func (fws *FirmwareService) FlashWithFutility(ctx context.Context, rwOnly bool, futilityImageArgs []string, apImagePath, ecImagePath string) error {
	// TODO: Remove this and move to call sites.
	if err := fws.ExtractFirmwareVersions(ctx, rwOnly, futilityImageArgs, apImagePath); err != nil {
		return errors.Wrap(err, "extract versions")
	}

	err := fws.sshFlash(ctx, rwOnly, futilityImageArgs, ecImagePath)
	if err != nil && strings.Contains(err.Error(), "CSME_LOCKED") {
		if fws.servoClient != nil {
			log.Printf("Attempting to unlock CSME via servo")
			op, err := fws.servoClient.StartServod(ctx, &api.StartServodRequest{
				ServoHostPath:             fws.ServoHostPath(),
				ServodDockerContainerName: fws.ServodDockerContainerName(),
				ServodPort:                fws.servoConfig.GetServodAddress().GetPort(),
				SerialName:                fws.servoConfig.GetSerial(),
				Board:                     fws.board,
				Model:                     fws.model,
			})
			if err != nil {
				return errors.Wrap(err, "failed to start servod")
			}
			waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			opAny, err := common_utils.WaitLongRunningOp(waitCtx, log.Default(), op)
			if err != nil {
				return errors.Wrap(err, "failed to start servod (lro)")
			}
			startResponse := new(api.StartServodResponse)
			if err := opAny.UnmarshalTo(startResponse); err != nil {
				return errors.Wrap(err, "failed to start servod (anypb)")
			}
			if startResponse.GetFailure() != nil {
				return errors.Wrapf(err, "failed to start servod: %s", startResponse.GetFailure().GetErrorMessage())
			}
			tmpName := fmt.Sprintf("/var/tmp/bios_%d.bin", fws.servoConfig.GetServodAddress().GetPort())
			defer func() {
				out, err := fws.servoClient.ExecCmd(ctx, &api.ExecCmdRequest{
					ServoHostPath:             fws.ServoHostPath(),
					ServodDockerContainerName: fws.ServodDockerContainerName(),
					Command:                   fmt.Sprintf("rm %s", tmpName),
				})
				if err != nil {
					log.Printf("Failed to rm temp file: %s, exit code %d: %s", err, out.GetExitInfo().GetStatus(), out.GetStderr())
				} else if out.GetExitInfo().GetStatus() != 0 {
					log.Printf("Failed to rm temp file, exit code %d: %s", out.GetExitInfo().GetStatus(), out.GetStderr())
				}
			}()
			log.Printf("Reading BIOS")
			out, err := fws.servoClient.ExecCmd(ctx, &api.ExecCmdRequest{
				ServoHostPath:             fws.ServoHostPath(),
				ServodDockerContainerName: fws.ServodDockerContainerName(),
				Command:                   fmt.Sprintf("futility read --servo_port=%d %s", fws.servoConfig.GetServodAddress().GetPort(), tmpName),
			})
			if err != nil {
				return errors.Wrapf(err, "backup fw failed, exit code %d: %s", out.GetExitInfo().GetStatus(), out.GetStderr())
			} else if out.GetExitInfo().GetStatus() != 0 {
				return errors.Errorf("backup fw failed, exit code %d: %s", out.GetExitInfo().GetStatus(), out.GetStderr())
			}
			log.Printf("Writing BIOS")
			out, err = fws.servoClient.ExecCmd(ctx, &api.ExecCmdRequest{
				ServoHostPath:             fws.ServoHostPath(),
				ServodDockerContainerName: fws.ServodDockerContainerName(),
				Command:                   fmt.Sprintf("futility update --servo_port=%d --quirks=unlock_csme -i %s --mode=recovery --wp=0", fws.servoConfig.GetServodAddress().GetPort(), tmpName),
			})
			if err != nil {
				return errors.Wrapf(err, "update fw failed, exit code %d: %s", out.GetExitInfo().GetStatus(), out.GetStderr())
			} else if out.GetExitInfo().GetStatus() != 0 {
				return errors.Errorf("update fw failed, exit code %d: %s", out.GetExitInfo().GetStatus(), out.GetStderr())
			}
			// Try the ssh flash again
			return fws.sshFlash(ctx, rwOnly, futilityImageArgs, ecImagePath)
		}
	}
	return err
}

// ExtractFirmwareVersions uses futility to get the version numbers from the images.
func (fws *FirmwareService) ExtractFirmwareVersions(ctx context.Context, rwOnly bool, futilityImageArgs []string, apImagePath string) error {
	if len(futilityImageArgs) == 0 {
		return errors.New("unable to gte version: no futility Image args provided")
	}

	futilityArgs := futilityImageArgs
	futilityArgs = append([]string{"update", "--manifest"}, futilityArgs...)

	connection := fws.GetConnectionToFlashingDevice()
	for i, a := range futilityArgs {
		futilityArgs[i] = Escape(a)
	}
	out, err := connection.RunCmd(ctx, "futility", futilityArgs)
	if err != nil {
		return errors.Wrap(err, "failed to exec futility")
	}

	// b/360198909: Sometimes the output contains control characters.
	out = strings.ReplaceAll(out, "\x01", "")
	log.Printf("Manifest: %s", out)

	versions := versionJSON{}
	err = yaml.Unmarshal([]byte(out), &versions)
	if err != nil {
		return errors.Wrapf(err, "failed to parse futility manifest: %q", out)
	}

	if !rwOnly && versions.Default.AP.Versions.RO != "" {
		fws.ExpectedVersions.AP.Versions.RO = versions.Default.AP.Versions.RO
	}
	if versions.Default.AP.Versions.RW != "" {
		fws.ExpectedVersions.AP.Versions.RW = versions.Default.AP.Versions.RW
	}
	if !rwOnly && versions.Default.EC.Versions.RO != "" {
		fws.ExpectedVersions.EC.Versions.RO = versions.Default.EC.Versions.RO
	}
	if versions.Default.AP.Versions.ECRW != "" && fws.board != "drallion" && fws.board != "sarien" {
		fws.ExpectedVersions.EC.Versions.RW = versions.Default.AP.Versions.ECRW
		fws.ExpectedVersions.EC.Versions.RWHash = ""
	}

	// If there is no EC RW version, get the EC RW hash from the AP image.
	if versions.Default.AP.Versions.ECRW == "" && apImagePath != "" && fws.board != "drallion" && fws.board != "sarien" {
		ecImagePath := fmt.Sprintf("%s-ecrw.hash", apImagePath)
		_, err = connection.RunCmd(ctx, "cbfstool", []string{fmt.Sprintf("'%s'", apImagePath), "extract", "-r", "FW_MAIN_A", "-n", "ecrw.hash", "-f", fmt.Sprintf("'%s'", ecImagePath)})
		if err != nil {
			return errors.Wrap(err, "failed to extract ecrw.hash")
		}
		out, err = connection.RunCmd(ctx, "od", []string{"-A", "n", "-t", "x1", fmt.Sprintf("'%s'", ecImagePath)})
		if err != nil {
			return errors.Wrap(err, "failed to get ecrw hash")
		}
		fws.ExpectedVersions.EC.Versions.RWHash = strings.Join(strings.FieldsFunc(out, unicode.IsSpace), "")
	}
	log.Printf("fws.ExpectedVersions: %+v", fws.ExpectedVersions)
	return nil
}

// GetVersions returns a FirmwareProvisionResponse with the expected firmware versions
func (fws *FirmwareService) GetVersions() *api.FirmwareProvisionResponse {
	r := new(api.FirmwareProvisionResponse)
	r.ApRoVersion = fws.ExpectedVersions.AP.Versions.RO
	r.ApRwVersion = fws.ExpectedVersions.AP.Versions.RW
	r.EcRoVersion = fws.ExpectedVersions.EC.Versions.RO
	r.EcRwVersion = fws.ExpectedVersions.EC.Versions.RW
	return r
}

// ActiveFirmwareVersions returns the firmware versions currently running on the DUT.
func (fws *FirmwareService) ActiveFirmwareVersions(ctx context.Context) (*FirmwareVersions, error) {
	var versions FirmwareVersions
	// Run crossystem ro_fwid fwid to get AP versions
	out, err := fws.connection.RunCmd(ctx, "crossystem", []string{"ro_fwid", "fwid"})
	if err != nil {
		return nil, errors.Wrap(err, "crossystem failed")
	}
	apVers := strings.SplitN(out, " ", 2)
	if len(apVers) != 2 {
		return nil, errors.Errorf("incorrect crossystem output: %q", out)
	}
	versions.AP.Versions.RO = apVers[0]
	versions.AP.Versions.RW = apVers[1]

	// Run ectool version to get EC versions
	out, stderr, err := RunDUTCommand(ctx, fws.DUTServer, time.Minute, "ectool", []string{"version"}, nil)
	if err != nil {
		// Wilco, etc.
		if strings.Contains(stderr, "Very likely this board doesn't have a Chromium EC") {
			return &versions, nil
		}
		return nil, errors.Wrap(err, "ectool failed")
	}
	roRE, err := regexp.Compile(`RO version:\s*(\S+)\s*\n`)
	if err != nil {
		return nil, errors.Wrap(err, "invalid roRE")
	}
	rwRE, err := regexp.Compile(`RW version:\s*(\S+)\s*\n`)
	if err != nil {
		return nil, errors.Wrap(err, "invalid rwRE")
	}
	if m := roRE.FindStringSubmatch(out); m != nil {
		versions.EC.Versions.RO = m[1]
	} else {
		return nil, errors.Errorf("EC RO version not found in %q", out)
	}
	if m := rwRE.FindStringSubmatch(out); m != nil {
		versions.EC.Versions.RW = m[1]
	} else {
		return nil, errors.Errorf("EC RW version not found in %q", out)
	}

	// Run ectool echash to get EC RW hash
	out, stderr, err = RunDUTCommand(ctx, fws.DUTServer, time.Minute, "ectool", []string{"echash"}, nil)
	if err != nil {
		// Wilco, etc.
		if strings.Contains(stderr, "Very likely this board doesn't have a Chromium EC") {
			return &versions, nil
		}
		return nil, errors.Wrap(err, "ectool failed")
	}
	statusRE, err := regexp.Compile(`status:\s*(\S+)\s*\n`)
	if err != nil {
		return nil, errors.Wrap(err, "invalid statusRE")
	}
	hashRE, err := regexp.Compile(`hash:\s*(\S+)\s*\n`)
	if err != nil {
		return nil, errors.Wrap(err, "invalid hashRE")
	}
	if m := statusRE.FindStringSubmatch(out); m == nil || m[1] != "done" {
		return nil, errors.Errorf("EC hash not done in %q", out)
	}
	if m := hashRE.FindStringSubmatch(out); m != nil {
		versions.EC.Versions.RWHash = m[1]
	} else {
		return nil, errors.Errorf("EC RW hash not found in %q", out)
	}
	return &versions, nil
}

func (fws *FirmwareService) sshFlash(ctx context.Context, rwOnly bool, futilityImageArgs []string, ecImagePath string) error {
	return fws.runFutility(ctx, rwOnly, futilityImageArgs, ecImagePath)
}

const (
	// The character class \w is equivalent to [0-9A-Za-z_]. Leading equals sign is unsafe in zsh,
	// see http://zsh.sourceforge.net/Doc/Release/Expansion.html#g_t_0060_003d_0027-expansion.
	leadingSafeChars  = `-\w@%+:,./`
	trailingSafeChars = leadingSafeChars + "="
)

// safeRE matches an argument that can be literally included in a shell
// command line without requiring escaping.
var safeRE = regexp.MustCompile(fmt.Sprintf("^[%s][%s]*$", leadingSafeChars, trailingSafeChars))

func (fws *FirmwareService) runFutility(ctx context.Context, rwOnly bool, futilityImageArgs []string, ecImagePath string) error {
	if len(futilityImageArgs) == 0 {
		return errors.New("unable to flash: no futility Image args provided")
	}

	futilityArgs := futilityImageArgs
	futilityArgs = append([]string{"update", "--mode=recovery"}, futilityArgs...)

	if rwOnly {
		futilityArgs = append(futilityArgs, "--wp=1")
	} else {
		futilityArgs = append(futilityArgs, "--wp=0")
	}

	if fws.force {
		futilityArgs = append(futilityArgs, "--force")
	}

	fws.RestartRequired = true
	connection := fws.GetConnectionToFlashingDevice()
	if ecImagePath != "" {
		// If we are flashing EC, we might lose SSH access, so use nohup to run futility in a temporary python script, and poll for results
		logFile := path.Join(path.Dir(ecImagePath), "futility.log")
		startupLogFile := path.Join(path.Dir(ecImagePath), "futility.start")
		pyScript := path.Join(path.Dir(ecImagePath), "futility.py")
		var scriptBody strings.Builder
		scriptBody.WriteString(`#!/usr/bin/env python3

import subprocess

with open("`)
		scriptBody.WriteString(logFile)
		scriptBody.WriteString(`", "wb", buffering=0) as outFile:
  rc = subprocess.run(["futility"`)
		for _, arg := range futilityArgs {
			scriptBody.WriteString(", '")
			scriptBody.WriteString(arg)
			scriptBody.WriteString("'")
		}
		scriptBody.WriteString(`], stdout=outFile, stderr=outFile, check=False, bufsize=0)
  outFile.write(f"EXIT CODE: {rc.returncode}\n".encode("utf-8"))
  outFile.flush()
  subprocess.run(["sync", "-d", "/var/tmp", "/usr/local/tmp"])
  subprocess.run(["reboot"])
`)
		_, _, err := RunDUTCommand(ctx, fws.DUTServer, time.Minute, "cat", []string{">", fmt.Sprintf("'%s'", pyScript)}, []byte(scriptBody.String()))
		if err != nil {
			return errors.Wrap(err, "failed to create futility.py")
		}
		defer connection.RunCmd(ctx, "rm", []string{"-f", fmt.Sprintf("'%s'", logFile)})

		_, err = connection.RunCmd(ctx, "bash", []string{"-c", Escape(fmt.Sprintf("nohup python3 '%s' </dev/null >&'%s' & exit", pyScript, startupLogFile))})
		if err != nil {
			return errors.Wrap(err, "failed to run nohup")
		}
		startTime := time.Now()
		exitCodeRe := regexp.MustCompile(`EXIT CODE: (-?\d+)`)
		csmeLockedRe := regexp.MustCompile(`The CSME was already locked`)
		logfileLen := 0

		// Poll every 10s for the log file to appear and have the "EXIT CODE:" string in it.
		for time.Since(startTime) < 10*time.Minute {
			time.Sleep(10 * time.Second)
			catCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			buf, err := connection.RunCmd(catCtx, "cat", []string{fmt.Sprintf("'%s'", logFile)})
			if err != nil {
				log.Printf("failed to download %q: %v", logFile, err)
				catCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()
				buf, err = connection.RunCmd(catCtx, "cat", []string{fmt.Sprintf("'%s'", startupLogFile)})
				if err != nil {
					log.Printf("failed to download %q: %v", startupLogFile, err)
					continue
				} else {
					log.Printf("Futility startup:\n%s", buf)
				}
			} else {
				if logfileLen > len(buf) {
					log.Printf("Logfile shrank! Might be an EC crash. Was %d, now %d", logfileLen, len(buf))
					log.Printf("Futility output:\n%s", buf)
					logfileLen = len(buf)
				} else {
					log.Printf("Futility output:\n%s", buf[logfileLen:])
					logfileLen = len(buf)
				}
			}
			m := csmeLockedRe.FindStringSubmatch(buf)
			if m != nil {
				log.Printf("Futility output:\n%s", buf)
				return errors.Errorf("CSME_LOCKED: %q", buf)
			}
			m = exitCodeRe.FindStringSubmatch(buf)
			if m != nil {
				log.Printf("Futility output:\n%s", buf)
				if m[1] == "0" {
					fws.RestartRequired = false
					return nil
				}
				return errors.Errorf("futility failed: %q", buf)
			}
		}
		return errors.New("Timeout waiting for futility")
	} else {
		for i, a := range futilityArgs {
			futilityArgs[i] = Escape(a)
		}

		if _, err := connection.RunCmd(ctx, "futility", futilityArgs); err != nil {
			return err
		}
	}
	return nil
}

// CheckForCSMESections looks at the image at imagePath, and returns true if csme_unlock is supported.
func (fws *FirmwareService) CheckForCSMESections(ctx context.Context, imagePath string) (bool, error) {
	// futility needs CONFIG_IFD_CHIPSET to be set in the config file in cbfs or the board to be nissa.
	// Also the idftool must exist.

	out, err := fws.connection.RunCmd(ctx, "which", []string{"ifdtool"})
	if err != nil {
		log.Printf("idftool not found:%s", string(out))
		return false, nil
	}
	// Extract the config file
	configFile := imagePath + "-config"
	out, err = fws.connection.RunCmd(ctx, "cbfstool", []string{Escape(imagePath), "extract", "-n", "config", "-f", configFile})
	if err != nil {
		return false, errors.Wrapf(err, "command output: %s", string(out))
	}
	reader, err := fws.connection.FetchFile(ctx, configFile)
	if err != nil {
		return false, errors.Wrap(err, "downloading file")
	}
	// Look for CONFIG_IFD_CHIPSET or the same special case for nissa that futility uses.
	sc := bufio.NewScanner(reader)
	for sc.Scan() {
		cfg, value, _ := strings.Cut(sc.Text(), "=")
		if cfg == "CONFIG_IFD_CHIPSET" || (cfg == "CONFIG_IFD_BIN_PATH" && strings.Contains(value, "/nissa/")) {
			log.Printf("Image %s has %s\n", imagePath, cfg)
			return true, nil
		}
	}
	log.Printf("Image %s has no CONFIG_IFD_CHIPSET\n", imagePath)
	return false, nil
}

// GetConnectionToFlashingDevice returns connection to the device that stores the
// firmware image and runs futility.
// Returns connection to ServoHost if fws.useServo, connection to DUT otherwise.
func (fws *FirmwareService) GetConnectionToFlashingDevice() common_utils.ServiceAdapterInterface {
	if fws.useServo {
		return fws.servoConnection
	}
	return fws.connection
}

// IsServoUsed returns whether servo is used during the provisioning.
func (fws *FirmwareService) IsServoUsed() bool {
	return fws.useServo
}

// IsForceUpdate returns whether an update is forced during the provisioning.
func (fws *FirmwareService) IsForceUpdate() bool {
	return fws.force
}

// GetMainRwPath returns the path of the read/write part of the AP firmware.
func (fws *FirmwareService) GetMainRwPath() string {
	return fws.mainRwPath.GetPath()
}

// GetMainRoPath returns the path of readonly part of the AP firmware.
func (fws *FirmwareService) GetMainRoPath() string {
	return fws.mainRoPath.GetPath()
}

// GetEcRoPath returns the path for the readonly portion of the EC firmware.
func (fws *FirmwareService) GetEcRoPath() string {
	return fws.ecRoPath.GetPath()
}

// GetEcRwPath returns the path for the read/write portion of the EC firmware.
func (fws *FirmwareService) GetEcRwPath() string {
	return fws.ecRwPath.GetPath()
}

type configData struct {
	ChromeOS struct {
		Configs []struct {
			Firmware struct {
				BuildTargets struct {
					// AP image name, if missing fallback to ImageName
					Coreboot string `yaml:"coreboot"`
					EC       string `yaml:"ec"`
					ZephyrEC string `yaml:"zephyr-ec"`
				} `yaml:"build-targets"`
				ImageName string `yaml:"image-name"`
			} `yaml:"firmware"`
			Name     string `yaml:"name"`
			Identity struct {
				SKUID int `yaml:"sku-id"`
			} `yaml:"identity"`
		}
	}
}

// ReadConfigYAML downloads config.yaml from the DUT and find the firmware binary names that should be used.
func (fws *FirmwareService) ReadConfigYAML(ctx context.Context) error {
	out, err := fws.connection.RunCmd(ctx, "crosid", nil)
	if err != nil {
		return errors.Wrap(err, "failed to run crosid")
	}
	re, err := regexp.Compile(`^SKU='([^']*)'`)
	if err != nil {
		return errors.Wrap(err, "sku regex failed")
	}
	m := re.FindStringSubmatch(out)
	sku := -1
	if m != nil && m[1] != "none" {
		sku, err = strconv.Atoi(m[1])
		if err != nil {
			return errors.Wrapf(err, "parse of SKU %q failed", m[1])
		}
		log.Printf("DUT sku = %d", sku)
	}

	yamlPath := "/usr/share/chromeos-config/yaml/config.yaml"
	ok, err := fws.connection.PathExists(ctx, yamlPath)
	if err != nil {
		return errors.Wrap(err, "failed to check config.yaml")
	} else if !ok {
		log.Printf("No config.yaml found on DUT at %q", yamlPath)
		return nil
	}

	config, err := fws.connection.FetchFile(ctx, yamlPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read %q", yamlPath)
	}
	defer config.Close()
	configYaml := configData{}
	parser := yaml.NewDecoder(config)
	err = parser.Decode(&configYaml)
	if err != nil {
		return errors.Wrap(err, "failed to parse config.yaml")
	}
	model := fws.GetModel()
	for _, config := range configYaml.ChromeOS.Configs {
		// Wilco devices have _signed suffixes on the model name.
		if config.Name == model || config.Name == model+"_signed" {
			// Treat config.Identity.SKUID <= 0 as a wildcard that matches any SKU.
			if sku >= 0 && config.Identity.SKUID > 0 && config.Identity.SKUID != sku {
				log.Printf("Incorrect SKU: sku=%+v config.Identity.SKUID=%+v", sku, config.Identity.SKUID)
				continue
			}
			log.Printf("config entry: %+v", config)
			thisAPName := config.Firmware.BuildTargets.Coreboot
			if thisAPName == "" {
				thisAPName = config.Firmware.ImageName
			}
			if thisAPName != "" {
				if fws.CorebootName != "" && fws.CorebootName != thisAPName {
					return errors.Errorf("ambiguous AP name for model %q sku %d, could be %q or %q", model, sku, fws.CorebootName, thisAPName)
				}
				fws.CorebootName = thisAPName
			}
			var thisECName string
			var legacyECName string
			// Bizarrely, zephyr builders use the coreboot name for the ec.bin file.
			if config.Firmware.BuildTargets.ZephyrEC != "" {
				thisECName = config.Firmware.BuildTargets.ZephyrEC
				legacyECName = thisAPName
			} else {
				thisECName = config.Firmware.BuildTargets.EC
				legacyECName = config.Firmware.BuildTargets.EC
			}
			if thisECName != "" {
				if fws.StandaloneECName != "" && fws.StandaloneECName != thisECName {
					return errors.Errorf("ambiguous EC name for model %q sku %d, could be %q or %q", model, sku, fws.StandaloneECName, thisECName)
				}
				fws.StandaloneECName = thisECName
			}
			if legacyECName != "" {
				if fws.LegacyECName != "" && fws.LegacyECName != legacyECName {
					return errors.Errorf("ambiguous legacy EC name for model %q sku %d, could be %q or %q", model, sku, fws.LegacyECName, legacyECName)
				}
				fws.LegacyECName = legacyECName
			}
		}
	}
	// Special case for reef boards. See b/398900326
	if fws.CorebootName != fws.LegacyECName && fws.LegacyECName == "reef" {
		log.Printf("Overriding EC name to '%q", fws.CorebootName)
		fws.LegacyECName = fws.CorebootName
	}
	log.Printf("config.yaml image names AP: %s EC(legacy): %s EC(standalone): %s", fws.CorebootName, fws.LegacyECName, fws.StandaloneECName)
	return nil
}

// DownloadAndProcess gets ready to extract the selected archive.
// It doesn't actually do any downloading and should be renamed.
func (fws *FirmwareService) DownloadAndProcess(ctx context.Context, gsPath string) error {
	connection := fws.GetConnectionToFlashingDevice()
	if _, alreadyProcessed := fws.imagesMetadata[gsPath]; !alreadyProcessed {
		// Infer names for the local files and folders from basename of gsPath.
		archiveFilename := filepath.Base(gsPath)

		// Try to get a descriptive name for the temporary folder.
		archiveSubfolder := archiveFilename
		if strings.HasPrefix(archiveSubfolder, "firmware_from_source") {
			splitGsPath := strings.Split(gsPath, "/")
			nameIdx := len(splitGsPath) - 2
			if nameIdx < 0 {
				nameIdx = 0
			}
			archiveSubfolder = splitGsPath[nameIdx]
		}

		// Use mktemp to safely create a unique temp directory in /var/tmp so that it survives reboots.
		archiveDir, err := connection.RunCmd(ctx, "mktemp", []string{"-d", "--tmpdir=/var/tmp", fmt.Sprintf("'cros-fw-provision.XXXXXXXXX.%s'", archiveSubfolder)})
		if err != nil {
			return errors.Wrap(err, "remote mktemp failed")
		}
		archiveDir = strings.Trim(archiveDir, "\n")

		fws.imagesMetadata[gsPath] = ImageArchiveMetadata{
			ArchiveDir: archiveDir,
		}
	}
	return nil
}

// GetImageMetadata returns (ImageArchiveMetadata, IsImageMetadataPresent)
func (fws *FirmwareService) GetImageMetadata(gspath string) (ImageArchiveMetadata, bool) {
	metadata, ok := fws.imagesMetadata[gspath]
	return metadata, ok
}

// ProvisionWithFlashEC flashes EC image using flash_ec script.
func (fws *FirmwareService) ProvisionWithFlashEC(ctx context.Context, ecImage, flashECScriptPath string) error {
	customBitbangRate := ""
	if fws.ecChip == "stm32" {
		customBitbangRate = "--bitbang_rate=57600"
	}
	flashCmdArgs := fmt.Sprintf("--ro --chip=%s --board=%s --image='%s' --port=%v %s --verify --verbose",
		fws.ecChip, fws.model, ecImage, fws.servoPort, customBitbangRate)
	output, err := fws.GetConnectionToFlashingDevice().RunCmd(ctx, flashECScriptPath, strings.Split(flashCmdArgs, " "))
	if err != nil {
		log.Println(output)
		return err
	}
	return nil
}
