// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cmd

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"infra/cmd/labtunnel/cmdutils"
	"infra/cmd/labtunnel/crosfleet"
	"infra/cmd/labtunnel/fileutils"
	clog "infra/cmd/labtunnel/log"
	"infra/cmd/labtunnel/ssh"
)

const (
	crosfleetHostnamePrefix = "crossk-"
	crosHostnameSuffix      = ".cros"
)

var satlabHostnameMatcher = regexp.MustCompile(`^(satlab-\w+)-(.*)-host.+$`)
var dockerNoSuchContainerMatcher = regexp.MustCompile(`No such container`)

const satlabUsername = "moblab"

// resolveDutHostname remove any prefixes if hostname is given, otherwise if
// hostname is "leased" will determine a host to use from crosfleet. Returns
// hostname as a string, boolean which is true if DUT is leased, and error if
// any problems determining DUT from crosfleet.
func resolveDutHostname(ctx context.Context, hostnameParam string) (string, bool, error) {
	if hostnameParam != "leased" {
		return resolveHostname(hostnameParam, ""), false, nil
	}
	hostnames, err := crosfleet.CrosfleetLeasedDUTs(ctx)
	if err != nil {
		return "", true, err
	}
	if len(hostnames) < 1 {
		return "", true, fmt.Errorf("could not find any DUTs leased from crosfleet")
	} else if len(hostnames) == 1 {
		clog.Logger.Printf("Defaulting to only leased DUT: %s", hostnames[0])
		return hostnames[0], true, nil
	}
	hostname, err := promptUserForDutChoice(ctx, hostnames)
	return hostname, true, err
}

func promptUserForDutChoice(ctx context.Context, hostnames []string) (string, error) {
	totalDuts := len(hostnames)
	prompt := fmt.Sprintf("Found %d leased DUTs please select the DUT you would like to tunnel to:\n", totalDuts)
	for i, hostname := range hostnames {
		prompt += fmt.Sprintf("%d: %s\n", i, hostname)
	}
	prompt += fmt.Sprintf("\nSelect from 0-%d: ", totalDuts-1)
	inputReader := bufio.NewReader(fileutils.NewContextualReaderWrapper(ctx, os.Stdin))
	for {
		var selected int
		fmt.Print(prompt)
		input, err := inputReader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read user input for prompt: %w", err)
		}
		n, err := fmt.Sscanf(input, "%d", &selected)
		if n != 1 || err != nil {
			continue
		}
		if selected >= totalDuts || selected < 0 {
			fmt.Printf("\nInvalid index %d\n\n", selected)
			selected = -1
			continue
		}
		clog.Logger.Printf("Using user selected leased DUT: %s", hostnames[selected])
		return hostnames[selected], nil
	}

	// Unreachable
}

func resolveHostname(hostnameParam string, suffixToAdd string) string {
	// Remove any crosfleet name prefix.
	hostnameParam = strings.TrimPrefix(hostnameParam, crosfleetHostnamePrefix)

	if suffixToAdd != "" {
		// Add suffix, keeping any existing cros suffix.
		if strings.HasSuffix(hostnameParam, crosHostnameSuffix) {
			hostnameParam = strings.TrimSuffix(hostnameParam, crosHostnameSuffix)
			hostnameParam += suffixToAdd
			hostnameParam += crosHostnameSuffix
		} else {
			hostnameParam += suffixToAdd
		}
	}
	return hostnameParam
}

func buildSSHManager() *ssh.ConcurrentSSHManager {
	return ssh.NewConcurrentSSHManager(ssh.NewRunner(sshOptions), time.Duration(sshRetryDelaySeconds)*time.Second)
}

func nextLocalPort(ctx context.Context) int {
	for {
		localPort := localPortStart
		localPortStart++
		available, err := isPortAvailable(ctx, localPort)
		if err != nil {
			panic(fmt.Errorf("failed to find next available open port, err: %w", err))
		}
		if available {
			return localPort
		}
	}
	// Unreachable
}

// lookupActualHosts resolves remote & ssh host, accounting for any that are satlab.
// Returns remote and ssh hosts
func lookupActualHosts(remoteHost, sshHost string) (string, string, error) {
	// user provided remote & ssh hosts, no need for lookup.
	if remoteHost != "" && sshHost != "" {
		return remoteHost, sshHost, nil
	}

	// default remote is "localhost"
	if remoteHost == "" {
		remoteHost = "localhost"
	}

	// perform lookup logic for satlab hosts
	if satlabDroneHostOverride != "" || isSatlabHost(sshHost) {
		if remoteHost != "localhost" {
			return "", "", fmt.Errorf("tunneling to a non-localhost port through a satlab device is not supported")
		}
		var satlabDroneHost string
		if satlabDroneHostOverride != "" {
			satlabDroneHost = satlabDroneHostOverride
		} else {
			var err error
			satlabDroneHost, err = buildSatlabHostname(sshHost)
			if err != nil {
				return "", "", fmt.Errorf("failed to parse satlab hostnames from ssh host %q: %w", sshHost, err)
			}
		}
		if !strings.Contains(satlabDroneHost, "@") {
			satlabDroneHost = fmt.Sprintf("%s@%s", satlabUsername, satlabDroneHost)
		}
		remoteHost = sshHost
		sshHost = satlabDroneHost
	}

	return remoteHost, sshHost, nil
}

// tunnelLocalPortToRemotePort establishes an ssh tunnel to remoteHost:remotePort on sshHost
// Returns its localhost:port.
func tunnelLocalPortToRemotePort(ctx context.Context, tunnelRegistry *ssh.TunnelRegistry, sshManager *ssh.ConcurrentSSHManager, tunnelName string, remoteHost string, remotePort int, sshHost string) (string, error) {
	localPort := nextLocalPort(ctx)
	listenHost := fmt.Sprintf("localhost:%d", localPort)

	remoteHost, sshHost, err := lookupActualHosts(remoteHost, sshHost)
	if err != nil {
		return "", err
	}

	description := fmt.Sprintf("TUNNEL-%-10s [%s -> %s -> %s:%d]", tunnelName, listenHost, sshHost, remoteHost, remotePort)
	sshManager.SSH(ctx, true, description, func(ctx context.Context, r *ssh.Runner) error {
		return r.TunnelLocalPortToRemotePort(ctx, localPort, remoteHost, remotePort, sshHost)
	})
	if remoteHost != "localhost" {
		tunnelRegistry.Set(remoteHost, listenHost)
	} else {
		tunnelRegistry.Set(sshHost, listenHost)
	}
	return listenHost, nil
}

// startSatlabServod starts servod container for satlab hosts.
// Returns the satlab, container name, container ip, and servod port.
func startSatlabServod(ctx context.Context, remoteHost, sshHost, config string) (string, string, string, int, error) {
	if satlabDroneHostOverride == "" && !isSatlabHost(sshHost) {
		return "", "", "", 0, fmt.Errorf("host is not on satlab")
	}
	dutHost, satlab, err := lookupActualHosts(remoteHost, sshHost)
	if err != nil {
		return "", "", "", 0, err
	}
	clog.Logger.Printf("%s, %s resolved to %s, %s", remoteHost, sshHost, satlab, dutHost)

	containerName := dutHost + "-labtunnel-servod"

	// satlab servod start looks up board, model, and servo-serial for dut.
	output, err := runSSHCommand(ctx, satlab, "/usr/local/bin/satlab", "servod", "start",
		"-host", dutHost, "-servod-container-name", containerName, "-no-servod")
	if err != nil {
		return "", "", "", 0, fmt.Errorf("could not start servod container on satlab: %w %s", err, output)
	}

	output, err = runSSHCommand(ctx, satlab, "/usr/local/bin/docker", "exec", "-e", "CONFIG="+config, "-d", containerName, "bash", "start_servod.sh")
	if err != nil {
		return "", "", "", 0, fmt.Errorf("could not run servod on satlab: %w %s", err, output)
	}

	portStr, err := runSSHCommand(ctx, satlab, "/usr/local/bin/docker", "exec", containerName, "printenv", "PORT")
	if err != nil {
		return "", "", "", 0, fmt.Errorf("could not obtain servod port on satlab: %w %s", err, portStr)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", "", "", 0, fmt.Errorf("servod port %q is not recognized: %w", portStr, err)
	}

	ipStr, err := runSSHCommand(ctx, satlab, "/usr/local/bin/docker", "inspect", "-f", "'{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}'", containerName)
	if err != nil {
		return "", "", "", 0, fmt.Errorf("could not obtain servod container ip: %w %s", err, ipStr)
	}

	clog.Logger.Printf("servod is runnin in container: %s, ip: %s, port: %d on %s", containerName, ipStr, port, satlab)
	return satlab, containerName, ipStr, port, nil
}

// stopSatlabServod stops the servod container running on satlab.
func stopSatlabServod(ctx context.Context, satlab, servodContainer string) error {
	output, err := runSSHCommand(ctx, satlab, "/usr/local/bin/docker", "kill", servodContainer)
	if err != nil {
		if dockerNoSuchContainerMatcher.MatchString(output) {
			return nil
		}
		return fmt.Errorf("error stopping servod container: %w %s", err, output)
	}
	return nil
}

// runSSHCommand runs command on sshHost via ssh.
// Returns output of command.
func runSSHCommand(ctx context.Context, sshHost string, cmd ...string) (string, error) {
	cmdArgs, err := ssh.SSHOptsCmdline(sshOptions)
	if err != nil {
		return "", fmt.Errorf("error parsing sshOptions: %w", err)
	}

	cmdArgs = append(cmdArgs, sshHost)
	cmdArgs = append(cmdArgs, cmd...)

	clog.Logger.Printf("runSSHCommand: ssh %s", strings.Join(cmdArgs, " "))
	c := exec.CommandContext(ctx, "ssh", cmdArgs...)
	out, err := c.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func isSatlabHost(hostname string) bool {
	return satlabHostnameMatcher.MatchString(hostname)
}

func buildSatlabHostname(satlabDeviceHostname string) (string, error) {
	matches := satlabHostnameMatcher.FindAllStringSubmatch(satlabDeviceHostname, -1)
	if len(matches) != 1 || len(matches[0]) != 3 {
		return "", fmt.Errorf("failed to parse satlab drone host and local device host from satlab device hostname %q", satlabDeviceHostname)
	}
	satlabID := matches[0][1]
	satlabLocation := matches[0][2]
	satlabDroneHost := fmt.Sprintf("%s-%s", satlabLocation, satlabID)
	return satlabDroneHost, nil
}

func isPortAvailable(ctx context.Context, port int) (bool, error) {
	ssCmd := exec.CommandContext(ctx, "ss", "-tuln")
	output, err := ssCmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check port %d is available with command %q", port, ssCmd.String())
	}
	outputLines := strings.Split(string(output), "\n")
	for _, line := range outputLines {
		fields := strings.Fields(line)
		if len(fields) < 5 || (fields[0] != "tcp" && fields[0] != "udp") {
			continue
		}
		addressParts := strings.Split(fields[4], ":")
		usedPort, err := strconv.Atoi(addressParts[len(addressParts)-1])
		if err != nil {
			continue
		}
		if usedPort == port {
			return false, nil
		}
	}
	return true, nil
}

func runContextualCommand(ctx context.Context, logPrefix string, command string, args ...string) {
	// Build cmd and ensure subprocesses are grouped.
	cmd, err := cmdutils.CreateContextualCmd(ctx, command, args...)
	if err != nil {
		panic(fmt.Errorf("failed to create command %q: %w", command, err))
	}

	// Log StdOut and StdErr with a prefix.
	cmdLogger := clog.NewLogger(logPrefix)
	logWriter := clog.NewWriter(cmdLogger)
	cmd.Stdout = logWriter
	cmd.Stderr = logWriter

	// Wait until the command completes or the context is cancelled.
	runErr := cmdutils.ExecuteAndWaitContextualCmd(ctx, cmd)
	if runErr != nil && ctx.Err() != nil {
		clog.Logger.Printf("Error running command %q: %v", command, runErr)
	}
}

func tunnelToDut(ctx context.Context, tunnelRegistry *ssh.TunnelRegistry, sshManager *ssh.ConcurrentSSHManager, tunnelID int, hostname string) (string, error) {
	return tunnelLocalPortToRemotePort(ctx, tunnelRegistry, sshManager, fmt.Sprint("DUT-", tunnelID), "", remotePortSSH, hostname)
}

func tunnelToRouter(ctx context.Context, tunnelRegistry *ssh.TunnelRegistry, sshManager *ssh.ConcurrentSSHManager, tunnelID int, hostname string) (string, error) {
	return tunnelLocalPortToRemotePort(ctx, tunnelRegistry, sshManager, fmt.Sprint("ROUTER-", tunnelID), "", remotePortSSH, hostname)
}

func tunnelToPcap(ctx context.Context, tunnelRegistry *ssh.TunnelRegistry, sshManager *ssh.ConcurrentSSHManager, tunnelID int, hostname string) (string, error) {
	return tunnelLocalPortToRemotePort(ctx, tunnelRegistry, sshManager, fmt.Sprint("PCAP-", tunnelID), "", remotePortSSH, hostname)
}

func tunnelToBtpeer(ctx context.Context, tunnelRegistry *ssh.TunnelRegistry, sshManager *ssh.ConcurrentSSHManager, tunnelID int, hostname string) (string, error) {
	return tunnelLocalPortToRemotePort(ctx, tunnelRegistry, sshManager, fmt.Sprint("BTPEER-", tunnelID), "", remotePortSSH, hostname)
}

func tunnelToChameleon(ctx context.Context, tunnelRegistry *ssh.TunnelRegistry, sshManager *ssh.ConcurrentSSHManager, tunnelID int, hostname string) (string, error) {
	return tunnelLocalPortToRemotePort(ctx, tunnelRegistry, sshManager, fmt.Sprint("CHAMELEON-", tunnelID), "", remotePortChameleond, hostname)
}

func genericTunnelToSSHPort(ctx context.Context, tunnelRegistry *ssh.TunnelRegistry, sshManager *ssh.ConcurrentSSHManager, tunnelID int, hostname string) (string, error) {
	return tunnelLocalPortToRemotePort(ctx, tunnelRegistry, sshManager, fmt.Sprint("SSH-", tunnelID), "", remotePortSSH, hostname)
}

func genericTunnelToChameleondPort(ctx context.Context, tunnelRegistry *ssh.TunnelRegistry, sshManager *ssh.ConcurrentSSHManager, tunnelID int, hostname string) (string, error) {
	return tunnelLocalPortToRemotePort(ctx, tunnelRegistry, sshManager, fmt.Sprint("CHAMELEOND-", tunnelID), "", remotePortChameleond, hostname)
}

func tunnelToRoutersUsingDutHost(ctx context.Context, tunnelRegistry *ssh.TunnelRegistry, sshManager *ssh.ConcurrentSSHManager, hostDut string, routerCount int) ([]string, error) {
	if routerCount < 1 {
		return nil, nil
	}
	var localHostnames []string
	var localHostname string
	var err error
	localHostname, err = tunnelToRouter(ctx, tunnelRegistry, sshManager, 1, resolveHostname(hostDut, "-router"))
	if err != nil {
		return nil, err
	}
	localHostnames = append(localHostnames, localHostname)
	for i := 2; i <= routerCount; i++ {
		localHostname, err = tunnelToRouter(ctx, tunnelRegistry, sshManager, i, resolveHostname(hostDut, fmt.Sprintf("-router%d", i)))
		if err != nil {
			return nil, err
		}
		localHostnames = append(localHostnames, localHostname)
	}
	return localHostnames, err
}

func tunnelToPcapsUsingDutHost(ctx context.Context, tunnelRegistry *ssh.TunnelRegistry, sshManager *ssh.ConcurrentSSHManager, hostDut string, pcapCount int) ([]string, error) {
	if pcapCount < 1 {
		return nil, nil
	}
	var localHostnames []string
	var localHostname string
	var err error
	localHostname, err = tunnelToPcap(ctx, tunnelRegistry, sshManager, 1, resolveHostname(hostDut, "-pcap"))
	if err != nil {
		return nil, err
	}
	localHostnames = append(localHostnames, localHostname)
	for i := 2; i <= pcapCount; i++ {
		localHostname, err = tunnelToPcap(ctx, tunnelRegistry, sshManager, i, resolveHostname(hostDut, fmt.Sprintf("-pcap%d", i)))
		if err != nil {
			return nil, err
		}
		localHostnames = append(localHostnames, localHostname)
	}
	return localHostnames, nil
}

func tunnelToBtpeersUsingDutHost(ctx context.Context, tunnelRegistry *ssh.TunnelRegistry, sshManager *ssh.ConcurrentSSHManager, hostDut string, btPeerCount int) ([]string, error) {
	if btPeerCount < 1 {
		return nil, nil
	}
	var localHostnames []string
	var localHostname string
	var err error
	for i := 1; i <= btPeerCount; i++ {
		localHostname, err = tunnelToBtpeer(ctx, tunnelRegistry, sshManager, i, resolveHostname(hostDut, fmt.Sprintf("-btpeer%d", i)))
		if err != nil {
			return nil, err
		}
		localHostnames = append(localHostnames, localHostname)
	}
	return localHostnames, nil
}

func tunnelToChameleonUsingDutHost(ctx context.Context, tunnelRegistry *ssh.TunnelRegistry, sshManager *ssh.ConcurrentSSHManager, hostDut string, tunnelID int) (string, error) {
	return tunnelToChameleon(ctx, tunnelRegistry, sshManager, tunnelID, resolveHostname(hostDut, "-chameleon"))
}

func pollDUTLease(ctx context.Context, hostDut string) context.Context {
	remain, err := crosfleet.DUTLeaseTimeRemainingSeconds(ctx, hostDut)
	if err != nil {
		clog.Logger.Printf("unable to determine lease info for %s, labtunnel will run until stopped", hostDut)
		return ctx
	}
	clog.Logger.Printf("found lease for %s, labtunnel will close in %d minutes or if lease is abandoned", hostDut, remain/60)
	ctx, cancel := context.WithCancel(ctx)
	// goroutine to cancel context when lease is expired or abandoned
	go func() {
		for {
			remain, err := crosfleet.DUTLeaseTimeRemainingSeconds(ctx, hostDut)
			if err == nil && remain == 0 {
				clog.Logger.Printf("lease ended closing tunnels")
				cancel()
				return
			}
			// poll every 60 seconds or wait remaining time on lease whichever is less
			timerSecs := int(math.Min(float64(remain), 60))
			t := time.NewTimer(time.Duration(timerSecs) * time.Second)
			select {
			case <-t.C:
				// poll again after timer expires
			case <-ctx.Done():
				return
			}
		}
	}()
	return ctx
}
