// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package paths

const (
	// ShivasCLI is the path to the shivas tool.
	ShivasCLI = "/usr/local/bin/shivas"

	// GetHostIdentifierScript is the path to the get_host_identifier script.
	GetHostIdentifierScript = "/usr/local/bin/get_host_identifier"

	// GetOSVersionScript is the path to get the os version infromation script.
	GetOSVersionScript = "/usr/local/bin/get_host_os_version"

	// DockerPath is the path to the wrapper around docker.
	DockerPath = "/usr/local/bin/docker"

	// HostsFilePath
	HostsFilePath = "/etc/dut_hosts/hosts"

	// LeasesPath is the path to IPs
	LeasesPath = "/var/lib/misc/dnsmasq.leases"

	// Fping is the path to the wrapper around fping
	Fping = "/usr/sbin/fping"

	// GetHostIPScript is the path to get the host ip script.
	GetHostIPScript = "/usr/local/bin/get_host_ip"

	// GetHostMACScript is the path to get the host mac address script.
	GetHostMACScript = "/usr/local/bin/get_host_mac"

	// NetInfoPathTemplate is the path to get internet info of satlab machine.
	NetInfoPathTemplate = "/sys/class/net/%v/address"

	// Grep the path of grep command
	Grep = "/bin/grep"

	// Reboot the path of reboot command.
	Reboot = "/usr/local/bin/reboot"
)
