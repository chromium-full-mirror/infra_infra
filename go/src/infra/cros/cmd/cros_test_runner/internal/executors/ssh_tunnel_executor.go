// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executors

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/internal/commands"
)

// SshTunnelExecutor represents executor for all ssh related commands.
type SshTunnelExecutor struct {
	*interfaces.AbstractExecutor

	SshTunnelCmd        *exec.Cmd
	SshReverseTunnelCmd *exec.Cmd
}

func NewSshTunnelExecutor() *SshTunnelExecutor {
	absExec := interfaces.NewAbstractExecutor(SshTunnelExecutorType)
	return &SshTunnelExecutor{AbstractExecutor: absExec}
}

func (ex *SshTunnelExecutor) ExecuteCommand(
	ctx context.Context,
	cmdInterface interfaces.CommandInterface) error {

	switch cmd := cmdInterface.(type) {
	case *commands.SshStartTunnelCmd:
		return ex.sshStartTunnelExecution(ctx, cmd)
	case *commands.SshStartReverseTunnelCmd:
		return ex.sshStartReverseTunnelExecution(ctx, cmd)
	case *commands.SshStopTunnelsCmd:
		return ex.sshStopTunnelsExecution(ctx, cmd)
	default:
		return fmt.Errorf("Command type %s is not supported by %s executor type!", cmd.GetCommandType(), ex.GetExecutorType())
	}
}

// sshStopTunnelsExecution executes the ssh stop tunnels command.
func (ex *SshTunnelExecutor) sshStopTunnelsExecution(
	ctx context.Context,
	cmd *commands.SshStopTunnelsCmd) error {

	var err error
	step, ctx := build.StartStep(ctx, "Stop SSH Tunnels")
	defer func() { step.End(err) }()

	if ex.SshTunnelCmd != nil {
		if err = ex.SshTunnelCmd.Process.Signal(os.Interrupt); err != nil {
			logging.Infof(ctx, "Failed to stop SSH Tunnel: %s", err)
		}
	}

	if ex.SshReverseTunnelCmd != nil {
		if err = ex.SshReverseTunnelCmd.Process.Signal(os.Interrupt); err != nil {
			logging.Infof(ctx, "Failed to stop SSH Reverse Tunnel: %s", err)
		}
	}

	return err
}

// sshStartTunnelExecution executes the ssh start tunnel command.
func (ex *SshTunnelExecutor) sshStartTunnelExecution(
	ctx context.Context,
	cmd *commands.SshStartTunnelCmd) error {

	var err error
	step, ctx := build.StartStep(ctx, "Start SSH Tunnel")
	defer func() { step.End(err) }()

	writer := step.Log("SSH Tunnel")
	cmd.SshTunnelPort = common.GetFreePort()
	ex.SshTunnelCmd = exec.Command(
		"autossh",
		"-M",
		"0",
		"-o",
		"ServerAliveInterval=5",
		"-o",
		"ServerAliveCountMax=2",
		"-o",
		"UserKnownHostsFile=/dev/null",
		"-o",
		"StrictHostKeyChecking=no",
		"-tt",
		"-L",
		fmt.Sprintf("%d:localhost:%d", cmd.SshTunnelPort, common.DutConnectionPort),
		fmt.Sprintf("root@%s", cmd.HostName),
		"-N",
	)

	go func() {
		if err := common.RunCommandWithCustomWriter(ctx, ex.SshTunnelCmd, "Start SSH Tunnel", writer); err != nil {
			logging.Infof(ctx, "error during starting ssh tunnel: %s", err.Error())
		}
	}()

	if err = waitForConnection(cmd.SshTunnelPort); err != nil {
		err = errors.Annotate(err, "Failed to connect to SSH Tunnel: ").Err()
		logging.Infof(ctx, "%s", err.Error())
	}

	return err
}

// sshStartTunnelExecution executes the ssh start tunnel command.
func (ex *SshTunnelExecutor) sshStartReverseTunnelExecution(
	ctx context.Context,
	cmd *commands.SshStartReverseTunnelCmd) error {

	var err error
	step, ctx := build.StartStep(ctx, "Start SSH Reverse Tunnel")
	defer func() { step.End(err) }()

	writer := step.Log("SSH Reverse Tunnel")
	cmd.SshReverseTunnelPort = common.GetFreePort()
	ex.SshReverseTunnelCmd = exec.Command(
		"autossh",
		"-M",
		"0",
		"-o",
		"ServerAliveInterval=5",
		"-o",
		"ServerAliveCountMax=2",
		"-o",
		"UserKnownHostsFile=/dev/null",
		"-o",
		"StrictHostKeyChecking=no",
		"-tt",
		"-R",
		fmt.Sprintf(
			"%d:localhost:%d",
			cmd.SshReverseTunnelPort,
			cmd.CacheServerPort),
		fmt.Sprintf("root@%s", cmd.HostName),
		"-p",
		fmt.Sprint(common.DutConnectionPort),
		"-N",
	)

	go func() {
		if err := common.RunCommandWithCustomWriter(ctx, ex.SshReverseTunnelCmd, "Start SSH Reverse Tunnel", writer); err != nil {
			logging.Infof(ctx, "error during starting ssh reverse tunnel: %s", err.Error())
		}
	}()

	return err
}

func waitForConnection(port uint16) error {
	var err error
	for st := time.Now(); time.Since(st) < time.Second*10; time.Sleep(time.Millisecond * 250) {
		conn, innerErr := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
		err = innerErr
		if err == nil {
			conn.Close()
			break
		}
	}

	return err
}
