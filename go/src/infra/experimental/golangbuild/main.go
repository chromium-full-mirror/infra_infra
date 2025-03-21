// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Binary golangbuild is a luciexe binary that builds and tests the code for the
// Go project. It supports building and testing go.googlesource.com/go as well as
// Go project subrepositories (e.g. go.googlesource.com/net) and on different branches.
//
// To build and run this locally end-to-end, follow these steps:
//
//	luci-auth login -scopes "https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/gerritcodereview https://www.googleapis.com/auth/cloud-platform"
//	cat > build.jsonpb <<EOF
//	{
//		"builder": {
//			"project": "go",
//			"bucket": "ci",
//			"builder": "linux-amd64"
//		},
//		"input": {
//			"properties": {
//				"project": "go",
//				"bootstrap_version": "1.20.6",
//				"host": {
//					"goarch": "amd64",
//					"goos": "linux"
//				},
//				"target": {
//					"goarch": "amd64",
//					"goos": "linux"
//				},
//				"git_cache": "gitcache",
//				"tools_cache": "tools"
//			},
//			"gitiles_commit": {
//				"host": "go.googlesource.com",
//				"project": "go",
//				"id": "27301e8247580e456e712a07d68890dc1e857000",
//				"ref": "refs/heads/master"
//			}
//		}
//	}
//	EOF
//	export SWARMING_SERVER=https://chromium-swarm.appspot.com
//	LUCIEXE_FAKEBUILD=./build.jsonpb golangbuild
//
// Modify `build.jsonpb` as needed in order to try different paths. The format of
// `build.jsonpb` is a JSON-encoded protobuf with schema `go.chromium.org/luci/buildbucket/proto.Build`.
// The input.properties field of this protobuf follows the `infra/experimental/golangbuildpb.Inputs`
// schema which represents input parameters that are specific to this luciexe, but may also contain
// namespaced properties that are injected by different services. For instance, CV uses the
// "$recipe_engine/cq" namespace.
//
// As an example, to try out a "try bot" path, try the following `build.jsonpb`:
//
//	{
//		"builder": {
//			"project": "go",
//			"bucket": "try",
//			"builder": "linux-amd64"
//		},
//		"input": {
//			"properties": {
//				"project": "go",
//				"bootstrap_version": "1.20.6",
//				"host": {
//					"goarch": "amd64",
//					"goos": "linux"
//				},
//				"target": {
//					"goarch": "amd64",
//					"goos": "linux"
//				},
//				"tools_cache": "tools",
//				"git_cache": "gitcache",
//				"$recipe_engine/cq": {
//					"active": true,
//					"runMode": "DRY_RUN"
//				}
//			},
//			"gerrit_changes": [
//				{
//					"host": "go-review.googlesource.com",
//					"project": "go",
//					"change": 460376,
//					"patchset": 1
//				}
//			]
//		}
//	}
//
// NOTE: by default, a luciexe fake build will discard the temporary directory created to run
// the build. If you'd like to retain the contents of the directory, specify a working directory
// to the golangbuild luciexe via the `--working-dir` flag. Be careful about where this working
// directory lives; particularly, make sure it isn't a subdirectory of a Go module a directory
// containing a go.mod file.
//
// ## Contributing
//
// To keep things organized and consistent, keep to the following guidelines:
//   - Only functions generate steps. Methods never generate steps.
//   - Keep step presentation and command execution separate from logic where possible
//     (which will make it easier to write unit tests).
//   - Steps should be function-scoped. Steps should be created at the start of a function
//     with the step end immediately deferred to function exit.
//   - Prefer wrapping errors (%w) instead of rendering them (%v). Errors contain rich
//     information like log links that may get lost when rendering them.
//
// ## Experiments
//
// When adding new functionality, consider putting it behind an experiment. Experiments are
// made available in the buildSpec and are propagated from the builder definition.
// Experiments in the builder definition are given a probability of being enabled on any given
// build, but always manifest in the current build as either "on" or "off."
// Experiments should have a name like "golang.my_specific_new_functionality" and should
// be checked for via spec.experiment("golang.my_specific_new_functionality").
//
// Experiments can be disabled at first (no work needs to be done on the builder definition),
// rolled out, and then tested in a real build environment via `led`
//
//	led get-build ... | \
//	led edit \
//	  -experiment golang.my_specific_new_functionality=true | \
//	led launch
//
// or via `bb add -ex "+golang.my_specific_new_functionality" ...`.
//
// Experiments can be enabled on LUCIEXE_FAKEBUILD runs through the "experiments" property (array
// of strings) on "input."
//
// ### Current experiments
//
//   - golang.force_test_outside_repository: Can be used to force running tests
//     from outside the repository to catch accidental reads outside of module
//     boundaries despite the repository not having opted-in to this test
//     behavior.
//   - golang.shard_by_weight: Use an alternative test sharding strategy that uses
//     precomputed weights to achieve a better balance of work across shards.
//   - luci.best_effort_platform: set by LUCI on less-supported platforms.
//     Affects which tools are downloaded from CIPD.
//
// ## Set up gomote environment locally
//
// If golangbuild is invoked with GOMOTE_SETUP=<project>/<bucket>/<builder>, then the luciexe
// protocol will be bypassed, and it will set up that builder's tools and environment in the
// current working directory. Then, it will interpret all command-line arguments as another
// command to execute.
//
// This is used during gomote setup as a wrapper for the buildlet, but can also be used locally
// to partially replicate an environment for a builder. For example, the following command will
// replicate the environment for gotip-linux-amd64 and drop you into a shell:
//
//	GOMOTE_SETUP=golang/ci-workers/gotip-linux-amd64-test_only golangbuild bash
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"unicode/utf8"

	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/experimental/golangbuild/golangbuildpb"
)

var ioProps = build.RegisterSplitProperty[*golangbuildpb.Inputs, *golangbuildpb.Outputs]("")

func main() {
	// If we're just performing setup for a gomote, skip the luciexe protocol stuff.
	if builderName := os.Getenv("GOMOTE_SETUP"); builderName != "" {
		if err := gomoteSetup(context.Background(), builderName, os.Args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: gomote setup: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if buildID := os.Getenv("GOMOTE_REPRO"); buildID != "" {
		if err := gomoteRepro(context.Background(), buildID, os.Args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: gomote repro: %v\n", err)
			os.Exit(1)
		}
		return
	}

	build.Main(func(ctx context.Context, args []string, st *build.State) error {
		ctx = withTopLevelLogger(ctx, st)
		spec, runErr := run(ctx, args, st, ioProps.GetInput(ctx))

		// Extract any links from the error. nil errors are OK and have no links.
		links := extractLinks(runErr)

		// Populate output properties.
		var outpb golangbuildpb.Outputs
		if runErr != nil {
			outpb.Failure = new(golangbuildpb.FailureSummary)
			outpb.Failure.Description = runErr.Error()
			for _, link := range links {
				outpb.Failure.Links = append(outpb.Failure.Links, &golangbuildpb.Link{
					Name: link.name,
					Url:  link.url,
				})
			}
			outpb.Failure.TestsFailed = errorTestsFailed(runErr)
		}
		if spec != nil {
			if spec.goSrc != nil {
				outpb.Sources = []*golangbuildpb.Source{spec.goSrc.asSource()}
			}
			if spec.subrepoSrc != nil {
				outpb.Sources = append(outpb.Sources, spec.subrepoSrc.asSource())
			}
		}
		ioProps.SetOutput(ctx, &outpb)

		// Set summary markdown.
		var sb strings.Builder
		if spec != nil {
			if spec.goSrc != nil || spec.subrepoSrc != nil {
				if spec.goSrc != nil {
					fmt.Fprintf(&sb, "* %s", spec.goSrc.asMarkdown())
				}
				if spec.subrepoSrc != nil {
					fmt.Fprintf(&sb, " / %s", spec.subrepoSrc.asMarkdown())
				}
				fmt.Fprint(&sb, "\n\n")
			}
			extraLogLinks := topLevelLogLinks(ctx)
			if len(extraLogLinks) != 0 {
				fmt.Fprintf(&sb, "Top-level logs:\n")
				for _, link := range extraLogLinks {
					fmt.Fprintf(&sb, "* [%s](%s)\n", link.name, link.url)
				}
				fmt.Fprintln(&sb)
			}
		}
		if runErr != nil {
			testsFailed := errorTestsFailed(runErr)
			if testsFailed {
				fmt.Fprintf(&sb, "To reproduce, try `gomote repro %d`.\n\n", st.Build().Id)
			} else {
				if e := runErr.Error(); !strings.ContainsAny(e, "\n`") {
					// Simple error, viable to put it in a Markdown code span.
					fmt.Fprintf(&sb, "Error: `%s`\n\n", e)
				} else {
					// Put the error in a Markdown code block (using indentation)
					// to avoid error content from being interpreted as Markdown.
					fmt.Fprintf(&sb, "Error:\n\n%s\n\n", "\t"+strings.ReplaceAll(e, "\n", "\n\t"))
				}
			}
			fmt.Fprintf(&sb, "Additional links for debugging:\n")
			for _, link := range links {
				fmt.Fprintf(&sb, "* [%s](%s)\n", link.name, link.url)
			}
		}
		within4KB := func(s string) string {
			if len(s) <= 4000 {
				return s
			}
			const suffix = "…\\<TRUNCATED\\>\n\n(Build summary truncated to fit within 4 KB; see Steps & Logs if more details are needed.)"
			var i int
			for ; !utf8.RuneStart(s[4000-len(suffix)-i]); i++ { // Walk back to nearest UTF-8 rune start.
			}
			return s[:4000-len(suffix)-i] + suffix
		}
		st.SetSummaryMarkdown(within4KB(sb.String()))

		return runErr
	})
}

func run(ctx context.Context, args []string, st *build.State, inputs *golangbuildpb.Inputs) (spec *buildSpec, err error) {
	log.Printf("run starting")

	// Collect enabled experiments.
	experiments := make(map[string]struct{})
	for _, ex := range st.Build().GetInput().GetExperiments() {
		experiments[ex] = struct{}{}
	}

	// Install some tools we'll need, including a bootstrap toolchain.
	toolsRoot, err := installTools(ctx, inputs, experiments)
	if err != nil {
		return nil, err
	}
	log.Printf("installed tools")

	// Install tools in context.
	ctx = withToolsRoot(ctx, toolsRoot)

	// Get the CAS instance.
	ctx, err = casInstanceFromEnv(ctx)
	if err != nil {
		return nil, infraErrorf("casInstanceFromEnv: %w", err)
	}

	// Define working directory.
	cwd, err := os.Getwd()
	if err != nil {
		return nil, infraErrorf("get CWD")
	}

	spec, err = deriveBuildSpec(ctx, cwd, experiments, st.Build(), inputs)
	if err != nil {
		return nil, infraWrap(err)
	}

	// Set up environment.
	ctx = spec.setEnv(ctx)
	ctx, err = spec.installDatastoreClient(ctx)
	if err != nil {
		return nil, err
	}

	// Select a runner based on the mode, then initialize and invoke it.
	var rn runner
	switch inputs.GetMode() {
	case golangbuildpb.Mode_MODE_ALL:
		rn = newAllRunner(inputs.GetAllMode())
	case golangbuildpb.Mode_MODE_COORDINATOR:
		rn = newCoordRunner(inputs.GetCoordMode())
	case golangbuildpb.Mode_MODE_BUILD:
		rn = newBuildRunner(inputs.GetBuildMode())
	case golangbuildpb.Mode_MODE_TEST:
		rn, err = newTestRunner(inputs.GetTestMode(), inputs.GetTestShard())
	case golangbuildpb.Mode_MODE_PERF:
		rn = newPerfRunner(inputs.GetPerfMode())
	}
	if err != nil {
		return nil, infraErrorf("initializing runner: %w", err)
	}
	return spec, rn.Run(ctx, spec, noOptions)
}

// runner is an interface that provides an abstraction over golangbuild's various modes.
//
// Every mode basically requires the same setup at the beginning of the build; runner
// determines what to do once we have all that.
type runner interface {
	Run(ctx context.Context, spec *buildSpec, opts runOptions) error
}

// runOptions are a set of options to pass to Run.
type runOptions uint64

const (
	noOptions runOptions = 0

	// fetchOnly indicates that Run should stop after fetching and constructing all
	// necessary dependencies. Generally this just means cloning repositories, but it's
	// also in this spirit of this option to run make.bash, for example. More concretely,
	// the runner must not run any tests or trigger any downstream builds.
	fetchOnly runOptions = 1 << iota
)

func (o runOptions) fetchOnly() bool {
	return o&fetchOnly != 0
}
