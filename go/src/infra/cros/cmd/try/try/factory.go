// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package try

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/errors"

	"infra/cros/internal/cmd"
	bb "infra/cros/lib/buildbucket"
)

func GetCmdFactory(authOpts auth.Options) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "factory --branch BRANCH [flags]",
		ShortDesc: "Run a factory branch builder.",
		CommandRun: func() subcommands.CommandRun {
			c := &factoryRun{}
			c.cmdRunner = cmd.RealCommandRunner{}
			c.tryRunBase.authOpts = authOpts
			c.addDryrunFlag()
			c.addBranchFlag("")
			c.addProductionFlag()
			c.addPatchesFlag()
			c.addPublishFlag()
			return c
		},
	}
}

// factoryRun tracks relevant info for a given `try factory` run.
type factoryRun struct {
	tryRunBase
	propsFile *os.File
}

// Run provides the logic for a `try factory` command run.
func (f *factoryRun) Run(_ subcommands.Application, _ []string, _ subcommands.Env) int {
	f.stdoutLog = log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)
	f.stderrLog = log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)

	ctx := context.Background()

	// Do not create a gerritClient for test structs with a mockClient.
	if f.gerritClient == nil {
		if err := f.createGerritClient(f.authOpts); err != nil {
			f.LogErr(err.Error())
			return CmdError
		}
	}

	// Need to call run first to do LUCI auth / set up other shared constructs.
	if ret, err := f.run(ctx); err != nil {
		f.LogErr(err.Error())
		return ret
	}
	if err := f.validate(ctx); err != nil {
		f.LogErr(err.Error())
		return CmdError
	}

	propsStruct, err := f.bbClient.GetBuilderInputProps(ctx, getFactoryBuilderFullName(f.branch, !f.production))
	if err != nil {
		f.LogErr(err.Error())
		return CmdError
	}

	skipPublish := !f.publish
	if err := bb.SetProperty(propsStruct, "$chromeos/cros_artifacts.skip_publish", skipPublish); err != nil {
		f.LogErr(err.Error())
		return CmdError
	}

	if len(f.patches) > 0 {
		f.bbAddArgs = append(f.bbAddArgs, patchListToBBAddArgs(f.patches)...)
	}

	var propsFile *os.File
	if f.propsFile != nil {
		propsFile = f.propsFile
	} else {
		propsFile, err = os.CreateTemp("", "input_props")
		if err != nil {
			f.LogErr(err.Error())
			return CmdError
		}
	}
	if err := bb.WriteStructToFile(propsStruct, propsFile); err != nil {
		f.LogErr(errors.Annotate(err, "writing input properties to tempfile").Err().Error())
		return UnspecifiedError
	}
	if f.propsFile == nil {
		defer propsFile.Close()
		defer os.Remove(propsFile.Name())
	}
	f.bbAddArgs = append(f.bbAddArgs, "-p", fmt.Sprintf("@%s", propsFile.Name()))

	if err := f.runFactoryBuilder(ctx); err != nil {
		f.LogErr(err.Error())
		return CmdError
	}
	return Success
}

// validate validates factory-specific args for the command.
func (f *factoryRun) validate(ctx context.Context) error {
	if f.branch == "" {
		return errors.New("must provide a factory branch with --branch")
	}
	if !strings.HasPrefix(f.branch, "factory-") || !strings.HasSuffix(f.branch, ".B") {
		return fmt.Errorf("provided branch does not look like a factory branch: %s", f.branch)
	}
	if builderExists, err := f.doesFactoryBranchHaveBuilder(ctx, f.branch, !f.production); err != nil {
		return err
	} else if !builderExists {
		return fmt.Errorf("factory builder does not seem to exist for branch %s", f.branch)
	}
	if err := f.tryRunBase.validate(); err != nil {
		return err
	}
	return nil
}

// doesFactoryBranchHaveBuilder checks whether the given branch has a factory builder configured.
func (f *factoryRun) doesFactoryBranchHaveBuilder(ctx context.Context, branch string, staging bool) (bool, error) {
	bucket := "factory"
	if staging {
		bucket = "staging"
	}
	allFactoryBuilders, err := f.bbClient.BBBuilders(ctx, bucket)
	if err != nil {
		return false, errors.Annotate(err, "querying bb for factory builders").Err()
	}
	return sliceContainsStr(allFactoryBuilders, getFactoryBuilderFullName(branch, staging)), nil
}

// getFactoryBuilderFullName finds the full name (<project>/<bucket>/<builder>) for the given factory branch.
func getFactoryBuilderFullName(branch string, staging bool) string {
	var bucket, stagingPrefix string
	if staging {
		bucket = "staging"
		stagingPrefix = "staging-"
	} else {
		bucket = "factory"
	}
	return fmt.Sprintf("chromeos/%s/%s%s-orchestrator", bucket, stagingPrefix, branch)
}

// runFactoryBuilder creates a factory build via `bb add`, and reports it to the user.
func (f *factoryRun) runFactoryBuilder(ctx context.Context) error {
	builderName := getFactoryBuilderFullName(f.branch, !f.production)
	_, err := f.bbClient.BBAdd(ctx, f.dryrun, append([]string{builderName}, f.bbAddArgs...)...)
	return err
}
