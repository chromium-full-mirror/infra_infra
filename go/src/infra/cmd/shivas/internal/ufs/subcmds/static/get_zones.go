// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package static

import (
	"strings"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"

	"go.chromium.org/infra/cmd/shivas/cmdhelp"
	"go.chromium.org/infra/cmd/shivas/site"
	"go.chromium.org/infra/cmd/shivas/utils"
	"go.chromium.org/infra/cmdsupport/cmdlib"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// GetZonesCmd get/list zones offered by UFS.
var GetZonesCmd = &subcommands.Command{
	UsageLine: "zones",
	ShortDesc: "Get all the zones offered by UFS",
	LongDesc: `Get all the zones offered by UFS

Example:

shivas get zones`,
	CommandRun: func() subcommands.CommandRun {
		c := &getZones{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.commonFlags.Register(&c.Flags)
		c.outputFlags.Register(&c.Flags)
		c.Flags.BoolVar(&c.keysOnly, "keys", false, cmdhelp.KeysOnlyText)
		return c
	},
}

type getZones struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	commonFlags site.CommonFlags
	outputFlags site.OutputFlags

	keysOnly bool
}

type zone struct {
	Name       string `json:"Name"`
	EnumName   string `json:"EnumName"`
	Department string `json:"Department"`
}

type noEmitZone struct {
	Name       string `json:"Name,omitempty"`
	EnumName   string `json:"EnumName,omitempty"`
	Department string `json:"Department,omitempty"`
}

func (c *getZones) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *getZones) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	if c.outputFlags.JSON() {
		emit := !utils.NoEmitMode(c.outputFlags.NoEmit())
		if emit {
			return utils.PrintJSON(getEmitZones(c.keysOnly))
		}
		return utils.PrintJSON(getNoEmitZones(c.keysOnly))
	}

	res := outputZoneStr(getEmitZones(c.keysOnly), c.keysOnly)
	if c.outputFlags.Tsv() {
		utils.PrintAllTSVs(res)
	} else {
		utils.PrintAllNormal(utils.ZoneTitle, res, c.keysOnly)
	}
	return nil
}

func getEmitZones(keysOnly bool) []*zone {
	zones := make([]*zone, 0)
	for zoneValue := range ufspb.Zone_value {
		if !strings.Contains(zoneValue, "UNSPECIFIED") {
			z := &zone{
				Name: strings.ToLower(ufsUtil.GetSuffixAfterSeparator(zoneValue, "_")),
			}
			if !keysOnly {
				z.EnumName = zoneValue
				z.Department = ufsUtil.ToUFSDept(zoneValue)
			}
			zones = append(zones, z)
		}
	}
	return zones
}

func getNoEmitZones(keysOnly bool) []*noEmitZone {
	zones := make([]*noEmitZone, 0)
	for zoneValue := range ufspb.Zone_value {
		if !strings.Contains(zoneValue, "UNSPECIFIED") {
			z := &noEmitZone{
				Name: strings.ToLower(ufsUtil.GetSuffixAfterSeparator(zoneValue, "_")),
			}
			if !keysOnly {
				z.EnumName = zoneValue
				z.Department = ufsUtil.ToUFSDept(zoneValue)
			}
			zones = append(zones, z)
		}
	}
	return zones
}

func outputZoneStr(zones []*zone, keysOnly bool) [][]string {
	res := make([][]string, len(zones))
	for i := 0; i < len(zones); i++ {
		if keysOnly {
			res[i] = []string{zones[i].Name}
			continue
		}
		res[i] = []string{
			zones[i].Name,
			zones[i].EnumName,
			zones[i].Department,
		}
	}
	return res
}
