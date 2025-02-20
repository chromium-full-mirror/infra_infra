// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cmd/crosfleet/internal/common"
	"go.chromium.org/infra/cmd/crosfleet/internal/site"
	"go.chromium.org/infra/libs/cipd"
)

// crosfleetDir is the CIPD parent directory for crosfleet packages.
const crosfleetParentDir = "chromiumos/infra/crosfleet/"
const crosfleetProdCIPDRef = "prod"

// Update subcommand: Update crosfleet tool.
var Update = &subcommands.Command{
	UsageLine: "update",
	ShortDesc: "update crosfleet tool",
	LongDesc: `Update crosfleet tool.

This is just a thin wrapper around CIPD.`,
	CommandRun: func() subcommands.CommandRun {
		c := &updateRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.printer.Register(&c.Flags)
		return c
	},
}

type updateRun struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
	printer   common.CLIPrinter
}

func (c *updateRun) Run(a subcommands.Application, _ []string, _ subcommands.Env) int {
	if err := c.innerRun(a); err != nil {
		c.printer.WriteTextStderr("%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

func (c *updateRun) innerRun(a subcommands.Application) error {
	d, err := executableDir()
	if err != nil {
		return err
	}
	root, err := findCIPDRootDir(d)
	if err != nil {
		return err
	}

	if err := cipdEnsureProd(a, root, &c.printer); err != nil {
		return err
	}
	c.printer.WriteTextStderr("%s: You may need to run crosfleet login again after the update", a.GetName())
	c.printer.WriteTextStderr("%s: Run crosfleet whoami to check login status", a.GetName())
	return nil
}

// executableDir returns the directory the current executable came
// from.
func executableDir() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", errors.Annotate(err, "get executable directory").Err()
	}
	return filepath.Dir(p), nil
}

func findCIPDRootDir(dir string) (string, error) {
	a, err := filepath.Abs(dir)
	if err != nil {
		return "", errors.Annotate(err, "find CIPD root dir").Err()
	}
	for d := a; d != "/"; d = filepath.Dir(d) {
		if isCIPDRootDir(d) {
			return d, nil
		}
	}
	return "", errors.Reason("find CIPD root dir: no CIPD root above %s", dir).Err()
}

func isCIPDRootDir(dir string) bool {
	fi, err := os.Stat(filepath.Join(dir, ".cipd"))
	if err != nil {
		return false
	}
	return fi.Mode().IsDir()
}

// currentEnsureFile captures the current state of all installed CIPD packages.
func ensureFileWithProdCrosfleet(dir string) (string, error) {
	packages, err := cipd.InstalledPackages("crosfleet")(dir)
	if err != nil {
		return "", err
	}
	var packageConfigs []string
	for _, p := range packages {
		if strings.HasPrefix(p.Package, crosfleetParentDir) {
			p.Pin.InstanceID = crosfleetProdCIPDRef
		}
		config := fmt.Sprintf("%s %s", p.Package, p.Pin.InstanceID)
		packageConfigs = append(packageConfigs, config)
	}
	return strings.Join(packageConfigs, "\n"), nil
}

// cipdEnsureProd takes an application and a directory and runs a command with
// arguments that will read a cipd manifest from stdin and then run "ensure".
//
// We dynamically generate an ensure file to ensure no other installed CIPD
// packages are removed during the crosfleet package update.
//
// Without this function, you need to run `sudo env PATH="$PATH" crosfleet update` in order to update
// crosfleet if crosfleet was installed as root.
//
// cipdEnsureProd assumes that the directory exists and that the [[dir]]/.cipd directory
// exists.
func cipdEnsureProd(a subcommands.Application, dir string, printer *common.CLIPrinter) error {
	ensureFile, err := ensureFileWithProdCrosfleet(dir)
	if err != nil {
		return err
	}
	// We create two runnable command objects that update the cipd directory.
	// One runs as the current user and the other always runs as root.
	// If the command that runs as the current user fails, then we try the second command.
	asSelf := exec.Command("cipd", "ensure", "-root", dir, "-ensure-file", "-")
	asSelf.Stdin = strings.NewReader(ensureFile)
	asSelf.Stdout = printer.GetOut()
	asSelf.Stderr = printer.GetErr()
	// Windows does not support sudo
	pathvar := fmt.Sprintf("PATH=%s", os.Getenv("PATH"))
	asRootUnix := exec.Command("sudo", "/usr/bin/env", pathvar, "cipd", "ensure", "-root", dir, "-ensure-file", "-")
	asRootUnix.Stdin = strings.NewReader(ensureFile)
	asRootUnix.Stdout = printer.GetOut()
	asRootUnix.Stderr = printer.GetErr()

	if err := asSelf.Run(); err == nil {
		return nil
	}

	// We unconditionally run `sudo` on all OS's, however, we expect it to fail on Windows.
	printer.WriteTextStderr("Retrying as root. Updating crosfleet through cipd.\n")
	if err := asRootUnix.Run(); err != nil {
		return fmt.Errorf("updating cipd as root: %w", err)
	}
	return nil
}
