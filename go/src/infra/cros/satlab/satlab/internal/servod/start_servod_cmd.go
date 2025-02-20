// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package servod

import (
	"context"
	"fmt"
	"strings"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cmd/shivas/utils"
	"go.chromium.org/infra/cmdsupport/cmdlib"
	"go.chromium.org/infra/cros/recovery/docker"
	"go.chromium.org/infra/cros/satlab/common/satlabcommands"
	"go.chromium.org/infra/cros/satlab/common/services/ufs"
	"go.chromium.org/infra/cros/satlab/common/site"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/lab"
	ufsApi "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// StartServodCmd is the command that will start a servod container
var StartServodCmd = &subcommands.Command{
	UsageLine: "start -host <hostname> [options ...]",
	ShortDesc: "starts servod container",
	LongDesc:  "Starts servod container",
	CommandRun: func() subcommands.CommandRun {
		c := &startServodRun{}

		c.Flags.StringVar(&c.host, "host", "", "Hostname of DUT")
		c.Flags.StringVar(&c.board, "board", "", "Board of DUT")
		c.Flags.StringVar(&c.model, "model", "", "Model of DUT")
		c.Flags.StringVar(&c.servoSerial, "servo-serial", "", "Servo Serial of DUT")
		c.Flags.StringVar(&c.servodContainerName, "servod-container-name", "", "Container name to run servod in; likely <host>-docker_servod")
		c.Flags.StringVar(&c.subrailConfig, "subrail-config", "", "power subrail config file")
		c.Flags.BoolVar(&c.noServodProcess, "no-servod", false, "Start container without the servod process running")
		c.Flags.StringVar(&c.servoSetup, "servo-setup", "", "Servo setup of DUT; Should not have 'SERVO_SETUP' prefix (ex. use 'dual_v4' rather than 'SERVO_SETUP_DUAL_V4'")
		c.Flags.BoolVar(&c.useRecMode, "rec-mode", false, "Start servod with REC_MODE=1 which allowed to sart servod without CCD/OCD.")
		c.Flags.StringVar(&c.dockerTag, "docker-tag", "", "Specify custom servod tag when start container. Default read from SERVOD_CONTAINER_LABEL env or use 'release'.")

		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.commonFlags.Register(&c.Flags)

		return c
	},
}

// startServodRun struct contains the arguments for the servod command
type startServodRun struct {
	subcommands.CommandRunBase

	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags

	host string

	board               string
	model               string
	servoSerial         string
	servodContainerName string
	subrailConfig       string
	noServodProcess     bool
	servoSetup          string
	useRecMode          bool
	dockerTag           string
}

// Run is what is called when a user inputs the startServodRun command
func (c *startServodRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

// innerRun handles all orchestration needed to pass appropriate clients, contexts, and commands into the application
// this pattern is done to facilitate easy testing of the `runOrchestratedCommand` function
func (c *startServodRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)
	ctx = utils.SetupContext(ctx, c.envFlags.GetNamespace())
	dhbSatlabID := c.commonFlags.SatlabID
	if dhbSatlabID == "" {
		var err error
		dhbSatlabID, err = satlabcommands.GetDockerHostBoxIdentifier(ctx, &executor.ExecCommander{})
		if err != nil {
			return err
		}
	}

	if err := c.validate(dhbSatlabID, args); err != nil {
		return err
	}

	ef := c.envFlags
	ufs, err := ufs.NewUFSClient(ctx, ef.GetUFSService(), &c.authFlags)
	if err != nil {
		cmdlib.PrintError(a, errors.Reason("Error connecting to UFS: %s", err).Err())
		return err
	}

	d, err := docker.NewClient(ctx)
	if err != nil {
		cmdlib.PrintError(a, errors.Reason("Error creating docker client: %s", err).Err())
		return err
	}

	return c.runOrchestratedCommand(ctx, d, ufs)
}

// runOrchestratedCommand contains the actual logic behind this command. It does two things
//  1. if the user does not provide all arguments needed to start the docker container, fetches from UFS
//  2. execute the docker command with the information either provided by the user or from UFS
func (c *startServodRun) runOrchestratedCommand(ctx context.Context, d DockerClient, ufs ufs.UFSClient) error {
	servoSetupEnum, err := getServoSetupEnum(c.servoSetup)
	if err != nil {
		return err
	}

	opts := ServodContainerOptions{
		containerName: c.servodContainerName,
		board:         c.board,
		model:         c.model,
		servoSerial:   c.servoSerial,
		withServod:    !c.noServodProcess, // notice negation here
		servoSetup:    servoSetupEnum,
		useRecMode:    c.useRecMode,
		dockerTag:     c.dockerTag,
	}

	// If user provides all needed data, we can skip the UFS fetch entirely which has utility for a DUT not deployed in UFS or when UFS is unreachable
	if err := opts.Validate(); err != nil {
		ufsMetadata, err := fetchMetadataFromUFS(ctx, ufs, c.host, &c.authFlags)

		if c.commonFlags.Verbose {
			fmt.Printf("Fetched metadata from UFS, recieved %+v", ufsMetadata)
		}

		if err != nil {
			return errors.Reason("Failed to fetch metadata from UFS. If all of -board, -model, -servo-serial, -servo-hostname are provided UFS fetch will be skipped: %v", err).Err()
		}

		// If partial data is provided (ex. subset of needed args) then we fetch all from UFS but only override variables not given
		if opts.board == "" {
			opts.board = ufsMetadata.board
		}
		if opts.model == "" {
			opts.model = ufsMetadata.model
		}
		if opts.servoSerial == "" {
			opts.servoSerial = ufsMetadata.servoSerial
		}
		if opts.containerName == "" {
			opts.containerName = ufsMetadata.servodContainerName
		}
		if opts.subrailConfig == "" {
			opts.subrailConfig = ufsMetadata.subrailConfig
		}
		// checking against the command itself rather than options because we always have a value in opts
		// and we only want to replace with UFS if the user passes nothing
		if c.servoSetup == "" {
			opts.servoSetup = ufsMetadata.servoSetup
		}

		// If any field still is "", means that both user input and UFS fetch did not provide the appropriate field
		if err := opts.Validate(); err != nil {
			return errors.Reason("Fetch from UFS had empty fields, indicating data we need is not in UFS. Please specific args like -board, -model, manually").Err()
		}
	}

	dockerArgs := buildServodContainerArgs(opts)
	if c.commonFlags.Verbose {
		fmt.Printf("Attempting to launch container with command:\n\t%s\n", docker.StartCommandString(opts.containerName, dockerArgs))
	}
	_, err = startServodContainer(ctx, d, opts.containerName, dockerArgs)

	if err != nil {
		return errors.Reason(fmt.Sprintf("Error launching docker container: %s", err)).Err()
	}

	return nil
}

// fetchMetadataFromUFS pulls information about the asset and DUT of a given host
func fetchMetadataFromUFS(ctx context.Context, ufsClient ufs.UFSClient, host string, authFlags *authcli.Flags) (ufsMetadata, error) {
	dut, err := ufsClient.GetMachineLSE(ctx, &ufsApi.GetMachineLSERequest{
		Name: ufsUtil.AddPrefix(ufsUtil.MachineLSECollection, host),
	})
	if err != nil {
		return ufsMetadata{}, errors.Reason("Error fetching DUT %s from UFS: %s", host, err).Err()
	}

	servo := dut.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo()
	subrailConfig := dut.GetChromeosMachineLse().GetDeviceLse().GetDut().GetSubrailConfig()
	servoSerial := servo.GetServoSerial()
	servodContainerName := servo.GetDockerContainerName()
	servoSetup := servo.GetServoSetup()

	if len(dut.GetMachines()) == 0 {
		return ufsMetadata{}, errors.Reason("Fetched DUT %s has no machineId", host).Err()
	}
	machineId := dut.GetMachines()[0]

	machine, err := ufsClient.GetMachine(ctx, &ufsApi.GetMachineRequest{
		Name: ufsUtil.AddPrefix(ufsUtil.MachineCollection, machineId),
	})
	if err != nil {
		return ufsMetadata{}, errors.Reason("Error fetching machine %s from UFS: %s", host, err).Err()
	}

	model := machine.GetChromeosMachine().GetModel()
	board := machine.GetChromeosMachine().GetBuildTarget()

	return ufsMetadata{board: board, model: model, servoSerial: servoSerial, servodContainerName: servodContainerName, subrailConfig: subrailConfig, servoSetup: servoSetup}, nil
}

// ufsMetadata is bag of data for fields we want to extract from UFS
type ufsMetadata struct {
	board               string
	model               string
	servoSerial         string
	servodContainerName string
	subrailConfig       string
	servoSetup          ufspb.ServoSetupType
}

// validate validates input arguments
// We primarily care that a) host is not empty and b) host is in the right format (satlab-<dhb_id>-<host>)
func (c *startServodRun) validate(dhbSatlabID string, positionalArgs []string) error {
	// ensures we did not recieve positional args
	if len(positionalArgs) > 0 {
		return errors.Reason("Got unexpected positional args, for usage see: satlab servo help start").Err()
	}

	// Ensures the host or required field is provided.
	if c.host == "" && (c.board == "" || c.servoSerial == "" || c.servodContainerName == "") {
		return errors.Reason("-host <hostname> is required").Err()
	}

	c.host = site.GetFullyQualifiedHostname(c.commonFlags.SatlabID, dhbSatlabID, site.Satlab, c.host)

	return nil
}

// getServoSetupEnum takes a human readable string and returns the ServoSetupType enum.
// It does so by taking the input string, converting to capitals, and prefixing with SERVO_SETUP_.
// So "dual_v4" -> SERVO_SETUP_DUAL_V4, or "SERVO_SETUP_REGULAR" -> "SERVO_SETUP_SERVO_SETUP_REGULAR".
func getServoSetupEnum(servoSetupString string) (ufspb.ServoSetupType, error) {
	if servoSetupString == "" {
		return ufspb.ServoSetupType_SERVO_SETUP_REGULAR, nil
	}

	formatted_enum := fmt.Sprintf("SERVO_SETUP_%s", strings.ToUpper(servoSetupString))

	enum_val, ok := ufspb.ServoSetupType_value[formatted_enum]
	if !ok {
		return ufspb.ServoSetupType_SERVO_SETUP_INVALID, errors.Reason("Invalid servo setup value: %s. It should be any value in ServoSetup enum servo.pb without the SERVO_SETUP_ prefix", servoSetupString).Err()
	}

	return ufspb.ServoSetupType(enum_val), nil
}

// hasEmptyString checks a list of strings for ""
// for readability
func hasEmptyString(args ...string) bool {
	for _, s := range args {
		if s == "" {
			return true
		}
	}

	return false
}
