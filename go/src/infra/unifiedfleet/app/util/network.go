// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"net"
	"strings"

	"go.chromium.org/luci/common/errors"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/util/iputil"
)

// The first ip is the subnet ID
// The next first 10 ips are reserved
const reserveFirst = 11

// The last ip is broadcast address.
const reserveLast = 1

// maxPreallocatedVlanSize is the maximum number of preallocated vlan addresses.
const maxPreallocatedVlanSize = 2000

// StringifyIP stringifies an IP. The standard library makes the interesting
// choice of mapping an empty IP address object to "<nil>" rather than "".
//
// Here we just return an empty string given an empty IP.
func StringifyIP(ip net.IP) string {
	if len(ip) == 0 {
		return ""
	}
	return ip.String()
}

// uint32ToIP converts a uint32 to an IP address
func uint32ToIP(addr uint32) net.IP {
	return net.ParseIP(IPv4IntToStr(addr))
}

// IPv4ToUint32 converts an IP to a uint32
func IPv4ToUint32(ip net.IP) (uint32, error) {
	if ip := ip.To4(); ip != nil {
		return uint32(ip[0])*256*256*256 + uint32(ip[1])*256*256 + uint32(ip[2])*256 + uint32(ip[3]), nil
	}
	return 0, fmt.Errorf("ip %s is not ipv4", ip.String())
}

// makeReservedIPsInVlan takes an inclusive range of ipv4s and produces reserved ipv4s for use in a vlan.
func makeReservedIPsInVlan(vlanName string, beginIP net.IP, endIP net.IP, maximum int) ([]*ufspb.IP, error) {
	if maximum <= 0 {
		return nil, errors.New("maximum must be positive")
	}
	if maximum > maxPreallocatedVlanSize {
		return nil, errors.New("maximum cannot exceed MaxPreallocatedVlanSize")
	}
	if iputil.IPDiff(beginIP, endIP).Sign() == 1 {
		return nil, errors.New("begin cannot be greater than end")
	}
	proposedLen := iputil.IPDiff(endIP, beginIP)
	proposedLen.Add(proposedLen, big.NewInt(1))
	if proposedLen.Cmp(big.NewInt(int64(maximum))) == 1 {
		return nil, errors.New("IP range exceeds maximum")
	}
	ips := make([]*ufspb.IP, 0, maximum)
	if err := iputil.IPIter(beginIP, endIP, func(ip net.IP) error {
		ipItem := FormatIP(vlanName, StringifyIP(ip), true, false)
		if ipItem == nil {
			return fmt.Errorf("%q %d failed to produce an IP address", vlanName, ip)
		}
		ips = append(ips, ipItem)
		return nil
	}); err != nil {
		return nil, err
	}
	return ips, nil
}

// getFreeStartIP gets the value of the first freeIP in a vlan.
//
// This is either the explicit freeStartIP given, or a fixed number of IPs
// after the start of the overall address by default.
func getFreeStartIP(freeStartIP string, startIP net.IP) (net.IP, error) {
	if freeStartIP == "" {
		ip := iputil.AddToIP(startIP, big.NewInt(reserveFirst))
		return ip, nil
	}
	if ip := net.ParseIP(freeStartIP); ip != nil {
		return ip, nil
	}
	return nil, fmt.Errorf("invalid IP address %q", freeStartIP)
}

// getFreeEndIP gets the value of the last freeIP in a vlan.
//
// This is either the explicit freeEndIP given, or a fixed number of IPs
// after the end of the overall IP address.
func getFreeEndIP(freeEndIP string, startIP net.IP, length int) (net.IP, error) {
	if freeEndIP == "" {
		ip := iputil.AddToIP(startIP, big.NewInt(int64(length-reserveLast-1)))
		return ip, nil
	}
	if ip := net.ParseIP(freeEndIP); ip != nil {
		return ip, nil
	}
	return nil, fmt.Errorf("invalid IP address %q", freeEndIP)
}

// ParseVlan parses vlan to a list of IPs
//
// vlanName here is a full vlan name, e.g. browser:123
// The first 10 and last 1 ip of this cidr block will be reserved and not returned to users
// for further operations
func ParseVlan(vlanName, cidr, freeStartIPStr, freeEndIPStr string) ([]*ufspb.IP, int, string, string, int, error) {
	ip, subnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, 0, "", "", 0, errors.Reason("invalid CIDR block %q for vlan %s", cidr, vlanName).Err()
	}
	ones, bits := subnet.Mask.Size()
	length := 1 << (bits - ones)
	startIP := ip
	freeStartIP, err := getFreeStartIP(freeStartIPStr, startIP)
	if err != nil {
		return nil, 0, "", "", 0, errors.Annotate(err, "computing free start IP %q for vlan %s", freeStartIP, vlanName).Err()
	}
	freeEndIP, err := getFreeEndIP(freeEndIPStr, startIP, length)
	if err != nil {
		return nil, 0, "", "", 0, errors.Annotate(err, "computing free end IP %q for vlan %s", freeStartIP, vlanName).Err()
	}
	used := iputil.IPDiff(freeEndIP, freeStartIP)
	reservedNum := length - int(used.Int64()) - 1
	ips, err := makeIPv4sInVlan(vlanName, startIP, length, freeStartIP, freeEndIP)
	if err != nil {
		return nil, 0, "", "", 0, err
	}
	return ips, length, StringifyIP(freeStartIP), StringifyIP(freeEndIP), reservedNum, nil
}

var errStopEarly = errors.New("stop early")

// makeIPv4sInVlan creates the IP objects in a Vlan that are intended to be created in datastore later.
func makeIPv4sInVlan(vlanName string, startIP net.IP, length int, freeStartIP net.IP, freeEndIP net.IP) ([]*ufspb.IP, error) {
	endIP := iputil.AddToIP(startIP, big.NewInt(int64(length)-1))
	reservedInitialIPs, err := makeReservedIPsInVlan(vlanName, startIP, iputil.AddToIP(freeStartIP, big.NewInt(-1)), maxPreallocatedVlanSize)
	if err != nil {
		return nil, errors.Annotate(err, "reserving initial IPs").Err()
	}
	var reservedFinalIPs []*ufspb.IP
	afterFreeEndIP := iputil.AddToIP(freeEndIP, big.NewInt(1))
	le := !iputil.IsNegative(iputil.IPDiff(endIP, afterFreeEndIP))
	if le {
		var err error
		reservedFinalIPs, err = makeReservedIPsInVlan(vlanName, afterFreeEndIP, endIP, maxPreallocatedVlanSize)
		if err != nil {
			return nil, errors.Annotate(err, "reserving final IPs").Err()
		}
	}
	ips := make([]*ufspb.IP, 0, maxPreallocatedVlanSize)
	iputil.IPIter(freeStartIP, freeEndIP, func(ip net.IP) error {
		if len(ips) > maxPreallocatedVlanSize {
			return errStopEarly
		}
		ips = append(ips, FormatIP(vlanName, ip.String(), false, false))
		return nil
	})
	out := []*ufspb.IP{}
	out = append(out, reservedInitialIPs...)
	out = append(out, ips...)
	out = append(out, reservedFinalIPs...)
	return out, nil
}

// makeIPv4Uint32 makes an IPv4 as a uint32 using notation that looks conventional.
//
// makeIPv4Uint32(127, 0, 0, 1) === 127.0.0.1
func makeIPv4Uint32(a, b, c, d int) uint32 {
	return uint32(a)*256*256*256 + uint32(b)*256*256 + uint32(c)*256 + uint32(d)
}

// FormatIP initialize an IP object
func FormatIP(vlanName, ipAddress string, reserve, occupied bool) *ufspb.IP {
	ip := net.ParseIP(ipAddress)
	var ipv4 uint32
	ipv4Str := ""
	idForName := fmt.Sprintf("0x%s", hex.EncodeToString(ip))
	switch {
	case len(ip) == 0:
		return nil
	case ip.To4() != nil:
		var err error
		ipv4, err = IPv4StrToInt(ipAddress)
		if err != nil {
			return nil
		}
		ipv4Str = ipAddress
		idForName = Int64ToStr(int64(ipv4))
	}
	return &ufspb.IP{
		Id:       GetIPName(vlanName, idForName),
		Ipv4:     ipv4,
		Ipv4Str:  ipv4Str,
		Ipv6:     ip,
		Vlan:     vlanName,
		Occupied: occupied,
		Reserve:  reserve,
	}
}

// ipv4Diff takes the difference between a startIPv4 and an endIPv4.
//
// It returns an error if and only if A) at least one argument is an invalid IP address or B) the end strictly precedes the start
func ipv4Diff(startIPv4 string, endIPv4 string) (uint64, error) {
	start, err := IPv4StrToInt(startIPv4)
	if err != nil {
		return 0, errors.Annotate(err, "diffing IPs %q and %q", startIPv4, endIPv4).Err()
	}
	end, err := IPv4StrToInt(endIPv4)
	if err != nil {
		return 0, errors.Annotate(err, "diffing IPs %q and %q", startIPv4, endIPv4).Err()
	}
	if start > end {
		return 0, errors.Reason("end IP %q precedes start IP %q", endIPv4, startIPv4).Err()
	}
	return uint64(end) - uint64(start), nil
}

// Uint32Iter runs a command over a range of Uint32's. Useful for iterating over IP addresses.
func Uint32Iter(start uint32, end uint32, f func(uint32) error) error {
	for num := start; num <= end; num++ {
		if err := f(num); err != nil {
			return err
		}
	}
	return nil
}

// IPv4StrToInt returns an uint32 address from the given ip address string.
func IPv4StrToInt(ipAddress string) (uint32, error) {
	ip := net.ParseIP(ipAddress)
	if ip != nil {
		ip = ip.To4()
	}
	if ip == nil {
		return 0, errors.Reason("invalid IPv4 address %q", ipAddress).Err()
	}
	return binary.BigEndian.Uint32(ip), nil
}

// IPv4IntToStr returns a string ip address
func IPv4IntToStr(ipAddress uint32) string {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, ipAddress)
	return ip.String()
}

// parseCidrBlock returns a tuple of (cidr_block, capacity of this block)
func parseCidrBlock(subnet, mask string) (string, int) {
	maskIP := net.ParseIP(mask)
	maskAddr := maskIP.To4()
	ones, sz := net.IPv4Mask(maskAddr[0], maskAddr[1], maskAddr[2], maskAddr[3]).Size()
	return fmt.Sprintf("%s/%d", subnet, ones), 1 << uint32(sz-ones)
}

// ParseMac returns a valid mac address after parsing user input.
func ParseMac(userMac string) (string, error) {
	newUserMac := formatMac(userMac)
	m, err := net.ParseMAC(newUserMac)
	if err != nil || len(m) != 6 {
		return "", errors.Reason("invalid mac address %q (before parsing %q)", newUserMac, userMac).Err()
	}
	bytes := make([]byte, 8)
	copy(bytes[2:], m)
	mac := make(net.HardwareAddr, 8)
	binary.BigEndian.PutUint64(mac, binary.BigEndian.Uint64(bytes))
	return mac[2:].String(), nil
}

func formatMac(userMac string) string {
	if strings.Contains(userMac, ":") {
		return userMac
	}

	var newMac string
	for i := 0; ; i += 2 {
		if i+2 > len(userMac)-1 {
			newMac += userMac[i:]
			break
		}
		newMac += userMac[i:i+2] + ":"
	}
	return newMac
}

// IsMacFormatValid check if the given mac address is in valid format
//
// TODO(gregorynisbet): only shivas uses this function, move it.
func IsMacFormatValid(userMac string) error {
	newUserMac := formatMac(userMac)
	m, err := net.ParseMAC(newUserMac)
	if err != nil || len(m) != 6 {
		return errors.Reason("Invalid mac address %q (before parsing %q)", newUserMac, userMac).Err()
	}
	return nil
}
