// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package site

import (
	"context"
	"errors"
	"flag"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/logging"
)

type Subcommand struct {
	subcommands.CommandRunBase

	LogConfig   logging.Config
	CommonFlags CommonFlags
	AuthFlags   authcli.Flags
}

func (c *Subcommand) ModifyContext(ctx context.Context) context.Context {
	return c.LogConfig.Set(ctx)
}

func (c *Subcommand) Init() {
	c.LogConfig.Level = logging.Info
	c.LogConfig.AddFlags(&c.Flags)
	c.AuthFlags.Register(&c.Flags, DefaultAuthOptions)
	c.AuthFlags.RegisterIDTokenFlags(&c.Flags)
	c.CommonFlags.Register(&c.Flags)
}

func (c *Subcommand) Done(ctx context.Context, err error) int {
	if err == nil {
		return 0
	}
	logging.Errorf(ctx, "%s\n", err)
	return 1
}

// CommonFlags are the flags common to all commands.
type CommonFlags struct {
	// local forces the use of the local protocol rather than https,
	// which makes sense because you're talking to a local service.
	local   bool
	dev     bool
	address string
}

// Register the common flags.
func (fl *CommonFlags) Register(f *flag.FlagSet) {
	f.BoolVar(&fl.local, "local", false, "talk to the local project.")
	f.BoolVar(&fl.dev, "dev", false, "use the dev cloud run project")
	f.StringVar(&fl.address, "address", "", "a host:port address, \"\" by default")
}

// HTTP returns whether to use HTTP or HTTPS (default).
func (fl *CommonFlags) HTTP() bool {
	return fl.local
}

// Prod returns whether we are talking to prod or not.
func (fl *CommonFlags) Prod() bool {
	return !fl.local && !fl.dev
}

// Local returns whether we are talking to local
func (fl *CommonFlags) Local() bool {
	return fl.local
}

// Host returns the host to contact.
func (fl *CommonFlags) Host() (string, error) {
	switch {
	case fl.address != "":
		return fl.address, nil
	case fl.dev && !fl.local:
		return "fleet-console-dev-1037063051440.us-central1.run.app", nil
	case !fl.dev && fl.local:
		return "localhost:8800", nil
	case !fl.dev && !fl.local:
		return "fleet-console-prod-1012156191214.us-central1.run.app", nil
	default:
		return "", errors.New("-dev and -local are alternatives")
	}
}

// UIHost returns the UI host to contact for a given environment.
func (fl *CommonFlags) UIHost() (string, error) {
	switch {
	case fl.address != "":
		return fl.address, nil
	case fl.dev && !fl.local:
		panic("dev UI exists, but I haven't added it to this tool yet.")
	case !fl.dev && fl.local:
		panic("local UI also exists, but I haven't added it to this tool yet.")
	case !fl.dev && !fl.local:
		return "ci.chromium.org", nil
	default:
		return "", errors.New("-dev and -local are alternatives")
	}
}
