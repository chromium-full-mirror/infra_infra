// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"
	swarmingv2 "go.chromium.org/luci/swarming/proto/api_v2"

	"go.chromium.org/infra/fleetconsole/internal/database/devicesdb"
	"go.chromium.org/infra/fleetconsole/internal/devicemanagerclient"
	"go.chromium.org/infra/fleetconsole/internal/site"
	"go.chromium.org/infra/fleetconsole/internal/ufsclient"
	"go.chromium.org/infra/unifiedfleet/api/ufsclients"
)

// CompareDevicesCommand gets all devices from ufs and device manager and compares them
var CompareDevicesCommand *subcommands.Command = &subcommands.Command{
	UsageLine: "compare-devices [options...]",
	ShortDesc: "Compares ufs and device manager devices",
	LongDesc:  "Gets all devices from ufs and device manager and compares them highlighting any differences. The output can be long so it's suggested to dump it to a file `compare-devices [options...] | tee output_file.out`",
	CommandRun: func() subcommands.CommandRun {
		c := &compareDevicesCommand{}
		c.Flags.BoolVar(&c.prod, "prod", false, `if set calls ufs and dm prod instead of dev`)
		c.Init()
		return c
	},
}

type compareDevicesCommand struct {
	site.Subcommand
	prod bool
}

func (c *compareDevicesCommand) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	err := c.innerRun(ctx, a, args, env)
	return c.Done(ctx, err)
}

func (c *compareDevicesCommand) innerRun(ctx context.Context, a subcommands.Application, args []string, env subcommands.Env) error {
	fmt.Println(time.Now().Format("2006 January 02 15:04 UTC"))

	swarmingURL := "https://chromeos-swarming.appspot.com"
	dmURL := devicemanagerclient.DMDevURL
	ufsURL := ufsclient.UfsDevURL
	if c.prod {
		dmURL = devicemanagerclient.DMProdURL
		ufsURL = ufsclient.UfsProdURL
	}

	deviceManagerClient, err := dmClient(ctx, dmURL, c.AuthFlags)
	if err != nil {
		return err
	}
	dmDevices, err := devicemanagerclient.GetAllDmDevices(
		ctx,
		&devicemanagerclient.Client{Leaser: deviceManagerClient},
	)
	if err != nil {
		return err
	}

	dmDevicesMap := make(map[string]*devicesdb.DeviceDAO)
	for _, d := range dmDevices {
		dbDevice := devicesdb.FromDeviceManagerDevice(d)
		dmDevicesMap[dbDevice.Id] = dbDevice
	}

	ufsClient, err := ufsclients.NewUFSClientFromCLI(ctx, ufsURL, &c.AuthFlags, nil)
	if err != nil {
		return err
	}
	ufsDevices, err := ufsclient.GetAllUfsDevices(ctx, ufsClient)
	if err != nil {
		return err
	}

	ufsDevicesMap := make(map[string]*devicesdb.DeviceDAO)
	for _, d := range ufsDevices {
		dbDevice := devicesdb.FromUfsDevice(d, "")
		ufsDevicesMap[dbDevice.Id] = dbDevice
	}

	swarmingClient, err := swarmingClient(ctx, swarmingURL, c.AuthFlags)
	if err != nil {
		return err
	}
	swarmingBots, err := swarmingClient.ListBots(ctx, []*swarmingv2.StringPair{
		{Key: "pool", Value: "ChromeOSSkylab"},
	})
	if err != nil {
		return err
	}

	swarmingDevicesMap := make(map[string]*devicesdb.DeviceDAO)
	for _, b := range swarmingBots {
		if swarmingDevice, err := devicesdb.FromSwarmingBots(b); err == nil {
			swarmingDevicesMap[swarmingDevice.Id] = swarmingDevice
		}
	}

	fmt.Printf("DM_count=%d, UFS_count=%d, Swarming_count=%d\n", len(dmDevices), len(ufsDevices), len(swarmingBots))

	var allDiffs []string
	for swarmingDutName := range swarmingDevicesMap {
		dmDevice := dmDevicesMap[swarmingDutName]
		ufsDevice := ufsDevicesMap[swarmingDutName]

		if dmDevice == nil && ufsDevice == nil {
			allDiffs = append(allDiffs, "In swarming missing from both "+swarmingDutName+"\n")
			continue
		}
		if dmDevice == nil && ufsDevice != nil {
			allDiffs = append(allDiffs, "In swarming missing just from DM "+swarmingDutName+"\n")
			continue
		}
		if ufsDevice == nil && dmDevice != nil {
			allDiffs = append(allDiffs, "In swarming missing just from UFS "+swarmingDutName+"\n")
			continue
		}
	}

	for id := range ufsDevicesMap {
		swarmingDevice := swarmingDevicesMap[id]
		if swarmingDevice == nil {
			allDiffs = append(allDiffs, "In ufs missing from swarming "+id+"\n")
			continue
		}
	}

	for id, ufsDevice := range ufsDevicesMap {
		swarmingDevice := swarmingDevicesMap[id]
		if swarmingDevice != nil {
			allDiffs = append(allDiffs, compareDevice(swarmingDevice, ufsDevice))
		}
	}

	slices.Sort(allDiffs)

	diffCount := 0
	missingCount := 0
	extraCount := 0
	correctCount := 0
	for _, diff := range allDiffs {
		if diff == "" {
			correctCount += 1
			continue
		}

		if strings.HasPrefix(diff, "In swarming missing just from UFS") || strings.HasPrefix(diff, "In swarming missing from both") {
			missingCount += 1
			continue
		}
		if strings.HasPrefix(diff, "In ufs missing from swarming") {
			extraCount += 1
			continue
		}

		if strings.HasPrefix(diff, "DIFF") {
			diffCount += 1
			continue
		}
	}

	extraUfsVsDm := 0
	for label := range ufsDevicesMap {
		dmDevice := dmDevicesMap[label]
		if dmDevice == nil {
			extraUfsVsDm += 1
		}
	}

	fmt.Println()
	fmt.Printf("Found %d devices with differences in labels between ufs and swarming\n", diffCount)
	fmt.Printf("Found %d in swarming missing from ufs\n", missingCount)
	fmt.Printf("Found %d in ufs missing from swarming\n", extraCount)
	fmt.Printf("Found %d in ufs missing from DM\n", extraUfsVsDm)
	fmt.Printf("Found %d correct\n", correctCount)

	fmt.Println()
	prettyMap, _ := json.MarshalIndent(allWrongLabelCount, "", "\t")
	fmt.Printf("Wrong labels count: %v\n", string(prettyMap))

	fmt.Println()
	for _, diff := range allDiffs {
		if diff == "" {
			continue
		}
		fmt.Println("-------------------------------------")
		fmt.Print(diff)
	}

	return nil
}

// Labels to ignore when checking for differences
var ignoreDiffs = []string{
	"version_info_os",
}

func compareDevice(swarmingDevice, ufsDevice *devicesdb.DeviceDAO) string {
	diffs := ""

	diffs += diffString("DutId", swarmingDevice.DutId, ufsDevice.DutId)
	// TODO?
	// diffs += diffString("Type", swarmingDevice.Type, ufsDevice.Type)

	if swarmingDevice.Address != nil && ufsDevice.Address != nil {
		diffs += diffString("Port", swarmingDevice.Address.Port, ufsDevice.Address.Port)
		diffs += diffString("Host", swarmingDevice.Address.Host, ufsDevice.Address.Host)
	} else {
		if swarmingDevice.Address == nil && ufsDevice.Address != nil {
			if ufsDevice.Address.Port != 0 && ufsDevice.Address.Host != "" {
				diffs += diffString("Address", swarmingDevice.Address, ufsDevice.Address)
			}
		}
		if swarmingDevice.Address != nil && ufsDevice.Address == nil {
			if swarmingDevice.Address.Port != 0 && swarmingDevice.Address.Host != "" {
				diffs += diffString("Address", swarmingDevice.Address, ufsDevice.Address)
			}
		}
	}

	if hasLabels(swarmingDevice) && !hasLabels(ufsDevice) {
		diffs += diffString("Labels", "has labels", "missing labels")
	} else if !hasLabels(swarmingDevice) && hasLabels(ufsDevice) {
		diffs += diffString("Labels", "missing labels", "has labels")
	} else {
		for label := range ufsDevice.DeviceSpec.Labels {
			if slices.Contains(ignoreDiffs, label) {
				continue
			}

			var swarmingValue string
			if swarmingDevice.DeviceSpec.Labels[label] != nil {
				values := swarmingDevice.DeviceSpec.Labels[label].Values
				slices.Sort(values)

				swarmingValue = strings.Join(values, ", ")
			} else {
				continue
			}

			ufsValue := "UNDEFINED"
			if ufsDevice.DeviceSpec.Labels[label] != nil {
				values := ufsDevice.DeviceSpec.Labels[label].Values
				slices.Sort(values)

				ufsValue = strings.Join(values, ", ")
			}

			diffs += diffString("label."+label, swarmingValue, ufsValue)
		}
	}

	if diffs != "" {
		diffs = diffString("legend", "Swarming", "UFS") + diffs
		diffs = "DIFF - " + swarmingDevice.Id + " -\n" + diffs
	}
	return diffs
}

var allWrongLabelCount = make(map[string]int)

func diffString(label string, s1, s2 any) string {
	if s1 == s2 {
		return ""
	}
	if label != "legend" {
		allWrongLabelCount[label] += 1
	}

	return fmt.Sprintf("%s: %v\t | \t %v\n", label, s1, s2)
}

func hasLabels(d *devicesdb.DeviceDAO) bool {
	return len(d.DeviceSpec.Labels) > 0
}
