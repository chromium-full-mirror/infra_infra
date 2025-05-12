// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package try

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/internal/cmd"
	"go.chromium.org/infra/cros/internal/gerrit"
	"go.chromium.org/infra/cros/lib/buildbucket"
)

const PatchRegexpPattern = `^crrev\.com\/([ci])\/(\d+)$`

type list []string

func (l *list) Set(value string) error {
	*l = strings.Split(strings.TrimSpace(value), ",")
	return nil
}

func (l *list) String() string {
	return strings.Join(*l, ",")
}

// tryRunBase contains data for a single `try` command run.
type tryRunBase struct {
	subcommands.CommandRunBase
	stdoutLog    *log.Logger
	stderrLog    *log.Logger
	authOpts     auth.Options
	bbAddArgs    []string
	cmdRunner    cmd.CommandRunner
	bbClient     *buildbucket.Client
	gerritClient gerrit.Client
	// Used for testing.
	skipProductionPrompt bool

	dryrun     bool
	verbose    bool
	branch     string
	production bool
	publish    bool
	// Patches of the form of "crrev.com/c/1234567", "crrev.com/i/1234567".
	patches      list
	buildTargets list
}

// addBranchFlag creates a `-branch` command-line flag to specify the branch.
func (t *tryRunBase) addBranchFlag(defaultValue string) {
	t.Flags.StringVar(&t.branch, "branch", defaultValue, "Specify the branch on which to run the builder.")
}

// addProductionFlag creates a `-production` command-line flag for a try subcommand.
func (t *tryRunBase) addProductionFlag() {
	t.Flags.BoolVar(&t.production, "production", false, "Run a production builder instead of a staging builder.")
}

// addPatchesFlag creates a `-gerrit-patches` command-line flag for a try subcommand.
func (t *tryRunBase) addPatchesFlag() {
	t.Flags.Var(&t.patches, "gerrit-patches", "(comma-separated) patches to apply to the build, e.g. crrev.com/c/1234567,crrev.com/i/1234567.")
	t.Flags.Var(&t.patches, "g", "alias for --gerrit-patches")
}

// addBuildTargetsFlag creates a `-build_targets` command-line flag for a try subcommand.
func (t *tryRunBase) addBuildTargetsFlag() {
	t.Flags.Var(&t.buildTargets, "build_targets", "(comma-separated) Build targets to run. If not set, the standard set of build targets will be used.")
}

// addDryrunFlag creates a `-dryrun` command-line flag for a try command.
func (t *tryRunBase) addDryrunFlag() {
	t.Flags.BoolVar(&t.dryrun, "dryrun", false, "Dry run (i.e. don't actually run `bb add`).")
}

// addPublishFlag creates a `-publish-artifacts` command-line flag to specify that artifacts should be published.
func (t *tryRunBase) addPublishFlag() {
	t.Flags.BoolVar(&t.publish, "publish-artifacts", false, "Publish artifacts to canonical location in addition to uploading to GS.")
}

// addVerboseFlag creates a `-verbose` command-line flag to specify the level of logging.
func (t *tryRunBase) addVerboseFlag() {
	t.Flags.BoolVar(&t.verbose, "verbose", false, "Log additional information.")
}

// validate validates base args for the command.
func (t *tryRunBase) validate() error {
	if len(t.patches) > 0 {
		patchSpec := regexp.MustCompile(PatchRegexpPattern)
		for _, patch := range t.patches {
			if !patchSpec.MatchString(patch) {
				return fmt.Errorf(`invalid patch "%s". patches must be of the format crrev.com/[ci]/<number>.`, patch)
			}
		}

		if t.production {
			return fmt.Errorf("-g/--gerrit-patches is only supported for staging builds")
		}
	}

	return nil
}

// run executes common run logic for all tryRunBase commands.
func (t *tryRunBase) run(ctx context.Context) (int, error) {
	t.bbClient = buildbucket.NewClient(t.cmdRunner, t.stdoutLog, t.stderrLog)

	if err := t.bbClient.EnsureLUCIToolsAuthed(ctx, "bb", "led"); err != nil {
		return AuthError, err
	}
	if err := t.tagBuilds(ctx); err != nil {
		return CmdError, err
	}
	if t.patches != nil && len(t.patches) > 0 {
		// Include ancestors of patches.
		if patchesWithAncestors, err := includeAllAncestors(ctx, t.gerritClient, t.patches); err != nil {
			return CmdError, err
		} else {
			t.patches = patchesWithAncestors
		}
	}

	return Success, nil
}

// createGerritClient creates an authenticated gerrit client.
func (t *tryRunBase) createGerritClient(authOpts auth.Options) error {
	ctx := context.Background()
	authedClient, err := auth.NewAuthenticator(ctx, auth.SilentLogin, authOpts).Client()
	if err != nil {
		return errors.Annotate(err, "Please run `%s auth-login` and sign in with your @google.com account", os.Args[0]).Err()
	}
	if t.gerritClient, err = gerrit.NewClient(authedClient); err != nil {
		return err
	}

	return nil
}

// promptYes prompts the user yes or no and returns the response as a boolean.
func (t *tryRunBase) promptYes() (bool, error) {
	t.LogOut("You are launching a production build. Please confirm (y/N):")
	b := bufio.NewReader(os.Stdin)
	i, err := b.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("error getting prompt response: %w", err)
	}
	switch strings.TrimSpace(strings.ToLower(i)) {
	case "y", "yes":
		return true, nil
	case "", "n", "no":
		return false, nil
	default:
		return false, nil
	}
}

// tagBuilds adds the invoker's username as a tag to builds.
func (t *tryRunBase) tagBuilds(ctx context.Context) error {
	email, err := t.getUserEmail(ctx)
	if err != nil {
		return err
	}
	t.bbAddArgs = append(t.bbAddArgs, "-t", fmt.Sprintf("tryjob-launcher:%s", email))
	return nil
}

func (t *tryRunBase) getUserEmail(ctx context.Context) (string, error) {
	stdout, _, err := t.RunCmd(ctx, "led", "auth-info")
	if err != nil {
		return "", err
	}
	return parseEmailFromAuthInfo(stdout)
}

// LogOut logs to stdout.
func (t *tryRunBase) LogOut(format string, a ...any) {
	if t.stdoutLog != nil {
		t.stdoutLog.Printf(format, a...)
	}
}

// LogOutIfVerbose logs to stdout if `-verbose` is set.
func (t *tryRunBase) LogOutIfVerbose(format string, a ...any) {
	if !t.verbose {
		return
	}
	if t.stdoutLog != nil {
		t.stdoutLog.Printf(format, a...)
	}
}

// LogErr logs to stderr.
func (t *tryRunBase) LogErr(format string, a ...any) {
	if t.stderrLog != nil {
		t.stderrLog.Printf(format, a...)
	}
}

// RunCmd executes a shell command.
func (t tryRunBase) RunCmd(ctx context.Context, name string, args ...string) (stdout, stderr string, err error) {
	var stdoutBuf, stderrBuf bytes.Buffer
	err = t.cmdRunner.RunCommand(ctx, &stdoutBuf, &stderrBuf, "", name, args...)
	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()
	if err != nil {
		return stdout, stderr, errors.Annotate(err, fmt.Sprintf("running `%s %s`", name, strings.Join(args, " "))).Err()
	}
	return stdout, stderr, nil
}
