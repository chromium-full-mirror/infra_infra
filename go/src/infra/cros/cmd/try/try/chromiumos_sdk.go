// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package try

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/internal/cmd"
	bb "go.chromium.org/infra/cros/lib/buildbucket"
)

func GetCmdChromiumOSSDK(authOpts auth.Options) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "chromiumos_sdk --branch BRANCH [flags]",
		ShortDesc: "Run a ChromiumOS SDK builder.",
		CommandRun: func() subcommands.CommandRun {
			c := &chromiumOSSDKRun{}
			c.cmdRunner = cmd.RealCommandRunner{}
			c.tryRunBase.authOpts = authOpts
			c.addDryrunFlag()
			c.addBranchFlag("")
			c.addPatchesFlag()
			c.addProductionFlag()
			c.addLaunchPUprFlag()
			c.addCQPolicyFlag()
			return c
		},
	}
}

// chromiumOSSDKRun tracks relevant info for a given `try chromiumos_sdk` run.
type chromiumOSSDKRun struct {
	tryRunBase
	propsFile  *os.File
	launchPUpr bool
	cqPolicy   string
}

func (r *chromiumOSSDKRun) addLaunchPUprFlag() {
	r.tryRunBase.Flags.BoolVar(
		&r.launchPUpr,
		"launch-pupr",
		false,
		"If given, the build will launch a PUpr build to create CL(s) that uprev to the built SDK. However, the uprev CLs might not work as expected on staging!",
	)
}

func (r *chromiumOSSDKRun) addCQPolicyFlag() {
	acceptableValues := [6]string{"do-nothing", "dry-run", "full-run", "abandon", "submit", ""}
	usage := fmt.Sprintf("Specify what PUpr should do with generated CLs. Acceptable values: %+v. Irrelevant if PUpr is not launched.", acceptableValues)
	r.tryRunBase.Flags.Func("cq-policy", usage, func(s string) error {
		for _, v := range acceptableValues {
			if s == v {
				r.cqPolicy = s
				return nil
			}
		}
		return fmt.Errorf("invalid cq-policy %s", s)
	})
}

// Run provides the logic for a `try chromiumos_sdk` command run.
func (r *chromiumOSSDKRun) Run(_ subcommands.Application, _ []string, _ subcommands.Env) int {
	r.stdoutLog = log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)
	r.stderrLog = log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)
	ctx := context.Background()

	// Do not create a gerritClient for test structs with a mockClient.
	if r.gerritClient == nil {
		if err := r.createGerritClient(r.authOpts); err != nil {
			r.LogErr(err.Error())
			return CmdError
		}
	}

	if err := r.validate(ctx); err != nil {
		r.LogErr(err.Error())
		return CmdError
	}

	// Need to call run first to do LUCI auth / set up other shared constructs.
	if ret, err := r.run(ctx); err != nil {
		r.LogErr(err.Error())
		return ret
	}

	// User email is used for setting BranchPolicy.
	// This isn't always necessary, but testing is easier if we can always spoof this command's output.
	userEmail, err := r.getUserEmail(ctx)
	if err != nil {
		r.LogErr(errors.Annotate(err, "getting user's email address").Err().Error())
		return CmdError
	}

	// Set properties.
	propsStruct, err := r.bbClient.GetBuilderInputProps(ctx, r.getBuilderFullName())
	if err != nil {
		r.LogErr(err.Error())
		return CmdError
	}
	if r.branch != "" {
		if err := bb.SetProperty(propsStruct, "manifest_branch", r.branch); err != nil {
			r.LogErr(err.Error())
			return CmdError
		}
	}
	if err := bb.SetProperty(propsStruct, "launch_pupr", r.launchPUpr); err != nil {
		r.LogErr(err.Error())
		return CmdError
	}
	if r.cqPolicy != "" {
		branchPolicy := createBranchPolicy(r.cqPolicy, userEmail)
		if err := bb.SetProperty(propsStruct, "pupr_branch_policy", branchPolicy); err != nil {
			r.LogErr(err.Error())
			return CmdError
		}
	}

	var propsFile *os.File
	if r.propsFile != nil {
		propsFile = r.propsFile
	} else {
		propsFile, err = os.CreateTemp("", "input_props")
		if err != nil {
			r.LogErr(err.Error())
			return CmdError
		}
	}
	if err := bb.WriteStructToFile(propsStruct, propsFile); err != nil {
		r.LogErr(errors.Annotate(err, "writing input properties to tempfile").Err().Error())
		return UnspecifiedError
	}
	if r.propsFile == nil {
		defer os.Remove(propsFile.Name())
	}
	r.bbAddArgs = append(r.bbAddArgs, "-p", fmt.Sprintf("@%s", propsFile.Name()))

	if len(r.patches) > 0 {
		r.bbAddArgs = append(r.bbAddArgs, patchListToBBAddArgs(r.patches)...)
	}
	if err := r.runBuilder(ctx); err != nil {
		r.LogErr(err.Error())
		return CmdError
	}
	return Success
}

// validate validates args for the command.
func (r *chromiumOSSDKRun) validate(ctx context.Context) error {
	return r.tryRunBase.validate()
}

// getBuilderFullName finds the full builder name (<project>/<bucket>/<builder>).
func (r *chromiumOSSDKRun) getBuilderFullName() string {
	var bucket, stagingPrefix string
	if r.production {
		bucket = "infra"
	} else {
		bucket = "staging"
		stagingPrefix = "staging-"
	}
	return fmt.Sprintf("chromeos/%s/%sbuild-chromiumos-sdk", bucket, stagingPrefix)
}

// runBuilder creates a ChromiumOS SDK build via `bb add`, and reports it to the user.
func (r *chromiumOSSDKRun) runBuilder(ctx context.Context) error {
	builderName := r.getBuilderFullName()
	_, err := r.bbClient.BBAdd(ctx, r.dryrun, append([]string{builderName}, r.bbAddArgs...)...)
	return err
}

// createBranchPolicy creates a struct representing a BranchPolicy message with the given CQ policy and reviewer.
// BranchPolicy, SendToCqPolicy, and Reviewer are all proto messages defined in infra/recipes/recipes/generator.proto.
func createBranchPolicy(cqPolicy, reviewerEmail string) map[string]any {
	// Values pulled from the SendToCqPolicy enum in generator.proto.
	var sendToCQPolicy = map[string]int{
		"do-nothing": 1,
		"dry-run":    2,
		"full-run":   3,
		"abandon":    4,
		"submit":     5,
	}[cqPolicy]
	reviewer := map[string]any{"email": reviewerEmail}
	return map[string]any{
		"pattern":                ".*",
		"reviewers":              []any{reviewer},
		"no_existing_cls_policy": sendToCQPolicy,
		"existing_cls_policy":    sendToCQPolicy,
	}
}
