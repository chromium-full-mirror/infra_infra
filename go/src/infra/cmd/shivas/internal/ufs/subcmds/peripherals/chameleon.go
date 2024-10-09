// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package peripherals

import (
	"fmt"
	"strings"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/flag"
	"go.chromium.org/luci/grpc/prpc"

	"infra/cmd/shivas/cmdhelp"
	"infra/cmd/shivas/site"
	"infra/cmd/shivas/utils"
	"infra/cmdsupport/cmdlib"
	lab "infra/unifiedfleet/api/v1/models/chromeos/lab"
	rpc "infra/unifiedfleet/api/v1/rpc"
	"infra/unifiedfleet/app/util"
)

var (
	AddChameleonCmd     = chamCmd(actionAdd)
	ReplaceChameleonCmd = chamCmd(actionReplace)
	DeleteChameleonCmd  = chamCmd(actionDelete)
)

// chamCmd creates command for adding, removing, or completely replacing Chameleon on a DUT.
func chamCmd(mode action) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "chameleon -dut {DUT name} -hostname {chameleon hostname} ",
		ShortDesc: "Manage Chameleon connect to a DUT",
		LongDesc:  cmdhelp.ManageChameleonLongDesc,
		CommandRun: func() subcommands.CommandRun {
			c := manageChamCmd{mode: mode}
			c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
			c.envFlags.Register(&c.Flags)
			c.commonFlags.Register(&c.Flags)

			c.Flags.StringVar(&c.dutName, "dut", "", "DUT name to update")
			c.Flags.StringVar(&c.hostname, "hostname", "", "hostname for Chameleon")
			c.Flags.Var(flag.StringSlice(&c.types), "type", "type of chameleon, ie. v2")
			c.Flags.Var(flag.StringSlice(&c.connectionTypes), "connection_type", "connection type of chameleon, can be specified multiple times, ie. dp, hdmi")

			c.Flags.StringVar(&c.rpmHostname, "rpm", "", "hostname for rpm connected to chameleon")
			c.Flags.StringVar(&c.rpmOutlet, "rpm-outlet", "", "outlet number of rpm connected to chameleon")
			c.Flags.BoolVar(&c.audioBoard, "audio-board", false, "audio board chameleon")
			c.Flags.StringVar(&c.trrsTypeName, "trrs", "", "type of trrs, ie. CTIA or OMTP, default to original trrs value or CTIA when audio-cable is set on DUT")

			return &c
		},
	}
}

// manageChamCmd supports adding, replacing, or deleting Cham.
type manageChamCmd struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags

	dutName  string
	hostname string

	types          []string
	chameleonTypes []lab.ChameleonType
	typesMap       map[lab.ChameleonType]bool

	connectionTypes          []string
	chameleonConnectionTypes []lab.ChameleonConnectionType
	connectionTypesMap       map[lab.ChameleonConnectionType]bool

	rpmHostname  string
	rpmOutlet    string
	audioBoard   bool
	trrsTypeName string
	trrsType     lab.Chameleon_TRRSType

	mode action
}

// Run executed the Chameleon management subcommand. It cleans up passed flags and validates them.
func (c *manageChamCmd) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.run(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

// run implements the core logic for Run.
func (c *manageChamCmd) run(a subcommands.Application, args []string, env subcommands.Env) error {
	if err := c.cleanAndValidateFlags(); err != nil {
		return err
	}
	ctx := cli.GetContext(a, c, env)
	ctx = utils.SetupContext(ctx, util.OSNamespace)

	hc, err := cmdlib.NewHTTPClient(ctx, &c.authFlags)
	if err != nil {
		return err
	}
	e := c.envFlags.Env()
	if c.commonFlags.Verbose() {
		fmt.Printf("Using UFS service %s\n", e.UnifiedFleetService)
	}

	client := rpc.NewFleetPRPCClient(&prpc.Client{
		C:       hc,
		Host:    e.UnifiedFleetService,
		Options: site.DefaultPRPCOptions(c.envFlags),
	})

	lse, err := client.GetMachineLSE(ctx, &rpc.GetMachineLSERequest{
		Name: util.AddPrefix(util.MachineLSECollection, c.dutName),
	})
	if err != nil {
		return err
	}
	if err := utils.IsDUT(lse); err != nil {
		return errors.Annotate(err, "DUT name is not a chromeOS machine").Err()
	}

	var (
		peripherals = lse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals()
		currentCham = peripherals.GetChameleon()
	)

	c.trrsType = setDefaultTrrs(
		c.trrsType,
		currentCham.GetTrrsType(),
		peripherals.GetAudio().GetAudioCable(),
	)

	nb, err := c.newCham(currentCham)
	if err != nil {
		return err
	}
	if c.commonFlags.Verbose() {
		fmt.Println("New Chameleon list", nb)
	}

	peripherals.Chameleon = nb
	//TODO(b:258280356) Use UFS chameleon field mask to do atomic partial update for on dut
	_, err = client.UpdateMachineLSE(ctx, &rpc.UpdateMachineLSERequest{MachineLSE: lse})
	return err
}

// newCham returns a new list of Cham based on the action specified in c and current list.
func (c *manageChamCmd) newCham(current *lab.Chameleon) (*lab.Chameleon, error) {
	switch c.mode {
	case actionAdd:
		if current != nil && current.GetHostname() != "" {
			return nil, errors.Reason("There already exist chameleon %s", current.GetHostname()).Err()
		}
		return c.createChameleon(), nil
	case actionReplace:
		return c.createChameleon(), nil
	case actionDelete:
		if c.hostname != current.GetHostname() {
			return nil, errors.Reason("deleting hostname %s does not match existing one %s", c.hostname, current.GetHostname()).Err()
		}
		return nil, nil
	default:
		return nil, errors.Reason("unknown action %d", c.mode).Err()
	}
}

const (
	errEmptyType         = "empty type"
	errDuplicateType     = "duplicate type specified"
	errDuplicateConnType = "duplicate connection type specified"
	errInvalidTRRSType   = "invalid TRRS type specified"
)

// cleanAndValidateFlags returns an error with the result of all validations. It strips whitespaces
// around hostnames and removes empty ones.
func (c *manageChamCmd) cleanAndValidateFlags() error {
	var errStrs []string
	if len(c.dutName) == 0 {
		errStrs = append(errStrs, errDUTMissing)
	}
	c.hostname = strings.TrimSpace(c.hostname)
	if c.hostname == "" {
		errStrs = append(errStrs, errNoHostname)
	}

	if c.typesMap == nil {
		c.typesMap = map[lab.ChameleonType]bool{}
	}

	if c.connectionTypesMap == nil {
		c.connectionTypesMap = map[lab.ChameleonConnectionType]bool{}
	}

	var types []lab.ChameleonType
	for _, ts := range c.types {
		ts = strings.ToUpper(strings.TrimSpace(ts))

		// Empty Type
		if len(ts) == 0 {
			if c.commonFlags.Verbose() {
				fmt.Println("Empty type specified")
			}
			errStrs = append(errStrs, errEmptyType)
			continue
		}

		if !isChameleonType(ts) {
			errStrs = append(errStrs, fmt.Sprintf("Invalid chameleon type specified %s", ts))
			continue
		}
		t := toChameleonType(ts)

		// Deprecation warning
		if isDeprecatingChameleonType(t) {
			fmt.Println("Warning, type is pending deprecation:", ts)
		}

		// Duplicate Type
		if c.typesMap[t] {
			if c.commonFlags.Verbose() {
				fmt.Println("Duplicate type specified:", ts)
			}
			errStrs = append(errStrs, fmt.Sprintf("%s: %s", errDuplicateType, ts))
			continue
		}
		c.typesMap[t] = true
		types = append(types, t)
	}
	c.chameleonTypes = types

	var connectionTypes []lab.ChameleonConnectionType
	for _, ts := range c.connectionTypes {
		ts = strings.ToUpper(strings.TrimSpace(ts))

		// Empty Type
		if len(ts) == 0 {
			if c.commonFlags.Verbose() {
				fmt.Println("Empty connection type specified")
			}
			errStrs = append(errStrs, errEmptyType)
			continue
		}

		if !isChameleonConnectionType(ts) {
			errStrs = append(errStrs, fmt.Sprintf("Invalid chameleon connection type specified %s", ts))
			continue
		}
		t := toChameleonConnectionType(ts)

		// Duplicate Type
		if c.connectionTypesMap[t] {
			if c.commonFlags.Verbose() {
				fmt.Println("Duplicate connection type specified:", ts)
			}
			errStrs = append(errStrs, fmt.Sprintf("%s: %s", errDuplicateConnType, ts))
			continue
		}
		c.connectionTypesMap[t] = true
		connectionTypes = append(connectionTypes, t)
	}
	c.chameleonConnectionTypes = connectionTypes

	if c.trrsTypeName != "" {

		c.trrsTypeName = strings.ToUpper(c.trrsTypeName)

		if trrsVal, ok := lab.Chameleon_TRRSType_value["TRRS_TYPE_"+c.trrsTypeName]; ok {
			c.trrsType = lab.Chameleon_TRRSType(trrsVal)
		} else {
			errStrs = append(
				errStrs,
				fmt.Sprintf(
					"%s (%s), only supports (%s)",
					errInvalidTRRSType,
					c.trrsTypeName,
					strings.Join(supportedTrrsTypes(), ","),
				),
			)
		}

	}

	if (c.rpmHostname != "" && c.rpmOutlet == "") || (c.rpmHostname == "" && c.rpmOutlet != "") {
		errStrs = append(errStrs, fmt.Sprintf("Need both rpm and its outlet. %s:%s is invalid", c.rpmHostname, c.rpmOutlet))
	}
	if len(errStrs) == 0 {
		return nil
	}

	return cmdlib.NewQuietUsageError(c.Flags, fmt.Sprintf("Wrong usage!!\n%s", strings.Join(errStrs, "\n")))
}

// createChameleon creates a *lab.Chameleon object from manageChamCmd variables.
func (c *manageChamCmd) createChameleon() *lab.Chameleon {
	ret := &lab.Chameleon{
		Hostname:                 c.hostname,
		ChameleonPeripherals:     c.chameleonTypes,
		ChameleonConnectionTypes: c.chameleonConnectionTypes,
		AudioBoard:               c.audioBoard,
		TrrsType:                 c.trrsType,
	}
	if c.rpmHostname != "" {
		ret.Rpm = &lab.OSRPM{
			PowerunitName:   c.rpmHostname,
			PowerunitOutlet: c.rpmOutlet,
		}
	}
	return ret
}

func isChameleonType(s string) bool {
	if val, ok := lab.ChameleonType_value[fmt.Sprintf("CHAMELEON_TYPE_%s", s)]; ok {
		return val != 0
	}
	return false
}

func toChameleonType(s string) lab.ChameleonType {
	if isChameleonType(s) {
		return lab.ChameleonType(lab.ChameleonType_value[fmt.Sprintf("CHAMELEON_TYPE_%s", s)])
	}
	return lab.ChameleonType_CHAMELEON_TYPE_INVALID
}

func isDeprecatingChameleonType(s lab.ChameleonType) bool {
	return s == lab.ChameleonType_CHAMELEON_TYPE_DP || s == lab.ChameleonType_CHAMELEON_TYPE_HDMI
}

func isChameleonConnectionType(s string) bool {
	if val, ok := lab.ChameleonConnectionType_value[fmt.Sprintf("CHAMELEON_CONNECTION_TYPE_%s", s)]; ok {
		return val != 0
	}
	return false
}

func toChameleonConnectionType(s string) lab.ChameleonConnectionType {
	if isChameleonConnectionType(s) {
		return lab.ChameleonConnectionType(lab.ChameleonConnectionType_value[fmt.Sprintf("CHAMELEON_CONNECTION_TYPE_%s", s)])
	}
	return lab.ChameleonConnectionType_CHAMELEON_CONNECTION_TYPE_INVALID
}

func supportedTrrsTypes() []string {
	supportedTrrsTypes := []string{}
	const plen = len("TRRS_TYPE_")
	for key := range lab.Chameleon_TRRSType_value {
		key = key[plen:]
		if key == "UNSPECIFIED" {
			continue
		}
		supportedTrrsTypes = append(supportedTrrsTypes, key)
	}
	return supportedTrrsTypes
}

func setDefaultTrrs(
	cliTrrs lab.Chameleon_TRRSType, // shivas -trrs
	originalTrrs lab.Chameleon_TRRSType, // from UFS
	hasAudioCable bool, // from UFS
) lab.Chameleon_TRRSType {
	if !hasAudioCable {
		return lab.Chameleon_TRRS_TYPE_UNSPECIFIED
	}
	if cliTrrs != lab.Chameleon_TRRS_TYPE_UNSPECIFIED {
		return cliTrrs
	}
	if originalTrrs != lab.Chameleon_TRRS_TYPE_UNSPECIFIED {
		return originalTrrs // preserve original TRRS if not provided
	}
	return lab.Chameleon_TRRS_TYPE_CTIA
}
