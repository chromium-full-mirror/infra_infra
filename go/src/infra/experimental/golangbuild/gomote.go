// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/system/environ"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/hardcoded/chromeinfra"
	"go.chromium.org/luci/lucictx"
	"go.chromium.org/luci/luciexe"

	"go.chromium.org/infra/experimental/golangbuild/golangbuildpb"
)

// gomoteSetup sets up a basic environment for a gomote from a builder name
// then invokes the command in args. This path must closely, if not identically,
// match the setup path before calling into one of the mode-specific runners.
func gomoteSetup(ctx context.Context, builderName string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command to run specified")
	}

	// Set up basic LUCI env.
	ctx, cwd, err := setupBasicLUCIEnv(ctx)
	if err != nil {
		return err
	}

	log.Printf("obtaining builder info for %s...", builderName)

	// Get builder info. In this context, we have to contact buildbucket,
	// since we're assuming that we're not participating in the luciexe
	// protocol.
	inputs, experiments, err := getBuilderInfo(ctx, builderName)
	if err != nil {
		return infraErrorf("obtaining builder info: %w", err)
	}

	log.Printf("installing tools...")

	// Install some tools we'll need, including a bootstrap toolchain.
	toolsRoot, err := installTools(ctx, inputs, experiments)
	if err != nil {
		return infraErrorf("installing tools: %w", err)
	}

	// Install tools in context.
	ctx = withToolsRoot(ctx, toolsRoot)

	// Get the CAS instance and set it in the environment.
	ctx, err = casInstanceFromEnv(ctx)
	if err != nil {
		return infraErrorf("casInstanceFromEnv: %w", err)
	}

	// Set up the rest of the environment.
	goroot := filepath.Join(cwd, "goroot")
	gopath := filepath.Join(cwd, "gopath")
	gocacheDir := filepath.Join(cwd, "gocache")
	goplscacheDir := filepath.Join(cwd, "goplscache")
	xdgcacheDir := filepath.Join(cwd, "xdgcache")
	ctx = setupEnv(ctx, inputs, builderName, goroot, gopath, gocacheDir, goplscacheDir, xdgcacheDir)

	// Log the environment changes.
	want := environ.FromCtx(ctx)
	base := environ.System()
	log.Printf("environment changes:\n%s", diffEnv(base, want))

	// Check if we want to fetch an extra project for the user out of convenience.
	var extraProject, extraDir string
	if !isGoProject(inputs.Project) {
		extraProject = inputs.Project
		// Put it in $PWD/x_${inputs.Project}, which is not where the builders put it, but makes so much
		// more sense for humans than whatever random temp directory we create. Simultaneously, we add the
		// "x_" prefix to reduce the chance of directory collisions.
		extraDir = filepath.Join(cwd, "x_"+inputs.Project)
	} else if isGoProject(inputs.Project) && inputs.GetMode() == golangbuildpb.Mode_MODE_PERF {
		extraProject = "benchmarks"
		extraDir = filepath.Join(cwd, "benchmarks") // Match the perf builder's behavior.
	}
	if extraProject != "" {
		// Fetch the subrepo at tip on behalf of the user.
		authenticator := createAuthenticator(ctx)
		subrepoSrc, err := sourceForBranch(ctx, authenticator, publicGoHost, extraProject, mainBranch)
		if err != nil {
			return fmt.Errorf("sourceForBranch: %w", err)
		}
		if err := fetchRepo(ctx, subrepoSrc, extraDir, inputs); err != nil {
			return err
		}
	}

	// Execute the command in args.
	cmd := command(ctx, args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("invoking %s...", cmd.String())
	return cmd.Run()
}

// gomoteRepro sets up the full environment for a gomote that matches a specific
// build, then invokes the command in args, if any. This path must closely, if not
// identically, match the setup path for a specific build.
func gomoteRepro(ctx context.Context, buildID string, args []string) error {
	// Set up basic LUCI env.
	ctx, cwd, err := setupBasicLUCIEnv(ctx)
	if err != nil {
		return err
	}

	log.Printf("obtaining build info for %s...", buildID)

	// Get build info. In this context, we have to contact buildbucket,
	// since we're assuming that we're not participating in the luciexe
	// protocol.
	build, inputs, experiments, err := getBuildInfo(ctx, buildID)
	if err != nil {
		return infraErrorf("obtaining builder info: %w", err)
	}

	// If we're looking at a coordinator build, print a helpful message in case someone
	// was led astray. We can consider having picking a child build on the user's behalf
	// automatically, but it also seems wrong to surprise those working on the infra.
	if inputs.GetMode() == golangbuildpb.Mode_MODE_COORDINATOR {
		log.Printf("***********************************************************************")
		log.Printf("Warning: running repro for coordinator build!")
		log.Printf("***********************************************************************")
		log.Printf("Unless you're debugging the infrastructure, this is likely in error.")
		log.Printf("If your intent is to debug a failing test, try re-running this")
		log.Printf("command with one of the following builds:")
		children, err := getChildBuilds(ctx, buildID)
		if err != nil {
			return err
		}
		for _, id := range children {
			log.Printf("\t * %s", id)
		}
		log.Printf("***********************************************************************")
	}

	log.Printf("installing tools...")

	// Install some tools we'll need, including a bootstrap toolchain.
	toolsRoot, err := installTools(ctx, inputs, experiments)
	if err != nil {
		return infraErrorf("installing tools: %w", err)
	}

	// Install tools in context.
	ctx = withToolsRoot(ctx, toolsRoot)

	// Get the CAS instance and set it in the environment.
	ctx, err = casInstanceFromEnv(ctx)
	if err != nil {
		return infraErrorf("casInstanceFromEnv: %w", err)
	}

	// Create a build spec, since we'll be fetching code.
	spec, err := deriveBuildSpec(ctx, cwd, experiments, build, inputs)
	if err != nil {
		return infraWrap(err)
	}

	// Set up Go project specific env.
	ctx = spec.setEnv(ctx)
	ctx, err = spec.installDatastoreClient(ctx)
	if err != nil {
		return err
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
		return infraErrorf("initializing runner: %w", err)
	}
	if err := rn.Run(ctx, spec, fetchOnly); err != nil {
		return err
	}

	// Log the environment changes.
	want := environ.FromCtx(ctx)
	base := environ.System()
	log.Printf("environment changes:\n%s", diffEnv(base, want))

	if len(args) == 0 {
		// No command to run.
		return nil
	}

	// Execute the command in args.
	cmd := command(ctx, args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("invoking %s...", cmd.String())
	return cmd.Run()
}

func setupBasicLUCIEnv(ctx context.Context) (context.Context, string, error) {
	// Define working directory.
	cwd, err := os.Getwd()
	if err != nil {
		return ctx, "", infraErrorf("get CWD: %v", err)
	}

	log.Printf("setting up build environment for gomote at %s...", cwd)

	// Set up the basic parts of the environment first.
	tmpDir := filepath.Join(cwd, "tmp")
	if err := os.MkdirAll(tmpDir, os.ModePerm); err != nil {
		return ctx, cwd, err
	}
	cacheDir := filepath.Join(cwd, "cache")
	if err := os.MkdirAll(cacheDir, os.ModePerm); err != nil {
		return ctx, cwd, err
	}

	// Set all luciexe temporary directories as well.
	for _, key := range luciexe.TempDirEnvVars {
		ctx = addEnv(ctx, fmt.Sprintf("%s=%s", key, tmpDir))
	}

	// Set up LUCI_CONTEXT.
	ctx = lucictx.SetLUCIExe(ctx, &lucictx.LUCIExe{
		CacheDir: cacheDir,
	})
	return ctx, cwd, nil
}

func getBuilderInfo(ctx context.Context, builderName string) (*golangbuildpb.Inputs, map[string]struct{}, error) {
	id, err := parseBuilderID(builderName)
	if err != nil {
		return nil, nil, err
	}

	// Contact buildbucket to obtain information about the builder.
	host := chromeinfra.BuildbucketHost
	if bbCtx := lucictx.GetBuildbucket(ctx); bbCtx != nil {
		if bbCtx.GetHostname() != "" {
			host = bbCtx.Hostname
		}
	}
	hc, err := createAuthenticator(ctx).Client()
	if err != nil {
		return nil, nil, fmt.Errorf("authenticator.Client: %w", err)
	}
	bc := bbpb.NewBuildersPRPCClient(&prpc.Client{
		C:       hc,
		Host:    host,
		Options: prpc.DefaultOptions(),
	})
	b, err := bc.GetBuilder(ctx, &bbpb.GetBuilderRequest{Id: id})
	if err != nil {
		return nil, nil, fmt.Errorf("getting builder info for %q: %w", builderName, err)
	}

	// Parse the properties out of the JSON string in the builder config.
	inputs := new(golangbuildpb.Inputs)
	if err := json.Unmarshal([]byte(b.GetConfig().GetProperties()), inputs); err != nil {
		return nil, nil, err
	}

	// Collect all experiments, but only pass through those that are on 100% of the time.
	experiments := make(map[string]struct{})
	for name, chance := range b.GetConfig().GetExperiments() {
		if chance != 100 {
			continue
		}
		experiments[name] = struct{}{}
	}
	return inputs, experiments, nil
}

func getBuildInfo(ctx context.Context, buildID string) (*bbpb.Build, *golangbuildpb.Inputs, map[string]struct{}, error) {
	// Contact buildbucket to obtain information about the builder.
	host := chromeinfra.BuildbucketHost
	if bbCtx := lucictx.GetBuildbucket(ctx); bbCtx != nil {
		if bbCtx.GetHostname() != "" {
			host = bbCtx.Hostname
		}
	}
	hc, err := createAuthenticator(ctx).Client()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("authenticator.Client: %w", err)
	}
	bc := bbpb.NewBuildsPRPCClient(&prpc.Client{
		C:       hc,
		Host:    host,
		Options: prpc.DefaultOptions(),
	})
	id, err := strconv.ParseInt(buildID, 10, 64)
	if err != nil {
		return nil, nil, nil, infraErrorf("converting build ID %s to integer: %v", buildID, err)
	}
	b, err := bc.GetBuild(ctx, &bbpb.GetBuildRequest{Id: id, Mask: &bbpb.BuildMask{AllFields: true}})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("getting build info for %q: %w", buildID, err)
	}

	// Parse the properties out of the proto in the build definition.
	inputs := new(golangbuildpb.Inputs)
	json, err := protojson.Marshal(b.GetInput().GetProperties())
	if err != nil {
		panic(err) // This should be impossible.
	}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(json, inputs); err != nil {
		return nil, nil, nil, err
	}

	// Collect all experiments, but only pass through those that are on 100% of the time.
	experiments := make(map[string]struct{})
	for _, name := range b.GetInput().GetExperiments() {
		experiments[name] = struct{}{}
	}
	return b, inputs, experiments, nil
}

func getChildBuilds(ctx context.Context, buildID string) ([]string, error) {
	host := chromeinfra.BuildbucketHost
	if bbCtx := lucictx.GetBuildbucket(ctx); bbCtx != nil {
		if bbCtx.GetHostname() != "" {
			host = bbCtx.Hostname
		}
	}
	hc, err := createAuthenticator(ctx).Client()
	if err != nil {
		return nil, fmt.Errorf("authenticator.Client: %w", err)
	}
	bc := bbpb.NewBuildsPRPCClient(&prpc.Client{
		C:       hc,
		Host:    host,
		Options: prpc.DefaultOptions(),
	})
	id, err := strconv.ParseInt(buildID, 10, 64)
	if err != nil {
		return nil, infraErrorf("converting build ID %s to integer: %v", buildID, err)
	}
	resp, err := bc.SearchBuilds(ctx, &bbpb.SearchBuildsRequest{Predicate: &bbpb.BuildPredicate{ChildOf: id}})
	if err != nil {
		return nil, fmt.Errorf("getting children for %s: %w", buildID, err)
	}
	var children []string
	for _, b := range resp.Builds {
		children = append(children, fmt.Sprintf("%d", b.Id))
	}
	return children, nil
}

func parseBuilderID(builderName string) (*bbpb.BuilderID, error) {
	c := strings.SplitN(builderName, "/", 3)
	if len(c) != 3 {
		return nil, fmt.Errorf("builder name must consist of 3 sections: <project>/<bucket>/<name>")
	}
	return &bbpb.BuilderID{
		Project: c[0],
		Bucket:  c[1],
		Builder: c[2],
	}, nil
}

func diffEnv(base, want environ.Env) string {
	// Find the likely verb for exporting environment variables.
	verb := "export"
	if runtime.GOOS == "windows" {
		verb = "set"
	}

	var sb strings.Builder
	// First, emit all new environment variables or modified ones.
	_ = want.Iter(func(name, value string) error {
		if baseValue, ok := base.Lookup(name); !ok || value != baseValue {
			fmt.Fprintf(&sb, "%s %s=%s\n", verb, name, value)
		}
		return nil
	})
	// Next, explicitly unset environment variables that are not present in want.
	_ = base.Iter(func(name, baseValue string) error {
		if _, ok := want.Lookup(name); !ok {
			fmt.Fprintf(&sb, "%s %s=\n", verb, name)
		}
		return nil
	})
	return sb.String()
}
