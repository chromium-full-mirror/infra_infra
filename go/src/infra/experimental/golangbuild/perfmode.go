// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	perfstorage "golang.org/x/perf/storage"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/luciexe/build"

	"infra/experimental/golangbuild/golangbuildpb"
)

// perfRunner runs performance tests and optionally uploads their results to perfdata.golang.org.
//
// For the main Go repository, it acquires a baseline toolchain and an experiment toolchain
// (the former is configured in golangbuildpb.PerfMode while the latter comes from the build's
// input sources), then runs cmd/bench in the x/benchmarks repository.
//
// For golang.org/x repos, it acquires a toolchain for the latest commit on the Go branch specified
// in the input properties, checks out the repository at the baseline commit and the commit
// specified in the input sources, and finally runs benchmarks via `go test`.
type perfRunner struct {
	props *golangbuildpb.PerfMode
}

// newPerfRunner creates a new PerfMode runner.
func newPerfRunner(props *golangbuildpb.PerfMode) *perfRunner {
	return &perfRunner{props: props}
}

// Run implements the runner interface for perfRunner.
func (r *perfRunner) Run(ctx context.Context, spec *buildSpec, opts runOptions) error {
	var (
		results    []byte
		extraAttrs map[string]string
		err        error
	)
	if isGoProject(spec.inputs.Project) {
		results, extraAttrs, err = runGoBenchmarks(ctx, spec, r.props, opts)
	} else {
		results, extraAttrs, err = runSubrepoBenchmarks(ctx, spec, r.props, opts)
	}
	if err != nil {
		return err
	}
	if opts.fetchOnly() {
		return nil
	}

	// Summarize results with benchstat.
	benchstatCmd := toolCmd(ctx, "benchstat", "-col", "toolchain@(baseline experiment)", "-ignore", "pgo,pkg,shortname", "-")
	benchstatCmd.Stdin = bytes.NewReader(results)
	formattedResults, err := cmdStepOutput(ctx, "benchstat", benchstatCmd, true)
	if err != nil {
		return err
	}
	topLevelLog(ctx, "benchmark results").Write(formattedResults)

	// Prepend extraAttrs. Note: we don't do this before benchstat, because it simplifies the "-ignore" pattern
	// and also because it more closely matches running benchstat on the output of the command, making it a bit
	// more reproducible for those that want to run it locally.
	var buf bytes.Buffer
	for key, value := range extraAttrs {
		fmt.Fprintf(&buf, "%s: %s\n", key, value)
	}
	buf.Write(results)

	// Upload benchmark results to perfdata.golang.org.
	return uploadBenchmarkResults(ctx, spec.auth, buf.Bytes())
}

func runGoBenchmarks(ctx context.Context, spec *buildSpec, perfProps *golangbuildpb.PerfMode, opts runOptions) ([]byte, map[string]string, error) {
	// Get a built Go toolchain or build it if necessary. This will be
	// our experiment toolchain.
	if err := getGo(ctx, spec, "", spec.goroot, spec.goSrc, false); err != nil {
		return nil, nil, err
	}

	// Get the baseline Go.
	gorootBaseline := filepath.Join(spec.workdir, "go_baseline")
	goBaselineSrc, err := sourceForBaseline(ctx, spec.auth, spec.goSrc, perfProps.Baseline)
	if err != nil {
		return nil, nil, err
	}
	if err := getGo(ctx, spec, "baseline", gorootBaseline, goBaselineSrc, false); err != nil {
		return nil, nil, err
	}

	// Get the tip of the benchmarks repo.
	benchmarksSrc, err := sourceForBranch(ctx, spec.auth, publicGoHost, "benchmarks", mainBranch)
	if err != nil {
		return nil, nil, err
	}

	// Fetch the benchmarks repository.
	benchmarksRoot := filepath.Join(spec.workdir, "benchmarks")
	if err := fetchRepo(ctx, benchmarksSrc, benchmarksRoot, spec.inputs); err != nil {
		return nil, nil, err
	}

	// If we only want to fetch, we're done.
	if opts.fetchOnly() {
		return nil, nil, nil
	}

	// Construct benchmark command.
	benchCmd := spec.goCmd(ctx, benchmarksRoot, "run",
		"./cmd/bench",
		"-goroot", spec.goroot,
		"-goroot-baseline", gorootBaseline,
		"-branch", spec.goSrc.branch,
		"-repository", "go",
	)

	var extraAttrs map[string]string
	if spec.goSrc.commit != nil {
		t, err := fetchCommitTime(ctx, spec.auth, spec.goSrc.commit)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to fetch commit time for %s: %w", spec.goSrc.asURL(), err)
		}
		extraAttrs = map[string]string{
			"experiment-commit":      spec.goSrc.commit.Id,
			"experiment-commit-time": t.In(time.UTC).Format(time.RFC3339Nano),
			"baseline-commit":        goBaselineSrc.commit.Id,
			"benchmarks-commit":      benchmarksSrc.commit.Id,
			"post-submit":            "true",
		}
	}

	// Run benchmarks.
	results, err := cmdStepOutput(ctx, "go run cmd/bench", benchCmd, false)
	return results, extraAttrs, err
}

func runSubrepoBenchmarks(ctx context.Context, spec *buildSpec, perfProps *golangbuildpb.PerfMode, opts runOptions) ([]byte, map[string]string, error) {
	// Fetch the subrepo at whatever we were triggered on.
	subrepoExperimentDir := filepath.Join(spec.workdir, spec.inputs.Project)
	if err := fetchRepo(ctx, spec.subrepoSrc, subrepoExperimentDir, spec.inputs); err != nil {
		return nil, nil, err
	}

	// Fetch the subrepo at whatever baseline is in the builder configuration.
	subrepoBaselineSrc, err := sourceForBaseline(ctx, spec.auth, spec.subrepoSrc, perfProps.Baseline)
	if err != nil {
		return nil, nil, err
	}
	subrepoBaselineDir := filepath.Join(spec.workdir, spec.inputs.Project+"_baseline")
	if err := fetchRepo(ctx, subrepoBaselineSrc, subrepoBaselineDir, spec.inputs); err != nil {
		return nil, nil, err
	}

	// Pick the baseline Go toolchain we're going to use, which is just the latest release
	// for the Go branch this builder is building against.
	goBaselineSrc, err := sourceForLatestGoRelease(ctx, spec.auth, spec.inputs.GoBranch)
	if err != nil {
		return nil, nil, err
	}

	// Get the baseline Go.
	gorootBaseline := filepath.Join(spec.workdir, "go_baseline")
	if err := getGo(ctx, spec, "baseline", gorootBaseline, goBaselineSrc, false); err != nil {
		return nil, nil, err
	}

	// Get the tip of the benchmarks repo.
	benchmarksSrc, err := sourceForBranch(ctx, spec.auth, publicGoHost, "benchmarks", mainBranch)
	if err != nil {
		return nil, nil, err
	}

	// Fetch the benchmarks repository.
	benchmarksRoot := filepath.Join(spec.workdir, "benchmarks")
	if err := fetchRepo(ctx, benchmarksSrc, benchmarksRoot, spec.inputs); err != nil {
		return nil, nil, err
	}

	// If we only want to fetch, we're done.
	if opts.fetchOnly() {
		return nil, nil, nil
	}

	// Construct benchmark command.
	benchCmd := goCmd(ctx, gorootBaseline, benchmarksRoot, "run",
		"./cmd/bench",
		"-goroot-baseline", gorootBaseline,
		"-subrepo", subrepoExperimentDir,
		"-subrepo-baseline", subrepoBaselineDir,
		"-branch", spec.goSrc.branch,
		"-repository", spec.inputs.Project,
	)

	// Add extra attributes. These will be added to the results later.
	var extraAttrs map[string]string
	if spec.subrepoSrc.commit != nil {
		t, err := fetchCommitTime(ctx, spec.auth, spec.subrepoSrc.commit)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to fetch commit time for %s: %w", spec.subrepoSrc.asURL(), err)
		}
		extraAttrs = map[string]string{
			"experiment-commit":      spec.subrepoSrc.commit.Id,
			"experiment-commit-time": t.In(time.UTC).Format(time.RFC3339Nano),
			"baseline-commit":        subrepoBaselineSrc.commit.Id,
			"toolchain-commit":       goBaselineSrc.commit.Id,
			"benchmarks-commit":      benchmarksSrc.commit.Id,
			"post-submit":            "true",
		}
	}

	// Run benchmarks.
	results, err := cmdStepOutput(ctx, "go run cmd/bench", benchCmd, false)
	return results, extraAttrs, err
}

func sourceForBaseline(ctx context.Context, auth *auth.Authenticator, src *sourceSpec, baseline string) (*sourceSpec, error) {
	if baseline == "parent" {
		return sourceForParent(ctx, auth, src)
	}
	if baseline == "latest_go_release" {
		if src.project != "go" {
			return nil, fmt.Errorf("the latest_go_release baseline is only supported for the go project")
		}
		return sourceForLatestGoRelease(ctx, auth, src.branch)
	}
	return sourceForRef(ctx, auth, publicGoHost, src.project, baseline)
}

func uploadBenchmarkResults(ctx context.Context, auth *auth.Authenticator, results []byte) (err error) {
	step, ctx := build.StartStep(ctx, "upload benchmark results")
	defer endInfraStep(step, &err) // Any failure in this function is an infrastructure failure.

	// Log the results we're going to upload before we do anything else.
	_, err = step.Log("results").Write(results)
	if err != nil {
		return err
	}

	// Create a perfstorage client.
	hc, err := auth.Client()
	if err != nil {
		return fmt.Errorf("auth.Client: %w", err)
	}
	client := &perfstorage.Client{BaseURL: "https://perfdata.golang.org", HTTPClient: hc}
	u := client.NewUpload(ctx)
	w, err := u.CreateFile("results")
	if err != nil {
		_ = u.Abort() // Intentionally ignored. This will usually generate an error, but we don't care.
		return fmt.Errorf("error creating perfdata file: %w", err)
	}
	// Write the results.
	if _, err := w.Write(results); err != nil {
		_ = u.Abort() // Intentionally ignored. This will usually generate an error, but we don't care.
		return fmt.Errorf("error writing perfdata file with contents %q: %w", results, err)
	}
	status, err := u.Commit()
	if err != nil {
		return fmt.Errorf("error committing perfdata file: %w", err)
	}

	// Write out the upload ID as a log.
	_, err = io.WriteString(step.Log("upload_id"), status.UploadID)
	return err
}
