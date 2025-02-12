// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cmd

import (
	"time"

	"github.com/spf13/cobra"

	"infra/cmd/labtunnel/ssh"
)

var (
	sshWatcherCmd = &cobra.Command{
		Use:   "sshwatcher host1 [host2 [... hostN]]",
		Short: "Ssh tunnel to host(s).",
		Long: `
Opens an ssh tunnel to the remote ssh port to specified host(s).

All tunnels are destroyed upon stopping labtunnel, and are restarted if
interrupted by a remote device reboot.

This is command is comparable to the independent "sshwatcher" utility, which
does the same thing. In this version, the local forwarded port does not have
to be specified and is selected like it is for other tunnels in labtunnel. See
"labtunnel --help" for more details on port selection.
`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tunnelRegistry := ssh.NewTunnelRegistry()
			sshManager := buildSSHManager()

			// Tunnel to the specified hosts.
			for i, host := range args {

				if _, err := genericTunnelToSSHPort(cmd.Context(), tunnelRegistry, sshManager, i+1, host); err != nil {
					return err
				}
			}

			time.Sleep(time.Second)
			tunnelRegistry.PrintToLog()
			sshManager.WaitUntilAllSSHCompleted(cmd.Context())
			return nil
		},
	}
)

func init() {
	rootCmd.AddCommand(sshWatcherCmd)
}
