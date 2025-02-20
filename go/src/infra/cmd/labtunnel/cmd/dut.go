// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"go.chromium.org/infra/cmd/labtunnel/log"
	"go.chromium.org/infra/cmd/labtunnel/ssh"
)

var (
	dutCmd = &cobra.Command{
		Use:   "dut <dut_hostname>",
		Short: "Ssh tunnel to dut.",
		Long: `
Opens an ssh tunnel to the remote ssh port to the dut as defined by
dut_hostname.

All tunnels are destroyed upon stopping labtunnel, and are restarted if
interrupted by a remote device reboot.

The dut hostname is resolved from <dut_hostname> by removing the prefix
"crossk-" if it is present.
`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tunnelRegistry := ssh.NewTunnelRegistry()
			sshManager := buildSSHManager()

			// Tunnel to dut.
			hostDut, leased, err := resolveDutHostname(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("could not determine hostname: %w", err)
			}
			localDut, err := tunnelToDut(cmd.Context(), tunnelRegistry, sshManager, 1, hostDut)
			if err != nil {
				return err
			}

			time.Sleep(time.Second)
			tunnelRegistry.PrintToLog()
			log.Logger.Printf("Example Tast call (in chroot): tast run %s <test>", localDut)
			ctx := cmd.Context()
			if leased {
				ctx = pollDUTLease(ctx, hostDut)
			}
			sshManager.WaitUntilAllSSHCompleted(ctx)
			return nil
		},
	}
)

func init() {
	rootCmd.AddCommand(dutCmd)
}
