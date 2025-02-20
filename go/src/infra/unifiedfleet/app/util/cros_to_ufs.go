// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"regexp"
	"strings"

	"google.golang.org/api/sheets/v4"

	"go.chromium.org/luci/common/errors"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
)

// Example: subnet 100.115.224.0 netmask 255.255.254.0 {
var dhcpdVlanRegexp = regexp.MustCompile(`subnet [0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3} netmask [0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3} {`)

// Example: 100.115.224.0
var ipRegexp = regexp.MustCompile(`[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}`)

// Example: fixed-address 100.115.224.2;
var fixedAddressRegexp = regexp.MustCompile(`fixed-address [0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3};`)

// Example: host host1 {
var hostNameRegexp = regexp.MustCompile(`host .*{`)

// Example: hardware ethernet aa:00:00:00:00:00
var ethernetRegexp = regexp.MustCompile(`hardware ethernet .*;`)

// Example: aa:00:00:00:00:00
var macAddrRegexp = regexp.MustCompile(`([a-fA-F0-9]{2}\:){5}[a-fA-F0-9]{2}`)

// DHCPConf defines the format of response after parsing a ChromeOS dhcp conf file
type DHCPConf struct {
	ValidVlans       []*ufspb.Vlan
	ValidIPs         []*ufspb.IP
	ValidDHCPs       []*ufspb.DHCPConfig
	DHCPsWithoutVlan []*ufspb.DHCPConfig
	MismatchedVlans  []*ufspb.Vlan
	DuplicatedVlans  []*ufspb.Vlan
	DuplicatedIPs    []*ufspb.IP
}

// ParseOSDhcpdConf parses dhcpd.conf
func ParseOSDhcpdConf(conf string, topology map[string]*ufspb.Vlan) (*DHCPConf, error) {
	respIPs := make([]*ufspb.IP, 0)
	respVlans := make(map[string]*ufspb.Vlan, 0)
	dhcps := make([]*ufspb.DHCPConfig, 0)
	dhcpsWithoutVlan := make([]*ufspb.DHCPConfig, 0)
	ipMaps := make(map[string]*ufspb.IP, 0)
	mismatchedVlans := make([]*ufspb.Vlan, 0)
	duplicatedVlans := make([]*ufspb.Vlan, 0)
	duplicatedIPs := make([]*ufspb.IP, 0)

	lines := strings.Split(conf, "\n")
	// Record the hostname which is under scanning
	foundHostname := ""
	// Record the mac address which is under scanning
	foundMacAddress := ""
	for _, line := range lines {
		// Skip commented line
		if strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.Trim(line, "\r"))

		// Parse lines like "subnet 100.115.224.0 netmask 255.255.254.0"
		if dhcpdVlanRegexp.MatchString(line) {
			subnet, vlan, notMatchTopology := parseSubnetAndMaskLine(line, topology)
			if _, ok := respVlans[subnet]; ok {
				duplicatedVlans = append(duplicatedVlans, vlan)
				continue
			}
			respVlans[subnet] = vlan
			if notMatchTopology {
				mismatchedVlans = append(mismatchedVlans, vlan)
			}
			startIP, err := IPv4StrToInt(subnet)
			if err != nil {
				return nil, errors.Reason("fail to parse subnet %s to uint32", subnet).Err()
			}
			for i := 0; i < int(vlan.CapacityIp); i++ {
				ipV4Str := IPv4IntToStr(startIP)
				ip := &ufspb.IP{
					Id:      GetIPName(vlan.GetName(), ipV4Str),
					Ipv4:    startIP,
					Ipv4Str: ipV4Str,
					Vlan:    vlan.GetName(),
				}
				respIPs = append(respIPs, ip)
				ipMaps[ipV4Str] = ip
				startIP++
			}
			continue
		}

		// Parse lines like "fixed-address 100.115.224.2"
		if fixedAddressRegexp.MatchString(line) {
			ips := ipRegexp.FindAllString(line, -1)
			foundIP := ips[0]
			if foundHostname == "" {
				return nil, errors.Reason("no hostname for the address (%s) ", foundIP).Err()
			}
			dhcp := &ufspb.DHCPConfig{
				Hostname:   foundHostname,
				MacAddress: foundMacAddress,
				Ip:         foundIP,
			}
			// Reset
			foundHostname = ""
			foundMacAddress = ""
			ip, ok := ipMaps[foundIP]
			if !ok {
				dhcpsWithoutVlan = append(dhcpsWithoutVlan, dhcp)
			} else {
				if ip.Occupied {
					duplicatedIPs = append(duplicatedIPs, ip)
				}
				ip.Occupied = true
				dhcps = append(dhcps, dhcp)
			}
		}

		// Parse lines like "host host1 {"
		if hostNameRegexp.MatchString(line) {
			res := strings.Split(strings.TrimSpace(line[0:len(line)-1]), " ")
			if len(res) != 2 {
				return nil, errors.Reason("wrong format of hostname (%s)", line).Err()
			}
			foundHostname = res[1]
		}

		// Parse lines like "hardware ethernet aa:00:00:00:00:00;"
		if ethernetRegexp.MatchString(line) {
			macAddr := macAddrRegexp.FindAllString(line, -1)
			if len(macAddr) > 1 {
				return nil, errors.Reason("wrong format of ethernet (%s)", line).Err()
			}
			// mac address can be empty, e.g. "hardware ethernet ;"
			if len(macAddr) == 1 {
				foundMacAddress = macAddr[0]
			}
		}
	}

	// Also return the vlans pre-defined in topology but haven't been setup in dhcp conf.
	for k, v := range topology {
		if _, ok := respVlans[k]; !ok {
			respVlans[k] = v
		}
	}
	vlans := make([]*ufspb.Vlan, 0, len(respVlans))
	for _, v := range respVlans {
		vlans = append(vlans, v)
	}
	return &DHCPConf{
		ValidVlans:       vlans,
		ValidIPs:         respIPs,
		ValidDHCPs:       dhcps,
		DHCPsWithoutVlan: dhcpsWithoutVlan,
		MismatchedVlans:  mismatchedVlans,
		DuplicatedIPs:    duplicatedIPs,
		DuplicatedVlans:  duplicatedVlans,
	}, nil
}

// ParseATLTopology parse the topology of ATL lab based on a Google sheet
func ParseATLTopology(data *sheets.Spreadsheet) (map[string]*ufspb.Vlan, []*ufspb.Vlan) {
	resp := make(map[string]*ufspb.Vlan, 0)
	dupcatedVlan := make([]*ufspb.Vlan, 0)
	header := make([]string, 0)
	for i, row := range data.Sheets[0].Data[0].RowData {
		// Skip empty line
		if row.Values[0].FormattedValue == "" {
			continue
		}

		// Skip but parse header
		if i == 0 {
			for _, cell := range row.Values {
				header = append(header, cell.FormattedValue)
			}
			// Invalid sheet info
			if len(header) == 0 {
				break
			}
			continue
		}

		addr, vlan := parseTopologyRow(header, row.Values)
		// Skip rows without empty string in column "VLAN #" and "Address"
		if addr != "" && vlan.Name != "" {
			if _, ok := resp[addr]; ok {
				dupcatedVlan = append(dupcatedVlan, vlan)
				continue
			}
			resp[addr] = vlan
		}
	}
	return resp, dupcatedVlan
}

func parseSubnetAndMaskLine(line string, topology map[string]*ufspb.Vlan) (string, *ufspb.Vlan, bool) {
	ips := ipRegexp.FindAllString(line, -1)
	subnet := ips[0]
	cidr, capacity := parseCidrBlock(subnet, ips[1])
	notMatchTopology := false
	vlan, ok := topology[subnet]
	if ok {
		vlan.CapacityIp = int32(capacity - 2)
		if vlan.VlanAddress != cidr {
			notMatchTopology = true
		}
		vlan.VlanAddress = cidr
	} else {
		name := GetCrOSLabName(subnet)
		vlan = &ufspb.Vlan{
			// Use subnet as part of name and randomly assign vlan's name to CrOS lab
			Name:        name,
			VlanAddress: cidr,
			// OS lab-specific, 2 last ips are reserved
			CapacityIp: int32(capacity - 2),
			VlanNumber: GetSuffixAfterSeparator(name, ":"),
		}
	}
	return subnet, vlan, notMatchTopology
}

func parseTopologyRow(header []string, rowValue []*sheets.CellData) (string, *ufspb.Vlan) {
	vlan := &ufspb.Vlan{}
	addr := ""
	mask := ""
	for j, cell := range rowValue {
		if j >= len(header) {
			break
		}
		switch header[j] {
		case "Subnet Name":
			vlan.Description = cell.FormattedValue
		case "VLAN #", "VLAN":
			if cell.FormattedValue != "" {
				vlan.Name = GetATLLabName(cell.FormattedValue)
				vlan.VlanNumber = GetSuffixAfterSeparator(vlan.Name, ":")
			}
		case "Allocated Size":
			vlan.CapacityIp = int32(*cell.EffectiveValue.NumberValue)
		case "Address":
			addr = cell.FormattedValue
		case "Mask":
			mask = cell.FormattedValue
		}
		if addr != "" && mask != "" {
			vlan.VlanAddress = addr + mask
		}
	}
	return addr, vlan
}
