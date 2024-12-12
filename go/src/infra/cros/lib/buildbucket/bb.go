// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package buildbucket

import (
	"context"
	gerr "errors"
	"fmt"
	"log"
	"os/exec"
	"path"
	"regexp"
	"strings"

	"go.chromium.org/luci/common/errors"

	"infra/cros/internal/cmd"
	"infra/cros/internal/util"
)

type Client struct {
	cmdRunner cmd.CommandRunner
	stdoutLog *log.Logger
	stderrLog *log.Logger
}

// NewClient creates a new Buildbucket client.
func NewClient(cmdRunner cmd.CommandRunner, stdoutLog *log.Logger, stderrLog *log.Logger) *Client {
	return &Client{
		cmdRunner: cmdRunner,
		stdoutLog: stdoutLog,
		stderrLog: stderrLog,
	}
}

// LogOut logs to stdout.
func (c *Client) LogOut(format string, a ...interface{}) {
	if c.stdoutLog != nil {
		c.stdoutLog.Printf(format, a...)
	}
}

// LogErr logs to stderr.
func (c *Client) LogErr(format string, a ...interface{}) {
	if c.stderrLog != nil {
		c.stderrLog.Printf(format, a...)
	}
}

// FakeAuthInfoRunner creates a FakeCommandRunner for `{tool} auth-info` (like bb or led).
func FakeAuthInfoRunner(tool string, exitCode int) cmd.FakeCommandRunner {
	return cmd.FakeCommandRunner{
		ExpectedCmd: []string{tool, "auth-info"},
		FailCommand: exitCode != 0,
		FailError:   createCmdFailError(exitCode),
	}
}

// FakeWhichRunner creates a FakeCommandRunner for `which {tool}` (like bb or led).
func FakeWhichRunner(tool string, exitCode int) cmd.FakeCommandRunner {
	return cmd.FakeCommandRunner{
		ExpectedCmd: []string{"which", tool},
		FailCommand: exitCode != 0,
		FailError:   createCmdFailError(exitCode),
		Stdout:      fmt.Sprintln(path.Join("path", "to", tool)),
	}
}

// FakeAuthInfoRunnerSuccessStdout is like FakeAuthInfoRunner with exitCode=0, plus stdout about the logged-in user.
// user should normally be an email address, such as "sundar@google.com".
// For now it doesn't mock OAuth token details.
func FakeAuthInfoRunnerSuccessStdout(tool string, user string) cmd.FakeCommandRunner {
	return cmd.FakeCommandRunner{
		ExpectedCmd: []string{tool, "auth-info"},
		Stdout:      fmt.Sprintf("Logged in as %s.\n\nOAuth token details:\n...", user),
	}
}

// createCmdFailError creates an error with the desired exit status.
func createCmdFailError(exitCode int) error {
	return exec.Command("bash", "-c", fmt.Sprintf("exit %d", exitCode)).Run()
}

// IsLUCIToolAuthed checks whether the named LUCI CLI tool is logged in.
func (c *Client) IsLUCIToolAuthed(ctx context.Context, tool string) (bool, error) {
	_, stderr, err := c.runCmd(ctx, tool, "auth-info")
	if err == nil {
		// If the tool is authed, then `auth-info` returns exit status 0, and Run() should give no error.
		return true, nil
	} else if exiterr, ok := errors.Unwrap(err).(*exec.ExitError); ok && exiterr.ExitCode() == 1 {
		// If the tool is not authed, then `auth-info` returns exit status 1.
		return false, nil
	}
	// Other problems return other exit statuses:
	// `bb fake-subcommand` and `led fake-subcommand` return exit status 2;
	// if the tool is altogether not found, then it'll return 127.
	fmt.Println(stderr)
	return false, err
}

// EnsureLUCIToolsAuthed ensures that multiple LUCI CLI tools are logged in.
// If any tools are not authed, it will return an error instructing the user to log into each unauthed tool.
func (c *Client) EnsureLUCIToolsAuthed(ctx context.Context, tools ...string) error {
	var unauthedTools []string
	var authCommands []string
	for _, tool := range tools {
		toolPath, err := c.ToolPath(ctx, tool)
		if err != nil {
			return err
		}
		if authed, err := c.IsLUCIToolAuthed(ctx, tool); err != nil {
			return err
		} else if !authed {
			unauthedTools = append(unauthedTools, tool)
			authCommands = append(authCommands, fmt.Sprintf("%s auth-login", toolPath))
		}
	}
	if len(unauthedTools) != 0 {
		return fmt.Errorf(
			"The following tools were not logged in: %s. Please run the following commands, then try again:\n\t%s",
			strings.Join(unauthedTools, ", "),
			strings.Join(authCommands, "\n\t"))
	}
	return nil
}

// ToolPath uses `which` to return the path to the given tool.
func (c *Client) ToolPath(ctx context.Context, tool string) (string, error) {
	stdout, stderr, err := c.runCmd(ctx, "which", tool)
	fmt.Println(stderr)
	if err != nil {
		return "", fmt.Errorf("failed to look up tool paths")
	}
	return strings.TrimSpace(stdout), nil
}

// runBBCmd runs a `bb` subcommand.
func (c *Client) runBBCmd(ctx context.Context, dryRun bool, subcommand string, args ...string) (stdout, stderr string, err error) {
	if dryRun {
		c.LogOut("would have run `bb %s %s`", subcommand, strings.Join(args, " "))
		return "", "", nil
	}
	return c.runCmd(ctx, "bb", util.PrependString(subcommand, args)...)
}

// BBAdd runs a `bb add` command, and prints stdout to the user. Returns the
// bbid of the build and an error (if any).
func (c *Client) BBAdd(ctx context.Context, dryRun bool, args ...string) (string, error) {
	stdout, stderr, err := c.runBBCmd(ctx, dryRun, "add", args...)
	if err != nil {
		c.LogErr(stderr)
		return "", err
	}
	if dryRun {
		return "dry_run_bbid", nil
	}
	c.LogOut("\n" + strings.Split(stdout, "\n")[0])
	bbidRegexp := regexp.MustCompile(`http:\/\/ci.chromium.org\/b\/(?P<bbid>\d+) `)
	matches := bbidRegexp.FindStringSubmatch(stdout)
	if matches == nil {
		return "", gerr.New("could not parse BBID from `bb add` stdout.")
	}
	return matches[1], nil
}

// FakeBBAddRunner creates a FakeCommandRunner for `bb add`.
// args should be a list of command args, including ["bb", "add"].
func FakeBBAddRunner(args []string, bbid string) cmd.FakeCommandRunner {
	stdout := fmt.Sprintf("http://ci.chromium.org/b/%s SCHEDULED ...\n", bbid)
	return cmd.FakeCommandRunner{
		ExpectedCmd: args,
		Stdout:      stdout,
	}
}

// getBuilders runs the `bb builders` command to get all builders in the given bucket.
// The bucket param should not include the project prefix (normally "chromeos/").
func (c *Client) BBBuilders(ctx context.Context, bucket string) ([]string, error) {
	stdout, stderr, err := c.runBBCmd(ctx, false, "builders", fmt.Sprintf("chromeos/%s", bucket))
	if err != nil {
		c.LogErr(stderr)
		return []string{}, err
	}
	return strings.Split(stdout, "\n"), nil
}
