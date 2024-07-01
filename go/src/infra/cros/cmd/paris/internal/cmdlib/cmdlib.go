// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cmdlib contains common base tool.
package cmdlib

import (
	"context"
	"flag"

	"google.golang.org/grpc/metadata"

	lflag "go.chromium.org/luci/common/flag"
	"go.chromium.org/luci/common/logging"

	"infra/cros/cmd/paris/internal/site"
	rem "infra/libs/skylab/inventory/removalreason"
	ufsUtil "infra/unifiedfleet/app/util"
)

// DefaultTaskPriority is the default priority for a swarming task.
var DefaultTaskPriority = 140

// CommonFlags controls some commonly-used CLI flags.
type CommonFlags struct {
	verbose bool
}

// Register sets up the common flags.
func (f *CommonFlags) Register(fl *flag.FlagSet) {
	fl.BoolVar(&f.verbose, "verbose", false, "log more details")
}

// Verbose returns if the command is set to verbose mode.
func (f *CommonFlags) Verbose() bool {
	return f.verbose
}

// SetLogging is used to sets the level for the logger when needed
func SetLogging(ctx context.Context, level logging.Level) context.Context {
	return logging.SetLevel(ctx, level)
}

// EnvFlags controls selection of the environment: either prod (default) or dev.
type EnvFlags struct {
	dev bool
}

// Register sets up the -dev argument.
func (f *EnvFlags) Register(fl *flag.FlagSet) {
	fl.BoolVar(&f.dev, "dev", false, "Run in dev environment.")
}

// Env returns the environment, either dev or prod.
func (f EnvFlags) Env() site.Environment {
	if f.dev {
		return site.Dev
	}
	return site.Prod
}

// RegisterRemovalReason sets up the command line arguments for specifying a removal reason.
func RegisterRemovalReason(rr *rem.RemovalReason, f *flag.FlagSet) {
	f.StringVar(&rr.Bug, "bug", "", "Bug link for why DUT is being removed.  Required.")
	f.StringVar(&rr.Comment, "comment", "", "Short comment about why DUT is being removed.")
	f.Var(lflag.RelativeTime{T: &rr.Expire}, "expires-in", "Expire removal reason in `days`.")
}

// SetupContext sets up context with namespace
func SetupContext(ctx context.Context, namespace string) context.Context {
	md := metadata.Pairs(ufsUtil.Namespace, namespace)
	return metadata.NewOutgoingContext(ctx, md)
}
