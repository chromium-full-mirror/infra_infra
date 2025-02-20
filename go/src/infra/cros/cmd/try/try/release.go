// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package try

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/maruel/subcommands"

	"go.chromium.org/chromiumos/infra/proto/go/chromiumos"
	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/internal/cmd"
	bb "go.chromium.org/infra/cros/lib/buildbucket"
)

func GetCmdRelease(authOpts auth.Options) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "release [flags]",
		ShortDesc: "Run a release builder.",
		CommandRun: func() subcommands.CommandRun {
			c := &releaseRun{}
			c.tryRunBase.cmdRunner = cmd.RealCommandRunner{}
			c.tryRunBase.authOpts = authOpts
			c.addDryrunFlag()
			c.addBranchFlag("main")
			c.addProductionFlag()
			c.addPatchesFlag()
			c.addBuildTargetsFlag()
			c.addVerboseFlag()
			c.Flags.BoolVar(&c.dev, "dev", false,
				"run a staging builder with the true ToT environment (recipes, bot image, etc.),"+
					"instead of a prod-like env.")
			c.Flags.BoolVar(&c.useProdTests, "prod_tests", false, "Run (production) HW tests even if in staging. "+
				"By default, HW tests are disabled in staging.")
			c.Flags.BoolVar(&c.skipPaygen, "skip_paygen", false, "Skip payload generation. Only supported for staging builds.")
			c.Flags.StringVar(&c.channelOverride, "channels", "", "Specify comma-separated channel(s) to sign on. E.g. --channels beta,dev,lts")
			if flag.NArg() > 1 && flag.Args()[1] == "help" {
				fmt.Printf("Run `cros try help` or `cros try help ${subcomand}` for help.")
				os.Exit(0)
			}
			return c
		},
	}
}

// releaseRun tracks relevant info for a given `try release` run.
type releaseRun struct {
	tryRunBase
	dev             bool
	useProdTests    bool
	skipPaygen      bool
	channelOverride string
	// Used for testing purposes. If set, props will be written to this file
	// rather than a temporary one.
	propsFile *os.File
}

// validate validates release-specific args for the command.
func (r *releaseRun) validate() error {
	if r.production {
		if r.dev {
			return errors.New("--dev and --production cannot be used together")
		}
		if r.skipPaygen {
			return errors.New("--skip_paygen is not supported for production builds")
		}
	}

	if err := r.tryRunBase.validate(); err != nil {
		return err
	}
	return nil
}

// checkChildrenExist checks that any explicitly requested build targets
// have a builder for the relevant branch.
func (r *releaseRun) checkChildrenExist(ctx context.Context) error {
	if len(r.buildTargets) > 0 {
		builderNames := r.getReleaseBuilderNames()
		bucket := "try-preprod"
		if r.dev {
			bucket = "try-dev"
		} else if r.production {
			bucket = "release"
		}
		for i, builderName := range builderNames {
			fullBuilderName := fmt.Sprintf("chromeos/%s/%s", bucket, builderName)
			_, err := r.bbClient.GetBuilderInputProps(ctx, fullBuilderName)
			if err != nil && strings.Contains(err.Error(), "not found") {
				return fmt.Errorf(
					"%s is not a valid build target for %s. (If you just "+
						"created the branch, you may need to wait 10-15 min "+
						"for LUCI to pick up the new builders.)",
					r.buildTargets[i], r.branch)
			}
		}
	}
	return nil
}

// Run provides the logic for a `try release` command run.
func (r *releaseRun) innerRun(_ subcommands.Application, _ []string, _ subcommands.Env) int {
	if err := r.validate(); err != nil {
		r.LogErr(err.Error())
		return CmdError
	}

	if r.production && !r.skipProductionPrompt {
		if yes, err := r.promptYes(); err != nil {
			r.LogErr(err.Error())
			return CmdError
		} else if !yes {
			r.LogOut("Exiting.")
			return Success
		}
	}

	ctx := context.Background()
	if ret, err := r.run(ctx); err != nil {
		r.LogErr(err.Error())
		return ret
	}

	if err := r.checkChildrenExist(ctx); err != nil {
		r.LogErr(err.Error())
		return CmdError
	}

	propsStruct, err := r.bbClient.GetBuilderInputProps(ctx, r.getReleaseOrchestratorName())
	if err != nil {
		r.LogErr(err.Error())
		if strings.Contains(err.Error(), "not found") {
			if strings.HasPrefix(r.branch, "stabilize-") {
				r.LogErr(fmt.Sprintf("Builder not found, is '%s' defined in stabilize_builders.textpb?", r.branch))
			}
		}
		return CmdError
	}

	// TODO(b/266850767): Remove in 2024.
	// crrev.com/c/4205799 updated `cros try` to track a CIPD ref instead of a
	// speific CIPD version, allowing us to push updates to users. We want to
	// invalidate try builds that (roughly) predated this change.
	// This can be removed after it has baked for a sufficiently long period of
	// time (several quarters).
	if err := bb.SetProperty(propsStruct, "$chromeos/cros_try.supported_build", true); err != nil {
		r.LogErr(err.Error())
		return CmdError
	}

	if err := bb.SetProperty(propsStruct, "build_failures_fatal", true); err != nil {
		r.LogErr(err.Error())
		return CmdError
	}

	if len(r.patches) > 0 {
		// If gerrit patches are set, the orchestrator by default will try to do
		// build planning, which is meaningless for release builds and drops
		// all children. This property skips pruning.
		if err := bb.SetProperty(propsStruct, "$chromeos/build_plan.disable_build_plan_pruning", true); err != nil {
			r.LogErr(err.Error())
			return CmdError
		}
		r.bbAddArgs = append(r.bbAddArgs, patchListToBBAddArgs(r.patches)...)
	}

	if r.useProdTests {
		if err := bb.SetProperty(propsStruct, "$chromeos/cros_test_plan.use_prod_config", true); err != nil {
			r.LogErr(err.Error())
			return CmdError
		}
	}

	if err := bb.SetProperty(propsStruct, "$chromeos/orch_menu.schedule_public_build", false); err != nil {
		r.LogErr(err.Error())
		return CmdError
	}

	if r.skipPaygen {
		r.LogOutIfVerbose("Setting skip paygen.")
		if err := bb.SetProperty(propsStruct, "$chromeos/orch_menu.skip_paygen", true); err != nil {
			r.LogErr(err.Error())
			return CmdError
		}
	}

	if r.channelOverride != "" {
		r.LogOutIfVerbose("Setting channel override: %v", r.channelOverride)
		channelList, err := parseChannelOverride(r.channelOverride)
		if err != nil {
			r.LogErr(err.Error())
			return CmdError
		}
		if err := bb.SetProperty(propsStruct, "$chromeos/cros_infra_config.override_release_channels", channelList); err != nil {
			r.LogErr(err.Error())
			return CmdError
		}
		if err := bb.SetProperty(propsStruct, "$chromeos/cros_infra_config.should_override_release_channels", true); err != nil {
			r.LogErr(err.Error())
			return CmdError
		}
	}

	if len(r.buildTargets) > 0 {
		r.LogOutIfVerbose("Setting build targets: %v", r.buildTargets)
		if err := bb.SetProperty(propsStruct, "$chromeos/orch_menu.child_builds", r.getReleaseBuilderNames()); err != nil {
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
	r.LogOutIfVerbose("Running with the following props: %v", propsStruct)
	if err := bb.WriteStructToFile(propsStruct, propsFile); err != nil {
		r.LogErr(errors.Annotate(err, "writing input properties to tempfile").Err().Error())
		return UnspecifiedError
	}
	if r.propsFile == nil {
		defer os.Remove(propsFile.Name())
	}
	r.bbAddArgs = append(r.bbAddArgs, "-p", fmt.Sprintf("@%s", propsFile.Name()))

	if err := r.runReleaseOrchestrator(ctx); err != nil {
		r.LogErr(err.Error())
		return CmdError
	}

	return Success
}

func (r *releaseRun) Run(_ subcommands.Application, _ []string, _ subcommands.Env) int {
	r.stdoutLog = log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)
	r.stderrLog = log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)

	// Do not create a gerritClient for test structs with a mockClient.
	if r.gerritClient == nil {
		if err := r.createGerritClient(r.authOpts); err != nil {
			r.LogErr(err.Error())
			return CmdError
		}
	}

	return r.innerRun(nil, nil, nil)
}

// getReleaseOrchestratorName finds the full name of the release orchestrator matching the try CLI flags.
func (r *releaseRun) getReleaseOrchestratorName() string {
	const project = "chromeos"
	var bucket, builder, stagingPrefix string
	if r.production {
		bucket = "release"
	} else {
		if r.dev {
			bucket = "try-dev"
		} else {
			bucket = "try-preprod"
		}
		stagingPrefix = "staging-"
	}
	if strings.HasPrefix(r.branch, "release-") {
		builder = fmt.Sprintf("%s%s-orchestrator", stagingPrefix, r.branch)
	} else {
		builder = fmt.Sprintf("%srelease-%s-orchestrator", stagingPrefix, r.branch)
	}
	return fmt.Sprintf("%s/%s/%s", project, bucket, builder)
}

func (r *releaseRun) getReleaseBuilderNames() []string {
	const project = "chromeos"
	var builder, stagingPrefix string
	if !r.production {
		stagingPrefix = "staging-"
	}
	builderNames := make([]string, len(r.buildTargets))
	for i, buildTarget := range r.buildTargets {
		if strings.HasPrefix(r.branch, "release-") {
			builder = fmt.Sprintf("%s%s-%s", stagingPrefix, buildTarget, r.branch)
		} else {
			builder = fmt.Sprintf("%s%s-release-%s", stagingPrefix, buildTarget, r.branch)
		}
		builderNames[i] = builder
	}

	return builderNames
}

// runReleaseOrchestrator creates a release orchestrator build via `bb add`, and reports it to the user.
func (r *releaseRun) runReleaseOrchestrator(ctx context.Context) error {
	orchName := r.getReleaseOrchestratorName()
	_, err := r.bbClient.BBAdd(ctx, r.dryrun, append([]string{orchName}, r.bbAddArgs...)...)
	return err
}

// parseChannelOverride parses the channel override string and returns a list of channels by number.
// The channel numbers come from chromiumos/common.proto.
// For example, "2,stable,dev" -> [2,1,3] and "invalid_channel,2,4" -> error.
func parseChannelOverride(channelOverride string) ([]string, error) {
	var errs []error
	channels := strings.Split(channelOverride, ",")
	out := make([]string, 0, len(channels))
	for _, channel := range channels {
		channel = strings.TrimSpace(channel)
		parsedChannelNumber, err := strconv.Atoi(channel)
		if err == nil { // if NO error, i.e. we parsed an integer
			if _, ok := chromiumos.Channel_name[int32(parsedChannelNumber)]; !ok {
				errs = append(errs, fmt.Errorf("invalid channel number %d", parsedChannelNumber))
				continue
			}
			out = append(out, strconv.Itoa(parsedChannelNumber))
		} else { // user supplied a non-integer channel name
			enumName := "CHANNEL_" + strings.ToUpper(channel)
			val, ok := chromiumos.Channel_value[enumName]
			if !ok {
				errs = append(errs, fmt.Errorf("invalid channel name %q", channel))
				continue
			}
			out = append(out, strconv.Itoa(int(val)))
		}
	}

	return out, errors.Join(errs...)
}
