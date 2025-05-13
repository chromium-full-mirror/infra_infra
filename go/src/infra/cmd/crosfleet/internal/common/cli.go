// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/maruel/subcommands"
	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/gcloud/googleoauth"

	"go.chromium.org/infra/cmd/crosfleet/internal/site"
	"go.chromium.org/infra/cmdsupport/cmdlib"
)

// Tag to add to Buildbucket builds to indicate which crosfleet subcommand was
// used to launch the build.
const CrosfleetToolTag = "crosfleet-tool"

// WriteCrosfleetUIPromptStderr writes a prompt to Stderr for users to visit
// the go/my-crosfleet PLX dashboard to track their crosfleet-launched tasks,
// as long as the CLI args do NOT include the -json flag. If -json WAS
// included, the function does nothing.
//
// NOTE: Parsing the -json flag here must happen separately from the normal
// CLIPrinter functions, since unlike those functions, this is run at the level
// of the outer "run"/"dut" commands, which don't have native access to the
// -json flag passed to their subcommands (e.g. "run test" or "dut lease").
func WriteCrosfleetUIPromptStderr(args []string) {
	for _, arg := range args {
		if arg == "-json" {
			return
		}
	}
	fmt.Fprintf(os.Stderr, "Visit http://go/my-crosfleet to track all of your crosfleet-launched tasks\n")
}

// CLIPrinter handles all command line output.
type CLIPrinter struct {
	json   bool
	silent bool
}

// Register parses the -json flag.
func (p *CLIPrinter) Register(fl *flag.FlagSet) {
	fl.BoolVar(&p.json, "json", false, "Format output as JSON.")
	fl.BoolVar(&p.silent, "silent", false, "Don't print any output from command.")
}

// WriteTextStdout writes the given human-readable output string (followed by
// a line break) to Stdout, as long as the CLI command was NOT passed the -json
// flag. If -json WAS passed, the function does nothing.
func (p *CLIPrinter) WriteTextStdout(output string, outputArgs ...any) {
	if p.json || p.silent {
		return
	}
	fmt.Fprintf(p.GetOut(), output+"\n", outputArgs...)
}

// WriteTextStderr writes the given human-readable output string (followed by
// a line break) to Stderr, as long as the CLI command was NOT passed the -json
// flag. If -json WAS passed, the function does nothing.
func (p *CLIPrinter) WriteTextStderr(output string, outputArgs ...any) {
	if p.json || p.silent {
		return
	}
	fmt.Fprintf(p.GetErr(), output+"\n", outputArgs...)
}

// WriteJSONStdout writes the given proto message as JSON (followed by a line
// break) to Stdout, as long as the CLI command WAS passed the -json flag. If
// -json was NOT passed, the function does nothing.
func (p *CLIPrinter) WriteJSONStdout(output proto.Message) {
	if p.silent {
		return
	}
	if p.json {
		fmt.Fprintf(p.GetOut(), "%s\n", protoJSON(output))
	}
}

// GetErr exposes the Stderr writer used by the CLI printer, returning nil if
// the -silent flag was passed.
func (p *CLIPrinter) GetErr() io.Writer {
	if p.silent {
		return nil
	}
	return os.Stderr
}

// GetOut exposes the Stdout writer used by the CLI printer, returning nil if
// the -silent flag was passed.
func (p *CLIPrinter) GetOut() io.Writer {
	if p.silent {
		return nil
	}
	return os.Stdout
}

// protoJSON returns the given proto message as pretty-printed JSON.
func protoJSON(message proto.Message) []byte {
	marshalOpts := protojson.MarshalOptions{
		EmitUnpopulated: false,
		Indent:          "\t",
	}
	json, err := marshalOpts.Marshal(proto.MessageV2(message))
	if err != nil {
		panic("Failed to marshal JSON")
	}
	return json
}

// EnvFlags controls selection of the environment: either prod (default) or dev.
type EnvFlags struct {
	dev, staging bool
}

// Register parses the -dev flag.
func (f *EnvFlags) Register(fl *flag.FlagSet) {
	fl.BoolVar(&f.dev, "dev", false, "Run in dev environment (mutually exclusive with -staging).")
	fl.BoolVar(&f.staging, "staging", false, "Run in staging environment (mutually exclusive with -dev).")
}

// Env returns the environment, either dev or prod.
func (f *EnvFlags) Env() site.Environment {
	if f.dev {
		return site.Dev
	} else if f.staging {
		return site.Staging
	}
	return site.Prod
}

// UseDev returns a bool indicating whether to use downstream dev environments.
func (f *EnvFlags) UseDev() bool {
	return f.dev
}

// ToKeyvalSlice converts a key-val map to a slice of "key:val" strings.
func ToKeyvalSlice(keyvals map[string]string) []string {
	var s []string
	for key, val := range keyvals {
		s = append(s, fmt.Sprintf("%s:%s", key, val))
	}
	return s
}

// GetUserEmail parses the given auth flags and returns the email of the
// authenticated crosfleet user.
func GetUserEmail(ctx context.Context, flags *authcli.Flags) (string, error) {
	opts, err := flags.Options()
	if err != nil {
		return "", nil
	}
	authenticator := auth.NewAuthenticator(ctx, auth.SilentLogin, opts)
	tempToken, err := authenticator.GetAccessToken(time.Minute)
	if err != nil {
		return "", err
	}
	authInfo, err := googleoauth.GetTokenInfo(ctx, googleoauth.TokenInfoParams{
		AccessToken: tempToken.AccessToken,
	})
	if err != nil {
		return "", err
	}
	if authInfo.Email == "" {
		return "", fmt.Errorf("no email found for the current user")
	}
	return authInfo.Email, nil
}

// PrintCmdError wraps the standard cmdlib.PrintError() function with a prompt
// to run `crosfleet login` if the error was LUCI auth-related.
func PrintCmdError(a subcommands.Application, err error) {
	if errors.Is(err, auth.ErrLoginRequired) {
		err = fmt.Errorf("%s: %s", err.Error(), "run `crosfleet login`")
	}
	cmdlib.PrintError(a, err)
}
