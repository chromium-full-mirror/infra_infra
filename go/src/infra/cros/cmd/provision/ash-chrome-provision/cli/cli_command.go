// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Represents the CLI command grouping
package cli

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/pkg/errors"

	"go.chromium.org/chromiumos/config/go/longrunning"
	"go.chromium.org/chromiumos/config/go/test/api"

	commonutils "go.chromium.org/infra/cros/cmd/provision/common-utils"
)

const (
	// version is the version info of this command. It is filled in during emerge.
	version             = "<unknown>"
	defaultLogDirectory = "/tmp/ash-chrome-provision/"
	defaultPort         = 80
)

// CLI command executed the provisioning as a CLI
type CLICommand struct {
	logFileName string
	log         *log.Logger
	startupFile string
	installFile string
	flagSet     *flag.FlagSet
}

func NewCLICommand() *CLICommand {
	cc := &CLICommand{
		flagSet: flag.NewFlagSet("cli", flag.ContinueOnError),
	}

	cc.flagSet.StringVar(&cc.logFileName, "log-path", defaultLogDirectory, fmt.Sprintf("Path to record execution logs. Default value is %s", defaultLogDirectory))
	cc.flagSet.StringVar(&cc.startupFile, "startup", "", "Specify the StartUp ProvisionStartupRequest jsonproto input file.")
	cc.flagSet.StringVar(&cc.installFile, "install", "", "Specify the Install InstallRequest jsonproto input file.")
	return cc
}

func (cc *CLICommand) Is(group string) bool {
	return strings.HasPrefix(group, "cli")
}

func (cc *CLICommand) Name() string {
	return "cli"
}

func (cc *CLICommand) Init(args []string) error {
	err := cc.flagSet.Parse(args)
	if err != nil {
		return err
	}

	cc.log, err = SetUpLog(cc.logFileName)
	if err != nil {
		return err
	}

	if err = cc.validate(); err != nil {
		return err
	}

	return nil
}

// Logger returns the log
func (cc *CLICommand) Logger() *log.Logger {
	return cc.log
}

// validate checks if inputs are ok
func (cc *CLICommand) validate() error {
	if cc.startupFile == "" {
		return errors.New("startup file not specified, use -startup")
	}

	if cc.installFile == "" {
		return errors.New("install file not specified, use -install")
	}
	return nil
}

// Run starts the ash chrome provision and does a single provision.
func (cc *CLICommand) Run() error {
	cc.log.Printf("Running CLI Mode (V2) - Ash-Chrome Provision:")

	ps, closer, err := NewAshChromeProvisionServer(0, cc.log)
	defer closer()
	if err != nil {
		cc.log.Println("Failed to create provision server: ", err)
		return err
	}

	startupProto, err := commonutils.ParseProvisionStartupRequest(cc.startupFile)
	if err != nil {
		return errors.Wrap(err, "unable to parse ProvisionStartupRequest proto")
	}
	installProto, err := commonutils.ParseInstallRequest(cc.installFile)
	if err != nil {
		return errors.Wrap(err, "unable to parse InstallRequest proto")
	}

	ctx := context.Background()
	startupRes, err := ps.StartUp(ctx, startupProto)
	if err != nil {
		return errors.Wrap(err, "failed to call StartUp")
	}
	cc.log.Printf("StartUp complete - %s", startupRes.String())

	op, err := ps.Install(ctx, installProto)
	if err != nil {
		return errors.Wrap(err, "failed to call Install")
	}
	cc.log.Printf("Install LRO - %s", op.String())
	op, err = ps.manager.WaitOperation(ctx, &longrunning.WaitOperationRequest{
		Name: op.Name,
	})
	if err != nil {
		return errors.Wrap(err, "failed to wait operation")
	}
	cc.log.Printf("Install complete - %s", op.String())
	if !op.Done {
		return errors.Errorf("operation not done: %s", op.String())
	}
	if op.GetError() != nil {
		return errors.Errorf("installed failed: %s", op.GetError().String())
	}
	if op.GetResponse() == nil {
		return errors.Errorf("no response returned: %s", op)
	}
	installResp := new(api.InstallResponse)
	if !op.GetResponse().MessageIs(installResp) {
		return errors.Errorf("unexpected response: %s", op.GetResponse().String())
	}
	if err := op.GetResponse().UnmarshalTo(installResp); err != nil {
		return errors.Wrap(err, "failed to unmarshal response")
	}
	if installResp.Status != api.InstallResponse_STATUS_SUCCESS {
		return errors.Errorf("install failed: %v", installResp.Status)
	}
	return nil
}
