// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"go.chromium.org/infra/cmd/labtunnel/log"
	"go.chromium.org/infra/cmd/labtunnel/ssh"
)

var (
	configFile string

	powerhtlCmd = &cobra.Command{
		Use:   "powerhtl <dut_hostname> --config <servo subrail config> --satlab <satlab>",
		Short: "Ssh tunnel to the DUT and servod of a power HTL testbed.",
		Long: `
Starts servod on power htl satlab & opens an ssh tunnels to remote ssh port and servod.
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

			// Start servod docker container & tunnel to it
			satlab, servodContainer, ip, port, err := startSatlabServod(cmd.Context(), "", hostDut, configFile)
			if err != nil {
				return err
			}
			defer func() {
				// The cmd.Context() may be invalid due to SIGINT, use a new one
				// for stopping the container.
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				err = stopSatlabServod(ctx, satlab, servodContainer)
				if err != nil {
					log.Logger.Printf("Warning, error while stopping servod container: %v", err)
				}
			}()
			localServod, err := tunnelLocalPortToRemotePort(cmd.Context(), tunnelRegistry, sshManager, "SERVOD", ip, port, satlab)
			if err != nil {
				return err
			}

			time.Sleep(time.Second)
			tunnelRegistry.PrintToLog()
			log.Logger.Printf(
				"Example Tast call (in chroot): tast run -var=servo=%s %s <test>",
				localServod, localDut)
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
	rootCmd.AddCommand(powerhtlCmd)
	powerhtlCmd.Flags().StringVar(&configFile, "config", "", "config.xml of subrail info in hdctools, ex: rex_r2.xml")
}
