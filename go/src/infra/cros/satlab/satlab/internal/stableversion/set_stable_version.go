// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stableversion

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/googleapis/gax-go/v2"
	"github.com/maruel/subcommands"
	moblabpb "google.golang.org/genproto/googleapis/chromeos/moblab/v1beta1"
	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"

	fleet "infra/appengine/crosskylabadmin/api/fleet/v1"
	"infra/cmd/shivas/utils"
	"infra/cmdsupport/cmdlib"
	"infra/cros/recovery/models"
	"infra/cros/satlab/common/google.golang.org/google/chromeos/moblab"
	"infra/cros/satlab/common/services/build_service"
	"infra/cros/satlab/common/site"
	"infra/cros/satlab/common/utils/misc"
)

// If allowSetModelBoard is true, then the user is allowed to create new entries for a host&model.
// If allowSetModelBoard is false, then the user is blocked from creating new entries a host&model.
//
// We set this variable to false. We want to force users to use the per-host override so that it is
// easier to replace the stable version implementation with a new service behind the scenes without
// a change in user-facing behavior.
const allowSetModelBoard = false

var SetStableVersionCmd = &subcommands.Command{
	UsageLine: `set-stable-version`,
	ShortDesc: `Set the stable version using {board, model} or {hostname}.`,
	LongDesc: `Set the stable version of ChromeOS, Firmware and Firmware Image (aka faftVersion).
	Here we have two strategies.
	The first is for partners when the user use a {board, model}.
	The partner user must also specify all of the {os, fw, fwImage} or none of them. Then we set stable version based on the live OS version.
	Second is for Googler when user uses a {hostname}. The Google user must also specify {os, fw, fwImage} xor {os, flex}.`,
	CommandRun: func() subcommands.CommandRun {
		r := &setStableVersionRun{}

		r.authFlags.Register(&r.Flags, site.DefaultAuthOptions)
		r.envFlags.Register(&r.Flags)
		r.commonFlags.Register(&r.Flags)

		// if allowSetModelBoard (
		r.Flags.StringVar(&r.board, "board", "", `the board or build target (used with model)`)
		r.Flags.StringVar(&r.model, "model", "", `the model (used with board)`)
		// )

		r.Flags.StringVar(&r.hostname, "hostname", "", `the hostname (used by itself)`)
		r.Flags.StringVar(&r.os, "os", "", `the OS version (aka crosVersion) to set (used with {-fw & -fwImage} or {-flex})`)
		r.Flags.StringVar(&r.fw, "fw", "", `the firmware version (aka firmwareVersion) to set (used with -os & -fwImage)`)
		r.Flags.StringVar(&r.fwImage, "fwImage", "", `the firmware image version (aka faftVersion) to set (used with -os & -fw)`)
		r.Flags.BoolVar(&r.isFlex, "flex", false, `force to set OS version only (internal users only for ChromeOS Flex)`)
		return r
	},
}

// SetStableVersionRun is the command for adminclient set-stable-version.
type setStableVersionRun struct {
	subcommands.CommandRunBase

	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags

	board    string
	model    string
	hostname string
	os       string
	fw       string
	fwImage  string
	isFlex   bool
}

// Run runs the command, prints the error if there is one, and returns an exit status.
func (c *setStableVersionRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	if err := c.innerRun(ctx, a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

// InnerRun implements Run using Board/Model or Hostname depending on provided args.
func (c *setStableVersionRun) innerRun(ctx context.Context, a subcommands.Application, args []string, env subcommands.Env) error {
	ctx = utils.SetupContext(ctx, c.envFlags.GetNamespace())
	stableVersion := &models.RecoveryVersion{
		Board:     c.board,
		Model:     c.model,
		OsImage:   c.os,
		FwVersion: c.fw,
		FwImage:   c.fwImage,
	}

	var innerRunStrategy func(context.Context, subcommands.Application, []string, subcommands.Env, *models.RecoveryVersion) error

	if site.IsPartner() {
		// Board Model flow: SFP EXTERNAL USERS ONLY
		innerRunStrategy = c.innerRunBoardModel
	} else {
		// Hostname flow: INTERNAL USERS ONLY
		innerRunStrategy = c.innerRunHostname
	}

	if err := innerRunStrategy(ctx, a, args, env, stableVersion); err != nil {
		return err
	}
	return nil
}

// InnerRunBoardModel is the implementation of setStableVersion that uses Board/Model and circumvents the cros-inventory call.
func (c *setStableVersionRun) innerRunBoardModel(ctx context.Context, a subcommands.Application, args []string, env subcommands.Env, rv *models.RecoveryVersion) error {

	fmt.Println("Satlab for Partners user detected...")
	numArgs, err := c.validateBoardModelArgs()
	if err != nil {
		return err
	}

	service, err := build_service.New(ctx)
	if err != nil {
		return errors.Annotate(err, "new Moblab API connector").Err()
	}
	if numArgs == 0 { // If os,fw, and fwImage not provided, use board/model to fetch arbitrary version
		rv, err = service.FindMostStableBuildByBoardAndModel(ctx, c.board, c.model)
		if err != nil {
			return errors.Annotate(err, "find most stable build").Err()
		}
	} else if numArgs < 3 { // If partial args provided, throw an error
		return fmt.Errorf("Please provide all or none of the following: -os, -fw, -fwImage")
	}
	if err := misc.StageAndWriteLocalStableVersion(ctx, service, rv); err != nil {
		return errors.Annotate(err, "stage and write local stable version").Err()
	}
	return nil
}

// InnerRunHostname is the implementation of setStableVersion for internal Satlab users that requires hostname et al.
func (c *setStableVersionRun) innerRunHostname(ctx context.Context, a subcommands.Application, args []string, env subcommands.Env, rv *models.RecoveryVersion) error {

	fmt.Println("Internal Satlab user detected...")
	if err := c.validateHostnameArgs(); err != nil {
		return err
	}
	newHostname, err := preprocessHostname(ctx, c.commonFlags, c.hostname, nil, nil)
	if err != nil {
		return errors.Annotate(err, "preprocess hostname").Err()
	}
	c.hostname = newHostname

	req, err := c.produceRequest(ctx, a, args, env)
	if err != nil {
		return errors.Annotate(err, "produce request").Err()
	}

	hc, err := cmdlib.NewHTTPClient(ctx, &c.authFlags)
	if err != nil {
		return errors.Annotate(err, "create http client").Err()
	}

	invWithSVClient := fleet.NewInventoryPRPCClient(
		&prpc.Client{
			C:       hc,
			Host:    c.envFlags.GetCrosAdmService(),
			Options: site.DefaultPRPCOptions,
		},
	)

	resp, err := invWithSVClient.SetSatlabStableVersion(ctx, req)
	if err != nil {
		return errors.Annotate(err, "get stable version").Err()
	}
	_, err = protojson.MarshalOptions{
		Indent: "  ",
	}.Marshal(resp)
	if err != nil {
		return errors.Annotate(err, "get stable version").Err()
	}

	stableVersion, _ := json.MarshalIndent(rv, "", " ")
	fmt.Println("-- Stable Version set successfully --\n", string(stableVersion))
	return nil
}

func (c *setStableVersionRun) validateBoardModelArgs() (int, error) {
	if c.isFlex {
		return 0, errors.Reason("Please do not use -flex when logged in as a Partner").Err()
	}
	if c.board == "" {
		return 0, errors.Reason("Please provide -board").Err()
	}
	if c.model == "" {
		return 0, errors.Reason("Please provide -model").Err()
	}
	count := 0
	if c.os != "" {
		count++
	}
	if c.fw != "" {
		count++
	}
	if c.fwImage != "" {
		count++
	}
	return count, nil
}

func (c *setStableVersionRun) validateHostnameArgs() error {
	if c.hostname == "" {
		return errors.Reason("Please provide -hostname of DUT").Err()
	}
	if c.os == "" {
		return errors.Reason("Please provide the OS version (aka crosVersion) -os").Err()
	}
	if c.isFlex {
		if c.fw != "" || c.fwImage != "" {
			return errors.Reason("Please do not use -flex with -fw nor -fwImage").Err()
		}
		// ChromeOS Flex do not have the fw and the fw_image, so for these cases we need to skip these checks.
		return nil
	}
	if c.fw == "" {
		return errors.Reason("Please provide the firmware version (aka firmwareVersion) -fw").Err()
	}
	if c.fwImage == "" {
		return errors.Reason("Please provide the firmware image version (aka faftVersion) -fwImage").Err()
	}
	return nil
}

// ProduceRequest creates a request that can be used as a key to set the stable version.
// If the command line arguments do not unambiguously indicate how to create such a request, we fail.
func (c *setStableVersionRun) produceRequest(ctx context.Context, a subcommands.Application, args []string, env subcommands.Env) (*fleet.SetSatlabStableVersionRequest, error) {
	req := &fleet.SetSatlabStableVersionRequest{}
	useHostnameStrategy := c.hostname != ""
	useBoardModelStrategy := allowSetModelBoard && (c.board != "") && (c.model != "")

	// Validate and populate the strategy field of the request.
	if err := func() error {
		if useHostnameStrategy {
			if useBoardModelStrategy {
				return errors.Reason("board and model should not be set if hostname is provided").Err()
			}
			req.Strategy = &fleet.SetSatlabStableVersionRequest_SatlabHostnameStrategy{
				SatlabHostnameStrategy: &fleet.SatlabHostnameStrategy{
					Hostname: c.hostname,
				},
			}
			return nil
		} // Hostname strategy not set.
		if !useBoardModelStrategy {
			return errors.Reason("must provide hostname").Err()
		}
		req.Strategy = &fleet.SetSatlabStableVersionRequest_SatlabBoardAndModelStrategy{
			SatlabBoardAndModelStrategy: &fleet.SatlabBoardAndModelStrategy{
				Board: c.board,
				Model: c.model,
			},
		}
		return nil
	}(); err != nil {
		return nil, err
	}

	// TODO(gregorynisbet): Consider adding validation here instead of on the server side
	req.CrosVersion = c.os
	req.FirmwareVersion = c.fw
	req.FirmwareImage = c.fwImage

	return req, nil
}

// MoblabClient interface provides subset of Moblab API methods relevant to Stable Version functionality
type MoblabClient interface {
	FindMostStableBuild(ctx context.Context, req *moblabpb.FindMostStableBuildRequest, opts ...gax.CallOption) (*moblabpb.FindMostStableBuildResponse, error)
	ListBuilds(ctx context.Context, req *moblabpb.ListBuildsRequest, opts ...gax.CallOption) *moblab.BuildIterator
	StageBuild(ctx context.Context, req *moblabpb.StageBuildRequest, opts ...gax.CallOption) (*moblab.StageBuildOperation, error)
	CheckBuildStageStatus(ctx context.Context, req *moblabpb.CheckBuildStageStatusRequest, opts ...gax.CallOption) (*moblabpb.CheckBuildStageStatusResponse, error)
}
