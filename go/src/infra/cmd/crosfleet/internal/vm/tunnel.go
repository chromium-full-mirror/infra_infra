// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"syscall"
	"time"

	"github.com/maruel/subcommands"
	"golang.org/x/crypto/ssh"

	"go.chromium.org/luci/common/cli"

	"go.chromium.org/infra/cmd/crosfleet/internal/common"
	"go.chromium.org/infra/vm_leaser/client"
)

const tunnelCmd = "tunnel"

var tunnel = &subcommands.Command{
	UsageLine: fmt.Sprintf("%s [FLAGS...]", tunnelCmd),
	ShortDesc: "Starts a tunnel to the VM.",
	LongDesc: `Starts a tunnel to the VM.

If there is only one active lease, omitting -address will select it.
Don't run this command inside CrOS chroot as it requires full version of corp-ssh-helper.

This command's behavior is subject to change without notice.
Do not build automation around this subcommand.`,
	CommandRun: func() subcommands.CommandRun {
		c := &tunnelRun{}
		c.envFlags.register(&c.Flags)
		c.tunnelFlags.register(&c.Flags)
		return c
	},
}

type tunnelRun struct {
	subcommands.CommandRunBase
	envFlags
	tunnelFlags
}

func (c *tunnelRun) Run(a subcommands.Application, _ []string, env subcommands.Env) int {
	if err := c.innerRun(a, env); err != nil {
		common.PrintCmdError(a, err)
		fmt.Fprintln(os.Stderr, "Visit http://go/chromeos-lab-vms-ssh for up-to-date docs on SSHing to a leased VM")
		return 1
	}
	return 0
}

func (c *tunnelRun) innerRun(a subcommands.Application, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)

	address := c.tunnelFlags.address

	if address == "" {
		config, err := c.envFlags.getClientConfig()
		if err != nil {
			return err
		}
		vmLeaser, err := client.NewClient(ctx, config)
		if err != nil {
			return err
		}
		defer vmLeaser.Close()

		vms, err := listLeases(vmLeaser, ctx)
		if err != nil {
			return err
		}

		if len(vms) == 0 {
			return errors.New("No active VM leases")
		}

		if len(vms) > 1 {
			printVMList(vms, os.Stdout)
			return fmt.Errorf("There are %d active leases, please select one using -address", len(vms))
		}

		fmt.Printf("Selected %s\n", vms[0].GetId())
		address = vms[0].Address.GetHost()
	}

	return c.startSSHProxy(address, c.tunnelFlags.port)
}

// startSSHProxy starts a TCP tunnel to the destination port on the GCP VPC
// network via the shared bastion host.
func (c *tunnelRun) startSSHProxy(address string, port int) error {
	const bastionHost = "nic0.crosfleet-bastion.us-central1-a.c.chromeos-gce-tests.internal.gcpnode.com"
	const bastionPort = "22"

	config, err := c.createSSHConfig()
	if err != nil {
		return err
	}

	phelper, pssh := net.Pipe()

	fmt.Println("Starting corp-ssh-helper")

	helper := exec.Command(
		"/usr/bin/corp-ssh-helper",
		"-relay=sup-ssh-relay.corp.google.com",
		"--proxy-mode=grue",
		bastionHost, bastionPort,
	)
	helper.Stdin = phelper
	helper.Stdout = phelper
	helper.Stderr = os.Stderr

	if err := helper.Start(); err != nil {
		return fmt.Errorf("Unable to start corp-ssh-helper: %w", err)
	}

	go func() {
		helper.Wait()
	}()

	// It takes time for corp-ssh-helper to start even if it fails (e.g. no corp
	// ssh certificate). Waiting for some time here to make sure corp-ssh-helper
	// is up and running.
	time.Sleep(3 * time.Second)
	if err := helper.Process.Signal(syscall.Signal(0)); err != nil {
		return fmt.Errorf("corp-ssh-helper exited immediately: %w", err)
	}

	fmt.Println("Connecting to bastion host")

	conn, in, out, err := ssh.NewClientConn(pssh, net.JoinHostPort(bastionHost, bastionPort), config)
	if err != nil {
		return fmt.Errorf("Unable to create channel through corp-ssh-helper: %w", err)
	}
	defer conn.Close()

	client := ssh.NewClient(conn, in, out)
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("Unable to start SSH session to bastion host: %w", err)
	}
	defer session.Close()

	listener, err := openNextAvailablePort()
	if err != nil {
		return fmt.Errorf("Unable to listen to local port: %w", err)
	}
	defer listener.Close()

	fmt.Printf("Ready to accept connections on %s forwarding to %s:%d\n", listener.Addr().String(), address, port)
	fmt.Println("Exiting this program will close all connections.")

	for {
		local, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("Unable to accept connection: %w", err)
		}
		go func(local net.Conn) {
			defer local.Close()

			remote, err := client.Dial("tcp", fmt.Sprintf("%s:%d", address, port))
			if err != nil {
				log.Fatalf("Unable to connect to %s:%d through bastion host: %v", address, port, err)
			}
			defer remote.Close()

			done := make(chan struct{}, 2)
			go func() {
				io.Copy(local, remote)
				done <- struct{}{}
			}()
			go func() {
				io.Copy(remote, local)
				done <- struct{}{}
			}()

			<-done
		}(local)
	}
}

// createSSHConfig creates the SSH config for the shared bastion host.
func (c *tunnelRun) createSSHConfig() (*ssh.ClientConfig, error) {
	user, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("Unable to get current user: %w", err)
	}

	var sshUser string
	if c.tunnelFlags.user != "" {
		sshUser = c.tunnelFlags.user
	} else {
		sshUser = user.Username + "_google_com"
	}

	// Read the SSH private key generated by gcloud CLI. OS Login requires this
	// key.
	gceKeyFile := user.HomeDir + "/.ssh/google_compute_engine"
	gceKey, err := os.ReadFile(gceKeyFile)
	if err != nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(gceKey)
	if err != nil {
		return nil, fmt.Errorf("Could not read GCE SSH private key")
	}

	return &ssh.ClientConfig{
		User: sshUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}, nil
}

// openNextAvailablePort finds the next available port and open for listening.
func openNextAvailablePort() (net.Listener, error) {
	const defaultPort = 9333

	localPort := defaultPort
	for localPort < 65536 {
		l, err := net.Listen("tcp", "localhost:"+fmt.Sprint(localPort))
		if err == nil {
			return l, nil
		}
		localPort += 1
	}
	return nil, errors.New("Cannot find an available port for listening")
}

func hostKeyCallback(string, net.Addr, ssh.PublicKey) error {
	return nil
}

// tunnelFlags contains parameters for the "vm tunnel" subcommand.
type tunnelFlags struct {
	address    string
	port       int
	listenAddr string
	user       string
}

// Registers tunnel-specific flags.
func (c *tunnelFlags) register(f *flag.FlagSet) {
	f.StringVar(&c.address, "address", "", "IP address of the instance. Leave empty to auto select.")
	f.IntVar(&c.port, "port", 22, "Port to tunnel to.")
	f.StringVar(&c.user, "user", "", "Custom user name for the bastion host. Normally it's not needed.")
}
