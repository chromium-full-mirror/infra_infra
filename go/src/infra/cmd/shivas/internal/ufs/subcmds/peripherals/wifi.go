// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package peripherals

import (
	"context"
	"fmt"
	"strings"

	"github.com/maruel/subcommands"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"

	"go.chromium.org/infra/cmd/shivas/cmdhelp"
	"go.chromium.org/infra/cmd/shivas/site"
	"go.chromium.org/infra/cmd/shivas/utils"
	"go.chromium.org/infra/cmdsupport/cmdlib"
	lab "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/lab"
	rpc "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

var (
	AddPeripheralWifiCmd     = wifiCmd(actionAdd)
	ReplacePeripheralWifiCmd = wifiCmd(actionReplace)
	DeletePeripheralWifiCmd  = wifiCmd(actionDelete)
)

var csvHeaderMap = map[string]bool{
	"dut":    true,
	"router": true,
}

// wifiCmd creates command for adding, removing, or completely replacing routers on a DUT.
func wifiCmd(mode action) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "peripheral-wifi -dut {DUT name} -router {hostname:h1,model:m1} [-router {hostname:hn,...}...]",
		ShortDesc: "Manage wifi router connections to a DUT",
		LongDesc:  cmdhelp.ManagePeripheralWifiLongDesc,
		CommandRun: func() subcommands.CommandRun {
			c := manageWifiCmd{mode: mode}
			c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
			c.envFlags.Register(&c.Flags)
			c.commonFlags.Register(&c.Flags)

			c.Flags.StringVar(&c.dutName, "dut", "", "DUT name to update")
			c.Flags.Var(utils.CSVStringList(&c.routers), "router", "comma separated router info. require hostname:h1")
			c.Flags.StringVar(&c.wifiFile, "f", "", "File path to csv or json file. Note: Can only use in replace action. json file replaces the whole wifi proto, csv file replace multiple duts")

			return &c
		},
	}
}

// manageWifiCmd supports adding, replacing, or deleting routers.
type manageWifiCmd struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags

	dutName    string
	routers    [][]string
	routersMap map[string]map[string]*lab.WifiRouter // set of WifiRouter

	wifiFile            string
	wifiJsonFileWifiObj *lab.Wifi
	isCSVUpdate         bool
	isJsonUpdate        bool
	mode                action
}

// Run executed the wifi management subcommand. It cleans up passed flags and validates them.
func (c *manageWifiCmd) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.run(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

// run implements the core logic for Run.
func (c *manageWifiCmd) run(a subcommands.Application, args []string, env subcommands.Env) error {
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

	if c.isCSVUpdate {
		// only running in replacing mode. checked in cleanAndValidateFlag
		for dutName := range c.routersMap {
			err = c.runSingleDut(ctx, client, dutName)
			if err != nil {
				return errors.Annotate(err, "update csv error %s", dutName).Err()
			}
		}
	} else {
		return c.runSingleDut(ctx, client, c.dutName)
	}

	return nil
}

func (c *manageWifiCmd) runSingleDut(ctx context.Context, client rpc.FleetClient, dutName string) error {
	lse, err := client.GetMachineLSE(ctx, &rpc.GetMachineLSERequest{
		Name: util.AddPrefix(util.MachineLSECollection, dutName),
	})
	if err != nil {
		return err
	}
	if err := utils.IsDUT(lse); err != nil {
		return errors.Annotate(err, "not a dut").Err()
	}

	var (
		peripherals = lse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals()
		currentWifi = peripherals.GetWifi()
	)
	nw, err := c.runWifiAction(currentWifi, dutName)
	if err != nil {
		return err
	}
	if c.commonFlags.Verbose() {
		fmt.Println("New Wifi", nw)
	}

	peripherals.Wifi = nw
	// TODO(b/226024082): Currently field masks are implemented in a limited way. Subsequent update
	// on UFS could add field masks for wifi and then they could be included here.
	_, err = client.UpdateMachineLSE(ctx, &rpc.UpdateMachineLSERequest{MachineLSE: lse})
	return err
}

// runWifiAction updates the routers in current based on the action specified in c.
// note: runWifiAction currently only modifies wifi.WifiRouters
func (c *manageWifiCmd) runWifiAction(current *lab.Wifi, dutName string) (*lab.Wifi, error) {
	switch c.mode {
	case actionAdd:
		return c.addWifiRouters(current, dutName)
	case actionReplace:
		if c.commonFlags.Verbose() {
			fmt.Println("Replacing", current)
		}
		return c.replaceWifiRouters(current, dutName)
	case actionDelete:
		return c.deleteWifiRouters(current, dutName)
	default:
		return nil, errors.Reason("unknown action %d", c.mode).Err()
	}
}

// replaceWifiRouters replaces routers in current with the routers in c.
func (c *manageWifiCmd) replaceWifiRouters(current *lab.Wifi, dutName string) (*lab.Wifi, error) {
	if len(c.routersMap[dutName]) != 0 {
		current.WifiRouters = make([]*lab.WifiRouter, 0)
		for hostname := range c.routersMap[dutName] {
			current.WifiRouters = append(current.WifiRouters, c.routersMap[dutName][hostname])
		}
	}
	return current, nil
}

// addWifiRouters adds routers in c to the routers in current.
// It returns an error if a duplicate is specified.
func (c *manageWifiCmd) addWifiRouters(current *lab.Wifi, dutName string) (*lab.Wifi, error) {
	for _, router := range current.GetWifiRouters() {
		if _, ok := c.routersMap[dutName][router.GetHostname()]; ok {
			return nil, errors.Reason("wifi router %s already exists", router.GetHostname()).Err()
		}
	}
	for hostname := range c.routersMap[dutName] {
		current.WifiRouters = append(current.WifiRouters, c.routersMap[dutName][hostname])
	}
	return current, nil
}

// deleteWifiRouters deletes routers in c with routers in current that have the
// same hostname.
func (c *manageWifiCmd) deleteWifiRouters(current *lab.Wifi, dutName string) (*lab.Wifi, error) {
	currentRoutersMap := make(map[string]*lab.WifiRouter)
	for _, router := range current.GetWifiRouters() {
		currentRoutersMap[router.GetHostname()] = router
	}
	for hostname := range c.routersMap[dutName] {
		if _, ok := currentRoutersMap[hostname]; !ok {
			return nil, errors.Reason("wifi router %s does not exist", hostname).Err()
		}
		delete(currentRoutersMap, hostname)
	}
	current.WifiRouters = make([]*lab.WifiRouter, 0, len(currentRoutersMap))
	for hostname := range currentRoutersMap {
		current.WifiRouters = append(current.WifiRouters, currentRoutersMap[hostname])
	}
	return current, nil
}

const (
	errDuplicateDut           = "duplicate dut specified"
	errDuplicateModel         = "duplicate model specified"
	errDuplicateRouterFeature = "duplicate router feature specified"
	errInvalidRouterFeature   = "invalid router feature"
	errNoRouter               = "at least one -router required"
)

// cleanAndValidateFlags returns an error with the result of all validations. It strips whitespaces
// around hostnames and removes empty ones.
func (c *manageWifiCmd) cleanAndValidateFlags() error {
	var errStrs []string
	if c.routersMap == nil {
		c.routersMap = map[string]map[string]*lab.WifiRouter{}
	}
	if len(c.wifiFile) != 0 {
		if utils.IsCSVFile(c.wifiFile) {
			c.isCSVUpdate = true
			records, err := utils.ParseMCSVFile(c.wifiFile)
			if err != nil {
				return errors.Annotate(err, "parsing CSV file error").Err()
			}
			for i, rec := range records {
				if i == 0 {
					if len(rec) == 0 {
						return errors.Annotate(err, "header should not be empty").Err()
					}
					for _, key := range rec {
						if !csvHeaderMap[key] {
							return errors.Reason("invalid header %q", key).Err()
						}
					}
					continue
				}
				dut, routers := parseWifiCSVRow(records[0], rec)
				if err := c.validateSingleDut(dut, routers); err != nil {
					return errors.Annotate(err, "invalid input row number %d", i).Err()
				}
			}
			return nil

		} else {
			c.isJsonUpdate = true
			if len(c.dutName) == 0 {
				errStrs = append(errStrs, errDUTMissing)
			}
			if c.wifiJsonFileWifiObj == nil {
				c.wifiJsonFileWifiObj = &lab.Wifi{}
			}
			c.routersMap[c.dutName] = map[string]*lab.WifiRouter{}
			if err := utils.ParseJSONFile(c.wifiFile, c.wifiJsonFileWifiObj); err != nil {
				return errors.Annotate(err, "json parse error").Err()
			}
			for _, router := range c.wifiJsonFileWifiObj.WifiRouters {
				if hostname := strings.TrimSpace(router.Hostname); hostname == "" {
					return errors.Reason("invalid router hostname. %s", router).Err()
				} else {
					c.routersMap[c.dutName][hostname] = router
				}
			}
			return nil
		}
	} else {
		if err := c.validateSingleDut(c.dutName, c.routers); err != nil {
			return err
		}
		if len(errStrs) != 0 {
			return cmdlib.NewQuietUsageError(c.Flags, fmt.Sprintf("Wrong usage!!\n%s", strings.Join(errStrs, "\n")))
		}
		return nil
	}
}

func parseWifiCSVRow(header []string, row []string) (dut string, routers [][]string) {
	for i, headerKey := range header {
		rowField := strings.ToLower(strings.TrimSpace(row[i]))
		switch headerKey {
		case "dut":
			dut = rowField
		case "router":
			if rowField != "" {
				routers = append(routers, strings.Split(rowField, ";"))
			}
		default:
			fmt.Println("shouldn't be in here since header fields are checked. add cases that is in csvHeaderMap")
		}
	}
	return dut, routers
}

// validateSingleDut validate input for single dut. Use for multiple row update and single cli input
func (c *manageWifiCmd) validateSingleDut(dutName string, routersInput [][]string) error {
	var errStrs []string
	if len(dutName) == 0 {
		errStrs = append(errStrs, errDUTMissing)
	}
	if _, ok := c.routersMap[dutName]; ok {
		return errors.Reason("%s: %s", errDuplicateDut, dutName).Err()
	}

	c.routersMap[dutName] = map[string]*lab.WifiRouter{}
	for _, routerCSV := range routersInput {
		newRouter := &lab.WifiRouter{}
		newRouterFeaturesMap := make(map[labapi.WifiRouterFeature]bool)
		for _, keyValStr := range routerCSV {
			keyValList := strings.Split(keyValStr, ":")
			if len(keyValList) != 2 {
				errStrs = append(errStrs, fmt.Sprintf("Invalid key:val for router tag %q", keyValList))
			}
			key := strings.ToLower(strings.TrimSpace(keyValList[0]))
			val := strings.ToLower(strings.TrimSpace(keyValList[1]))
			switch key {
			case "hostname":
				if newRouter.GetHostname() != "" {
					errStrs = append(errStrs, errDuplicateHostname)
				}
				newRouter.Hostname = val
			case "model":
				if newRouter.GetModel() != "" {
					errStrs = append(errStrs, errDuplicateModel)
				}
				newRouter.Model = val
			case "supported_feature":
				val = strings.ToUpper(val)
				if fInt, ok := labapi.WifiRouterFeature_value[val]; !ok {
					errStrs = append(errStrs, fmt.Sprintf("%s: %q", errInvalidRouterFeature, val))
				} else {
					if newRouterFeaturesMap[labapi.WifiRouterFeature(fInt)] {
						errStrs = append(errStrs, errDuplicateRouterFeature)
					}
					newRouterFeaturesMap[labapi.WifiRouterFeature(fInt)] = true
				}
			default:
				errStrs = append(errStrs, fmt.Sprintf("unsupported router key: %q", key))
			}
		}
		if newRouter.GetHostname() == "" {
			errStrs = append(errStrs, errEmptyHostname)
			continue
		}
		for feature := range newRouterFeaturesMap {
			newRouter.SupportedFeatures = append(newRouter.SupportedFeatures, feature)
		}
		if _, ok := c.routersMap[dutName][newRouter.GetHostname()]; ok {
			errStrs = append(errStrs, errDuplicateHostname)
		}
		c.routersMap[dutName][newRouter.GetHostname()] = newRouter
	}
	if len(c.routersMap[dutName]) == 0 {
		errStrs = append(errStrs, errNoRouter)
	}
	if len(errStrs) != 0 {
		return cmdlib.NewQuietUsageError(c.Flags, fmt.Sprintf("Wrong usage!!\n%s", strings.Join(errStrs, "\n")))
	}
	return nil
}
