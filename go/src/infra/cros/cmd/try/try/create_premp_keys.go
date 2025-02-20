// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package try

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/maruel/subcommands"
	"google.golang.org/protobuf/encoding/protojson"

	bapipb "go.chromium.org/chromiumos/infra/proto/go/chromite/api"
	pb "go.chromium.org/chromiumos/infra/proto/go/chromiumos"
	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/internal/cmd"
	bb "go.chromium.org/infra/cros/lib/buildbucket"
)

func GetCmdCreatePreMPKeys(authOpts auth.Options) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "create_premp_keys --board BOARD [flags]",
		ShortDesc: "Create PreMP Keys for the given build target.",
		CommandRun: func() subcommands.CommandRun {
			c := &createPreMPKeysRun{}
			c.cmdRunner = cmd.RealCommandRunner{}
			c.tryRunBase.authOpts = authOpts
			c.addDryrunFlag()
			c.addProductionFlag()
			c.Flags.StringVar(&c.buildTarget, "build_target", "", "Build target to create keys for.")
			c.Flags.IntVar(&c.bug, "bug", 0, "bug ID to associate with this key creation, e.g. 318522770.")
			c.Flags.BoolVar(&c.addLoem, "add-loem", false, "whether to add loem to existing keyset.")
			return c
		},
	}
}

// createPreMPKeysRun tracks relevant info for a given `try create_premp_keys` run.
type createPreMPKeysRun struct {
	tryRunBase
	propsFile   *os.File
	buildTarget string
	bug         int
	addLoem     bool
}

// Run provides the logic for a `try create_premp_keys` command run.
func (f *createPreMPKeysRun) Run(_ subcommands.Application, _ []string, _ subcommands.Env) int {
	f.stdoutLog = log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)
	f.stderrLog = log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)

	ctx := context.Background()

	// Need to call run first to do LUCI auth / set up other shared constructs.
	if ret, err := f.run(ctx); err != nil {
		f.LogErr(err.Error())
		return ret
	}
	if err := f.validate(ctx); err != nil {
		f.LogErr(err.Error())
		return CmdError
	}

	keyManagerBuilderName := getKeyManagerBuilderFullName(!f.production)
	propsStruct, err := f.bbClient.GetBuilderInputProps(ctx, keyManagerBuilderName)
	if err != nil {
		f.LogErr(err.Error())
		return CmdError
	}

	// Set `create_premp_keys_requests` property.
	createPreMPKeysRequest := protojson.Format(&bapipb.CreatePreMPKeysRequest{
		BuildTarget: &pb.BuildTarget{
			Name: f.buildTarget,
		},
		AddLoem: f.addLoem,
	})
	var request interface{}
	if err := json.Unmarshal([]byte(createPreMPKeysRequest), &request); err != nil {
		f.LogErr(err.Error())
		return CmdError
	}
	if err := bb.SetProperty(propsStruct, "create_premp_keys_request", request); err != nil {
		f.LogErr(err.Error())
		return CmdError
	}
	if err := bb.SetProperty(propsStruct, "bug", f.bug); err != nil {
		f.LogErr(err.Error())
		return CmdError
	}

	// TODO(b/318522770) Support other fields of CreatePreMPKeysRequest.

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
		defer os.Remove(propsFile.Name())
	}
	f.bbAddArgs = append(f.bbAddArgs, "-p", fmt.Sprintf("@%s", propsFile.Name()))

	if err := f.runKeyManagerBuilder(ctx, keyManagerBuilderName); err != nil {
		f.LogErr(err.Error())
		return CmdError
	}
	return Success
}

// validate validates args for the command.
func (f *createPreMPKeysRun) validate(ctx context.Context) error {
	if f.buildTarget == "" {
		return errors.New("must provide a build target with --build_target")
	}
	if f.bug == 0 {
		return errors.New("--bug is required.")
	}
	if err := f.tryRunBase.validate(); err != nil {
		return err
	}
	return nil
}

// getKeyManagerBuilderFullName gets the full name of the builder.
func getKeyManagerBuilderFullName(staging bool) string {
	var bucket, stagingPrefix string
	if staging {
		bucket = "staging"
		stagingPrefix = "staging-"
	} else {
		bucket = "release"
	}
	return fmt.Sprintf("chromeos/%s/%skey-manager", bucket, stagingPrefix)
}

// runKeyManagerBuilder creates a key-manager build via `bb add`, and reports it to the user.
func (f *createPreMPKeysRun) runKeyManagerBuilder(ctx context.Context, keyManagerBuilderName string) error {
	_, err := f.bbClient.BBAdd(ctx, f.dryrun, append([]string{keyManagerBuilderName}, f.bbAddArgs...)...)
	return err
}
