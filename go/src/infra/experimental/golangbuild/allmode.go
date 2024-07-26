// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"

	"infra/experimental/golangbuild/golangbuildpb"
)

// allRunner gets Go (building it if no prebuilt toolchain is available for the current
// platform), then runs tests for the current project.
//
// This is the most basic form of serial execution and represents the simplest Go build.
type allRunner struct {
	props *golangbuildpb.AllMode
}

// newAllRunner creates a new AllMode runner.
func newAllRunner(props *golangbuildpb.AllMode) *allRunner {
	return &allRunner{props: props}
}

// Run implements the runner interface for allRunner.
func (r *allRunner) Run(ctx context.Context, spec *buildSpec, opts runOptions) error {
	// Get a built Go toolchain or build it if necessary.
	if err := getGo(ctx, spec, "", spec.goroot, spec.goSrc, false); err != nil {
		return err
	}
	// Determine what ports to test.
	ports := []*golangbuildpb.Port{spec.inputs.Target}
	if spec.inputs.MiscPorts {
		// Note: There may be code changes in cmd/dist or cmd/go that have not
		// been fully reviewed yet, and it is a test error if goDistList fails.
		var err error
		ports, err = goDistList(ctx, spec, noSharding)
		if err != nil {
			return err
		}
	}
	// Run tests. (Also fetch dependencies if applicable.)
	if isGoProject(spec.inputs.Project) {
		if opts.fetchOnly() {
			return nil
		}
		return runGoTests(ctx, spec, noSharding, ports)
	}
	repoDir, err := fetchSubrepo(ctx, spec)
	if err != nil {
		return err
	}
	if opts.fetchOnly() {
		return nil
	}
	return runSubrepoTests(ctx, spec, repoDir, ports)
}
