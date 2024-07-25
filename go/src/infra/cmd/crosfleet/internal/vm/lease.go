// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"flag"
	"fmt"
	"net/mail"
	"sort"
	"strings"

	"github.com/maruel/subcommands"
	"google.golang.org/protobuf/types/known/durationpb"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/cli"

	"infra/cmd/crosfleet/internal/common"
	"infra/cmdsupport/cmdlib"
	croscommon "infra/cros/cmd/common_lib/common"
	"infra/libs/vmlab"
	vmapi "infra/libs/vmlab/api"
	"infra/vm_leaser/client"
)

const (
	// maxLeaseLengthMinutes is 24 hours in minutes.
	maxLeaseLengthMinutes = 24 * 60
	// maxDiskSize to protect GCE resources (in GB).
	maxDiskSize = 200
	// leaseCmdName is the name of the `crosfleet vm lease` command.
	leaseCmdName = "lease"
)

// leaseFlags contains parameters for the "vm lease" subcommand.
type leaseFlags struct {
	durationMins int64
	board        string
	build        string
	diskSize     int64
}

// Registers lease-specific flags.
func (c *leaseFlags) register(f *flag.FlagSet) {
	f.Int64Var(&c.durationMins, "minutes", 60, "Duration of lease in minutes.")
	f.StringVar(&c.board, "board", "", "Board name for the VM image, for example betty-arc-r, the latest release image will be used")
	f.StringVar(&c.build, "build", "", "Build path of the VM image, for example betty-arc-r-release/R119-15626.0.0, should not be used with -board")
	f.Int64Var(&c.diskSize, "disk-size", 13, "Disk size of VM in GB.")
}

var lease = &subcommands.Command{
	UsageLine: fmt.Sprintf("%s [FLAGS...]", leaseCmdName),
	ShortDesc: "lease VM for debugging",
	LongDesc: `Lease VM for debugging.

VMs can be leased by either specifying a VM board via -board or a VM image name
via -image.

This command's behavior is subject to change without notice.
Do not build automation around this subcommand.`,
	CommandRun: func() subcommands.CommandRun {
		c := &leaseRun{}
		c.envFlags.register(&c.Flags)
		c.leaseFlags.register(&c.Flags)
		return c
	},
}

type leaseRun struct {
	subcommands.CommandRunBase
	envFlags
	leaseFlags
	printer common.CLIPrinter
}

func (c *leaseRun) Run(a subcommands.Application, _ []string, env subcommands.Env) int {
	if err := c.innerRun(a, env); err != nil {
		common.PrintCmdError(a, err)
		return 1
	}
	return 0
}

func (c *leaseRun) innerRun(a subcommands.Application, env subcommands.Env) error {
	if err := c.leaseFlags.validate(&c.Flags); err != nil {
		return err
	}
	ctx := cli.GetContext(a, c, env)

	config, err := c.envFlags.getClientConfig()
	if err != nil {
		return err
	}
	vmLeaser, err := client.NewClient(ctx, config)
	if err != nil {
		return err
	}
	defer vmLeaser.Close()

	if _, err = mail.ParseAddress(vmLeaser.Email); err != nil {
		return fmt.Errorf("Failed to validate email of current user: %v", err)
	}

	var image string
	if c.board != "" {
		iapi, err := vmlab.NewImageApi(vmapi.ProviderId_CLOUDSDK)
		if err != nil {
			return err
		}
		latestImage, err := getLatestImage(iapi, c.board)
		if err != nil {
			return err
		}
		image = latestImage
	} else {
		c.printer.WriteTextStdout(fmt.Sprintf("Importing VM image for build %s\nThis will take about 5 minutes if the image was not imported before.", c.build))
		resp, err := vmLeaser.VMLeaserClient.ImportImage(ctx, &api.ImportImageRequest{
			ImagePath: c.build,
		})
		if err != nil {
			return fmt.Errorf("Image import returned error: %w\nPlease verify if the build path is correct.", err)
		}
		image = resp.ImageName
	}
	c.printer.WriteTextStdout(fmt.Sprintf("Leasing VM with image %s", image))

	imageName := fmt.Sprintf("projects/%s/global/images/%s", croscommon.GceProject, image)
	resp, err := vmLeaser.VMLeaserClient.LeaseVM(ctx, &api.LeaseVMRequest{
		LeaseDuration: &durationpb.Duration{
			Seconds: c.durationMins * 60,
		},
		HostReqs: &api.VMRequirements{
			GceImage:                 imageName,
			GceProject:               croscommon.GceProject,
			GceNetwork:               croscommon.GceNetwork,
			GceMachineType:           croscommon.GceMachineTypeN14,
			SubnetModeNetworkEnabled: true,
			GceDiskSize:              c.diskSize,
		},
		TestingClient: api.VMTestingClient_VM_TESTING_CLIENT_CROSFLEET,
		Labels: map[string]string{
			"client":    "crosfleet",
			"leased-by": sanitizeForLabel(vmLeaser.Email),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to lease VM: %w", err)
	}

	c.printer.WriteTextStdout("Successfully created VM")
	c.printer.WriteTextStdout(fmt.Sprintf("Instance name: %s", resp.GetVm().GetId()))
	c.printer.WriteTextStdout(fmt.Sprintf("Region: %s", resp.GetVm().GetGceRegion()))
	c.printer.WriteTextStdout(fmt.Sprintf("IP address: %s", resp.GetVm().GetAddress().GetHost()))
	c.printer.WriteTextStdout(fmt.Sprintf("SSH port: %d", resp.GetVm().GetAddress().GetPort()))
	c.printer.WriteTextStdout(fmt.Sprintf("Lease time: %d minutes", c.durationMins))
	c.printer.WriteTextStdout("Visit http://go/chromeos-lab-vms-ssh for up-to-date docs on SSHing to a leased VM")
	return nil
}

// getLatestImage retrieves the latest VM image for a specific board.
//
// This function gets the latest VM image in terms of build number. This is
// because previous builds may be rebuilt after a newer build is released. We
// want to always use the latest released image instead.
func getLatestImage(iapi vmapi.ImageApi, board string) (string, error) {
	images, err := iapi.ListImages(fmt.Sprintf("(labels.build-type:release AND labels.board:%s)", board))
	if err != nil {
		return "", err
	}
	if len(images) == 0 {
		return "", fmt.Errorf("Cannot find any images for board %s", board)
	}
	sort.SliceStable(images, func(i, j int) bool {
		return images[i].GetName() > images[j].GetName()
	})
	return images[0].GetName(), nil
}

func (c *leaseFlags) validate(f *flag.FlagSet) error {
	var errors []string
	if c.durationMins <= 0 {
		errors = append(errors, "duration should be greater than 0")
	}
	if c.durationMins > maxLeaseLengthMinutes {
		errors = append(errors, fmt.Sprintf("duration cannot exceed %d minutes (%d hours)", maxLeaseLengthMinutes, maxLeaseLengthMinutes/60))
	}
	if c.board == "" && c.build == "" {
		errors = append(errors, "either -board or -build must be specified")
	}
	if c.board != "" && c.build != "" {
		errors = append(errors, "-board and -build should not be used together")
	}
	if c.diskSize <= 0 {
		errors = append(errors, "disk size should be greater than 0")
	}
	if c.diskSize > maxDiskSize {
		errors = append(errors, fmt.Sprintf("disk size cannot exceed %d GB", maxDiskSize))
	}

	if len(errors) > 0 {
		return cmdlib.NewUsageError(*f, strings.Join(errors, "\n"))
	}
	return nil
}
