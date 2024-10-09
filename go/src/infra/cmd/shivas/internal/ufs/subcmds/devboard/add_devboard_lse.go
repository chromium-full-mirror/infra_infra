// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devboard

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/maruel/subcommands"
	"google.golang.org/genproto/protobuf/field_mask"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/grpc/prpc"

	"infra/cmd/shivas/cmdhelp"
	"infra/cmd/shivas/site"
	"infra/cmd/shivas/utils"
	"infra/cmdsupport/cmdlib"
	ufspb "infra/unifiedfleet/api/v1/models"
	chromeosLab "infra/unifiedfleet/api/v1/models/chromeos/lab"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
	ufsUtil "infra/unifiedfleet/app/util"
)

// Regexp to enforce the input format
var servoHostPortRegexp = regexp.MustCompile(`^[a-zA-Z0-9\-\.]+:[0-9]+$`)

// defaultPools contains the list of pools used by default.
var defaultPools = []string{"DUT_POOL_QUOTA"}

// AddDevboardLSECmd adds a MachineLSE to the database.
var AddDevboardLSECmd = &subcommands.Command{
	UsageLine: "devboard-lse [options ...]",
	ShortDesc: "Add a devboard LSE",
	LongDesc:  cmdhelp.AddDevboardLSELongDesc,
	CommandRun: func() subcommands.CommandRun {
		c := &addDevboard{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.commonFlags.Register(&c.Flags)

		// Asset location fields
		c.Flags.StringVar(&c.zone, "zone", "", "Zone that the asset is in. "+cmdhelp.ZoneFilterHelpText)
		c.Flags.StringVar(&c.rack, "rack", "", "Rack that the asset is in.")

		// Devboard/MachineLSE common fields
		c.Flags.StringVar(&c.name, "name", "", "unique name for the devboard.")
		c.Flags.StringVar(&c.asset, "asset", "", "asset name for the machine.")
		c.Flags.StringVar(&c.servo, "servo", "", "servo hostname and port as hostname:port. (port is assigned by UFS if missing)")
		c.Flags.StringVar(&c.servoSerial, "servo-serial", "", "serial number for the servo. Can skip for Servo V3.")
		c.Flags.StringVar(&c.servoSetupType, "servo-setup", "", "servo setup type. Allowed values are "+cmdhelp.ServoSetupTypeAllowedValuesString()+", UFS assigns REGULAR if unassigned.")
		c.Flags.StringVar(&c.servoDockerContainerName, "servod-docker", "", "servod docker container name. Required if serovd is running on docker")
		c.Flags.Var(utils.CSVString(&c.pools), "pools", "comma separated pools assigned to the devboard. 'DUT_POOL_QUOTA' is used if nothing is specified")
		c.Flags.StringVar(&c.state, "state", "", cmdhelp.StateHelp)
		c.Flags.StringVar(&c.description, "desc", "", "description for the machine.")
		return c
	},
}

type addDevboard struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags

	// Asset location fields
	zone string
	rack string

	// Devboard/MachineLSE common fields
	name                     string
	asset                    string
	servo                    string
	servoSerial              string
	servoSetupType           string
	servoDockerContainerName string
	pools                    []string
	state                    string
	description              string
}

// devboardDeployUFSParams contains all the data that are needed for deployment of a single devboard.
type devboardDeployUFSParams struct {
	Devboard *ufspb.MachineLSE // MachineLSE of the devboard to be updated
}

func (c *addDevboard) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *addDevboard) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	if err := c.validateArgs(); err != nil {
		return err
	}
	ctx := cli.GetContext(a, c, env)
	ctx = utils.SetupContext(ctx, ufsUtil.OSNamespace)
	hc, err := cmdlib.NewHTTPClient(ctx, &c.authFlags)
	if err != nil {
		return err
	}

	e := c.envFlags.Env()
	if c.commonFlags.Verbose() {
		fmt.Printf("Using UFS service %s \n", e.UnifiedFleetService)
	}

	devboardParams, err := c.parseArgs()
	if err != nil {
		return err
	}

	ic := ufsAPI.NewFleetPRPCClient(&prpc.Client{
		C:       hc,
		Host:    e.UnifiedFleetService,
		Options: site.DefaultPRPCOptions(c.envFlags),
	})

	for _, param := range devboardParams {
		if len(param.Devboard.GetMachines()) == 0 {
			fmt.Printf("Failed to add devboard %s to UFS. It is not linked to any Asset(Machine).\n", param.Devboard.GetName())
			continue
		}
		if err := c.addDevboardToUFS(ctx, ic, param); err != nil {
			fmt.Printf("Failed to add devboard %s to UFS. %s\n", param.Devboard.GetName(), err.Error())
			// skip deployment
			continue
		}
	}
	return nil
}

func (c addDevboard) validateArgs() error {
	if c.asset == "" {
		return cmdlib.NewQuietUsageError(c.Flags, "Need asset name to create a devboard")
	}
	if c.servo != "" {
		// If the servo is not servo V3. Then servo serial is needed
		host, _, err := parseServoHostnamePort(c.servo)
		if err != nil {
			return err
		}
		if !ufsUtil.ServoV3HostnameRegex.MatchString(host) && c.servoSerial == "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Cannot skip servo serial. Not a servo V3 device.")
		}
	} else if c.servoSerial != "" || c.servoSetupType != "" || c.servoDockerContainerName != "" {
		return cmdlib.NewQuietUsageError(c.Flags, fmt.Sprintf("Wrong usage!!\nProvided extra servo details when servo hostname is not provided."))
	}
	if c.servoSetupType != "" {
		if _, ok := chromeosLab.ServoSetupType_value[appendServoSetupPrefix(c.servoSetupType)]; !ok {
			return cmdlib.NewQuietUsageError(c.Flags, "Invalid servo setup %s", c.servoSetupType)
		}
	}
	if c.zone != "" && !ufsUtil.IsUFSZone(ufsUtil.RemoveZonePrefix(c.zone)) {
		return cmdlib.NewQuietUsageError(c.Flags, "Invalid zone %s", c.zone)
	}
	return nil
}

func (c *addDevboard) parseArgs() ([]*devboardDeployUFSParams, error) {
	// command line parameters
	devboardParams, err := c.initializeLSEAndAsset()
	if err != nil {
		return nil, err
	}
	return []*devboardDeployUFSParams{devboardParams}, nil
}

func (c *addDevboard) addDevboardToUFS(ctx context.Context, ic ufsAPI.FleetClient, param *devboardDeployUFSParams) error {
	if !ufsUtil.ValidateTags(param.Devboard.Tags) {
		return fmt.Errorf(ufsAPI.InvalidTags)
	}
	res, err := ic.CreateMachineLSE(ctx, &ufsAPI.CreateMachineLSERequest{
		MachineLSE:   param.Devboard,
		MachineLSEId: param.Devboard.GetName(),
	})
	if err != nil {
		fmt.Printf("Failed to add devboard %s to UFS. UFS add failed %s\n", param.Devboard.GetName(), err)
		return err
	}
	res.Name = ufsUtil.RemovePrefix(res.Name)
	utils.PrintProtoJSON(res, !utils.NoEmitMode(false))
	fmt.Printf("Successfully added devboard to UFS: %s \n", res.GetName())
	return nil
}

func (c *addDevboard) initializeLSEAndAsset() (*devboardDeployUFSParams, error) {
	devboard := &ufspb.ChromeOSDeviceLSE_Devboard{
		Devboard: &chromeosLab.Devboard{},
	}
	lse := &ufspb.MachineLSE{
		Lse: &ufspb.MachineLSE_ChromeosMachineLse{
			ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
				ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
					DeviceLse: &ufspb.ChromeOSDeviceLSE{
						Device: devboard,
					},
				},
			},
		},
	}
	var name, servoHost, servoSerial string
	var pools, machines []string
	var servoPort int32
	var servoSetup chromeosLab.ServoSetupType
	resourceState := ufsUtil.ToUFSState(c.state)
	{
		// comm and line parameters
		name = c.name
		var err error
		if c.servo != "" {
			servoHost, servoPort, err = parseServoHostnamePort(c.servo)
			if err != nil {
				return nil, err
			}
			servoSerial = c.servoSerial
			servoSetup = chromeosLab.ServoSetupType(chromeosLab.ServoSetupType_value[appendServoSetupPrefix(c.servoSetupType)])
		}
		machines = []string{c.asset}
		pools = c.pools
	}
	lse.Name = name
	// Hostname must be set to validate the proto even though
	// devboards don't have hostnames.
	lse.Hostname = name
	lse.Machines = machines

	// Use the input params if available for all the options.
	lse.Description = c.description
	lse.ResourceState = resourceState

	servo := &chromeosLab.Servo{}
	lse.GetChromeosMachineLse().GetDeviceLse().GetDevboard().Servo = servo
	if servoHost != "" {
		// if servo-host is not provided then do not set any servo field.
		servo.ServoHostname = servoHost
		servo.ServoPort = servoPort
		servo.ServoSerial = servoSerial
		servo.ServoSetup = servoSetup
		servo.DockerContainerName = c.servoDockerContainerName
	}
	if len(pools) > 0 && pools[0] != "" {
		lse.GetChromeosMachineLse().GetDeviceLse().GetDevboard().Pools = pools
	} else {
		lse.GetChromeosMachineLse().GetDeviceLse().GetDevboard().Pools = defaultPools
	}
	return &devboardDeployUFSParams{
		Devboard: lse,
	}, nil
}

func parseServoHostnamePort(servo string) (string, int32, error) {
	var servoHostname string
	var servoPort int32
	servoHostnamePort := strings.Split(servo, ":")
	if len(servoHostnamePort) == 2 {
		servoHostname = servoHostnamePort[0]
		port, err := strconv.ParseInt(servoHostnamePort[1], 10, 32)
		if err != nil {
			return "", int32(0), err
		}
		servoPort = int32(port)
	} else {
		servoHostname = servoHostnamePort[0]
	}
	return servoHostname, servoPort, nil
}

func appendServoSetupPrefix(servoSetup string) string {
	return fmt.Sprintf("SERVO_SETUP_%s", servoSetup)
}

// updateAssetToUFS calls UpdateAsset API in UFS with asset and partial paths
func (c *addDevboard) updateAssetToUFS(ctx context.Context, ic ufsAPI.FleetClient, asset *ufspb.Asset, paths []string) error {
	if len(paths) == 0 {
		// If no update is available. Skip doing anything
		return nil
	}
	mask := &field_mask.FieldMask{
		Paths: paths,
	}
	_, err := ic.UpdateAsset(ctx, &ufsAPI.UpdateAssetRequest{
		Asset:      asset,
		UpdateMask: mask,
	})
	return err
}
