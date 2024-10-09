// Copyright 2023 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package experimental

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/maruel/subcommands"
	"google.golang.org/api/sheets/v4"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/grpc/prpc"

	"infra/cmd/shivas/site"
	"infra/cmd/shivas/utils"
	"infra/cmdsupport/cmdlib"
	"infra/libs/sheet"
	ufspb "infra/unifiedfleet/api/v1/models"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
	ufsUtil "infra/unifiedfleet/app/util"
)

// ImportOSNicsCmd contains audit-duts command specification
var ImportOSNicsCmd = &subcommands.Command{
	UsageLine: "import-os-nics",
	ShortDesc: "import OS nic info",
	LongDesc: `import OS nic info based on https://docs.google.com/spreadsheets/d/1Lj2HiLT0dLBQXcLoRv3fArnlAkyYxZd6vVfQ-fBaEQI/edit?pli=1&resourcekey=0-ZLPb7FgibPlqG_QH9SrMRA#gid=1219489772.
	./shivas import-os-nics ...`,
	CommandRun: func() subcommands.CommandRun {
		c := &ImportOSNicsRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.commonFlags.Register(&c.Flags)

		c.Flags.StringVar(&c.dhcpFilepath, "dhcp-filepath", "", "the file path of the dhcp file which contains macaddress for all to-be-imported assets")
		return c
	},
}

type ImportOSNicsRun struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags

	dhcpFilepath string
}

func (c *ImportOSNicsRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

func (c *ImportOSNicsRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) (err error) {
	if err := c.validateArgs(); err != nil {
		return err
	}

	ctx := cli.GetContext(a, c, env)
	ns, err := c.envFlags.Namespace(nil, ufsUtil.OSNamespace)
	if err != nil {
		return err
	}
	ctx = utils.SetupContext(ctx, ns)
	hc, err := cmdlib.NewHTTPClient(ctx, &c.authFlags)
	if err != nil {
		return err
	}
	e := c.envFlags.Env()
	if c.commonFlags.Verbose() {
		fmt.Printf("Using UFS service %s\n", e.UnifiedFleetService)
	}

	hostsToMacAddresses, err := importMacAddress(c.dhcpFilepath)
	if err != nil {
		return err
	}

	sheetClient, err := sheet.NewClient(ctx, hc)
	if err != nil {
		return err
	}
	resp, err := sheetClient.Get(ctx, "1Lj2HiLT0dLBQXcLoRv3fArnlAkyYxZd6vVfQ-fBaEQI", []string{"ToR+Hydra Map(OS)"})
	if err != nil {
		return err
	}

	rackInfos := parseSwitchInfo(resp)
	shivasSwitchCmd := "shivas add switch -name %s -capacity 48 -rack %s\n"
	for _, r := range rackInfos {
		if r.ToR1 != "" {
			fmt.Printf(shivasSwitchCmd, r.ToR1, r.rackName)
		}
		if r.ToR2 != "" {
			fmt.Printf(shivasSwitchCmd, r.ToR2, r.rackName)
		}
	}
	ic := ufsAPI.NewFleetPRPCClient(&prpc.Client{
		C:       hc,
		Host:    e.UnifiedFleetService,
		Options: site.DefaultPRPCOptions(c.envFlags),
	})
	nicInfos, err := parseNicInfo(ctx, rackInfos, hostsToMacAddresses, ic)
	if err != nil {
		return err
	}
	shivasNicCmd := "shivas add nic -name %s:eth0 -switch %s -mac %s -machine %s -switch-port %d\n"
	for _, n := range nicInfos {
		if n.machineName != "" {
			// deployed DUT
			if n.macAddr == "" {
				return fmt.Errorf("%s is deployed but cannot find its macaddress", n.hostName)
			}
			fmt.Printf(shivasNicCmd, n.hostName, n.switchName, n.macAddr, n.machineName, n.port)
		}
	}
	return nil
}

func (c *ImportOSNicsRun) validateArgs() error {
	if c.dhcpFilepath == "" {
		return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe dhcp file path has to be specified.")
	}
	return nil
}

type rackInfo struct {
	rackName string
	ToR1     string
	ToR2     string
	column   int
}

type hostInfo struct {
	hostName    string
	switchName  string
	macAddr     string
	port        int
	machineName string
}

func parseSwitchInfo(torMap *sheets.Spreadsheet) []*rackInfo {
	res := []*rackInfo{}
	perRowRackMap := make(map[int]*rackInfo)
	perRowMaxRackNum := 25
	for _, row := range torMap.Sheets[0].Data[0].RowData {
		firstColumn := strings.TrimSpace(row.Values[0].FormattedValue)
		// Skip empty line
		if firstColumn == "" {
			continue
		}

		if firstColumn == "footprint" {
			curIndex := 1
			for curIndex <= perRowMaxRackNum {
				rackName := strings.TrimSpace(row.Values[curIndex+3].FormattedValue)
				if rackName != "" {
					perRowRackMap[curIndex] = &rackInfo{rackName: rackName, column: curIndex}
				}
				curIndex++
			}
		}
		if firstColumn == "ToR2" {
			curIndex := 1
			for curIndex <= perRowMaxRackNum {
				tor2 := strings.TrimSpace(row.Values[curIndex+3].FormattedValue)
				if tor2 != "" && ifSwitch(tor2) {
					perRowRackMap[curIndex].ToR2 = tor2
				}
				curIndex++
			}
		}
		if firstColumn == "ToR1" {
			curIndex := 1
			for curIndex <= perRowMaxRackNum {
				tor1 := strings.TrimSpace(row.Values[curIndex+3].FormattedValue)
				if tor1 != "" && ifSwitch(tor1) {
					perRowRackMap[curIndex].ToR1 = tor1
				}
				curIndex++
			}

			tempRacks := make([]*rackInfo, 0)
			for _, v := range perRowRackMap {
				tempRacks = append(tempRacks, v)
			}
			sort.SliceStable(tempRacks, func(i, j int) bool {
				return tempRacks[i].column < tempRacks[j].column
			})
			res = append(res, tempRacks...)
			// reset the per-row map
			perRowRackMap = make(map[int]*rackInfo)
		}
	}
	return res
}

func parseNicInfo(ctx context.Context, racks []*rackInfo, hostsToMacAddresses map[string]string, ic ufsAPI.FleetClient) ([]*hostInfo, error) {
	res := make([]*hostInfo, 0)
	hostnames := make([]string, 0)
	for _, rack := range racks {
		if rack.ToR1 != "" {
			r, hosts, err := getHostsForTor1(rack.rackName, rack.ToR1)
			if err != nil {
				return nil, err
			}
			res = append(res, r...)
			hostnames = append(hostnames, hosts...)
		}
		if rack.ToR2 != "" {
			r, hosts, err := getHostsForTor2(rack.rackName, rack.ToR2)
			if err != nil {
				return nil, err
			}
			res = append(res, r...)
			hostnames = append(hostnames, hosts...)
		}
	}
	hosts := utils.ConcurrentGet(ctx, ic, hostnames, utils.GetSingleMachineLSE)
	hostsToMachines := make(map[string]string, 0)
	for _, h := range hosts {
		dut := h.(*ufspb.MachineLSE)
		hostsToMachines[ufsUtil.RemovePrefix(dut.Name)] = dut.GetMachines()[0]
	}

	for _, r := range res {
		r.macAddr = hostsToMacAddresses[r.hostName]
		r.machineName = hostsToMachines[r.hostName]
	}
	return res, nil
}

// Port mapping for Tor1 and Tor2:
// https://docs.google.com/spreadsheets/d/1Qhr_ZttemysTern0NWjV6j9VVZn3Kbe_DfuF8BlzfKM/edit#gid=1942548728
func getHostsForTor1(rackName, tor string) ([]*hostInfo, []string, error) {
	prefix, err := getHostnamePrefix(rackName)
	if err != nil {
		return nil, nil, err
	}

	res := make([]*hostInfo, 0)
	hostnames := make([]string, 0)
	for i := 1; i <= 14; i++ {
		hostname := prefix + "host" + strconv.Itoa(56-(i-1)*2)
		res = append(res, &hostInfo{
			hostName:   hostname,
			switchName: tor,
			port:       i,
		})
		hostnames = append(hostnames, hostname)
	}
	for i := 35; i <= 48; i++ {
		hostname := prefix + "host" + strconv.Itoa(55-(i-35)*2)
		res = append(res, &hostInfo{
			hostName:   hostname,
			switchName: tor,
			port:       i,
		})
		hostnames = append(hostnames, hostname)
	}
	return res, hostnames, nil
}

func getHostsForTor2(rackName, tor string) ([]*hostInfo, []string, error) {
	prefix, err := getHostnamePrefix(rackName)
	if err != nil {
		return nil, nil, err
	}
	res := make([]*hostInfo, 0)
	hostnames := make([]string, 0)
	for i := 1; i <= 14; i++ {
		hostname := prefix + "host" + strconv.Itoa(27-(i-1)*2)
		res = append(res, &hostInfo{
			hostName:   hostname,
			switchName: tor,
			port:       i,
		})
		hostnames = append(hostnames, hostname)
	}
	for i := 35; i <= 48; i++ {
		hostname := prefix + "host" + strconv.Itoa(28-(i-35)*2)
		res = append(res, &hostInfo{
			hostName:   hostname,
			switchName: tor,
			port:       i,
		})
		hostnames = append(hostnames, hostname)
	}
	return res, hostnames, nil
}

func getHostnamePrefix(rackName string) (string, error) {
	// rackName is in format "03-12"
	// Split the string into two parts, the first part is the row number and the second part is the rack number.
	parts := strings.Split(rackName, "-")
	// Convert the row number to a string.
	rowInt, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", err
	}
	rackInt, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", err
	}
	row := strconv.Itoa(rowInt)
	rack := strconv.Itoa(rackInt)
	// Compose the new string.
	return "chromeos8-row" + row + "-rack" + rack + "-", nil
}

func importMacAddress(dhcpFilepath string) (map[string]string, error) {
	res := make(map[string]string)
	// Open a local file, this local file should be the same as https://source.corp.google.com/chrome-golo/services/dhcpd/sfo36-cr/dhcp-vlan400-production
	f, err := os.Open(dhcpFilepath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if err != nil {
		return res, err
	}

	// Create a regular expression to match the lines that we want to print.
	re := regexp.MustCompile(`host (.+) { hardware ethernet (.+); fixed-address (.+); ddns-hostname (.+); option host-name (.+);\s*}\s*|\}`)

	// Read each line of the file.
	for scanner := bufio.NewScanner(f); scanner.Scan(); {
		// Match the line against the regular expression.
		match := re.FindStringSubmatch(scanner.Text())

		// If the line matches the regular expression, print the hostname and mac address.
		if match != nil {
			res[match[1]] = match[2]
		}
	}
	return res, nil
}

func ifSwitch(tor string) bool {
	return strings.Contains(tor, ".sfo36")
}
