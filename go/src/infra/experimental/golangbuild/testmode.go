// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	goversion "go/version"
	"hash/crc32"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/exp/slices"
	"golang.org/x/mod/modfile"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/luci/common/system/environ"
	"go.chromium.org/luci/luciexe/build"

	"infra/experimental/golangbuild/golangbuildpb"
	"infra/experimental/golangbuild/testweights"
)

// testRunner runs a non-strict subset of available tests. It requires a prebuilt
// toolchain to be available (it will not create one on-demand).
//
// This implements "test mode" for golangbuild.
type testRunner struct {
	props *golangbuildpb.TestMode
	shard testShard
}

// newTestRunner creates a new TestMode runner.
func newTestRunner(props *golangbuildpb.TestMode, gotShard *golangbuildpb.TestShard) (*testRunner, error) {
	shard := noSharding
	if gotShard != nil {
		shard = testShard{
			shardID: gotShard.ShardId,
			nShards: gotShard.NumShards,
		}
		if shard.shardID >= shard.nShards {
			return nil, fmt.Errorf("invalid test shard designation: shard ID is %d, num shards is %d", shard.shardID, shard.nShards)
		}
	}
	return &testRunner{props: props, shard: shard}, nil
}

// Run implements the runner interface for testRunner.
func (r *testRunner) Run(ctx context.Context, spec *buildSpec, opts runOptions) error {
	// Get a built Go toolchain and require it to be prebuilt if we're not in fetchOnly mode.
	// In fetchOnly mode, there's likely a human involved, and they could be debugging something
	// old that doesn't have a prebuilt binary for it anymore. In that case, we want to build it
	// on behalf of the human involved.
	if err := getGo(ctx, spec, "", spec.goroot, spec.goSrc, !opts.fetchOnly()); err != nil {
		return err
	}
	// Determine what ports to test.
	ports := []*golangbuildpb.Port{spec.inputs.Target}
	if spec.inputs.MiscPorts {
		// Note: There may be code changes in cmd/dist or cmd/go that have not
		// been fully reviewed yet, and it is a test error if goDistList fails.
		var err error
		ports, err = goDistList(ctx, spec, r.shard)
		if err != nil {
			return err
		}
	}
	// Run tests. (Also fetch dependencies if applicable.)
	if spec.inputs.Project == "go" {
		if opts.fetchOnly() {
			return nil
		}
		return runGoTests(ctx, spec, r.shard, ports)
	}
	// N.B. If we're going to run subrepo tests, then randomize the name of
	// the directory we fetch into. This helps prevent tests from relying on
	// specific paths into the repo on the builder. If we're only fetching,
	// then it's very likely there's a human involved, and we'd actually like
	// a nice deterministic name.
	repoDir, err := fetchSubrepo(ctx, spec, !opts.fetchOnly())
	if err != nil {
		return err
	}
	if opts.fetchOnly() {
		return nil
	}
	return runSubrepoTests(ctx, spec, repoDir, ports)
}

// testShard is a test shard identity that can be used to deterministically filter tests.
type testShard struct {
	shardID uint32 // The ID, in the range [0, nShards-1].
	nShards uint32 // Total number of shards (at least 1).
}

// shouldRunTest deterministically returns true for whether the shard identity should run
// the test by name. The name of the test doesn't matter, as long as it's consistent across
// test shards.
func (s testShard) shouldRunTest(name string) bool {
	return crc32.ChecksumIEEE([]byte(name))%s.nShards == s.shardID
}

// noSharding indicates that no sharding should take place. It represents executing the entirety
// of the test suite.
var noSharding = testShard{shardID: 0, nShards: 1}

func runGoTests(ctx context.Context, spec *buildSpec, shard testShard, ports []*golangbuildpb.Port) (err error) {
	step, ctx := build.StartStep(ctx, "run tests")
	defer endStep(step, &err)

	if spec.inputs.Project != "go" {
		return infraErrorf("runGoTests called for a subrepo builder")
	}
	gorootSrc := filepath.Join(spec.goroot, "src")

	if spec.inputs.CompileOnly {
		// If compiling any one port fails, keep going and report all at the end.
		g := new(errgroup.Group)
		g.SetLimit(runtime.NumCPU())
		var testErrors = make([]error, len(ports))
		for i, p := range ports {
			i, p := i, p
			portContext := addPortEnv(ctx, p, "GOMAXPROCS="+fmt.Sprint(max(1, runtime.NumCPU()/len(ports))))
			// TODO(go.dev/issue/62067): Dump the raw JSON to a file and log it once `go build -json` is
			// available.
			testCmd := spec.wrapTestCmd(portContext, spec.distTestCmd(portContext, gorootSrc, "", nil), "")
			g.Go(func() error {
				testErrors[i] = cmdStepRun(portContext, fmt.Sprintf("compile %s port", p), testCmd, false)
				return nil
			})
		}
		g.Wait()
		return errors.Join(testErrors...)
	}

	if len(ports) != 1 || !proto.Equal(ports[0], spec.inputs.Target) {
		return infraErrorf("testing multiple ports is only supported in CompileOnly mode")
	}

	// Determine what tests to run.
	//
	// If noSharding is true, tests will be left as the empty slice, which means to
	// use dist test's default behavior of running all tests.
	var tests []string
	if shard != noSharding {
		// Collect the list of tests for this shard.
		var err error
		tests, err = goDistTestList(ctx, spec, shard)
		if err != nil {
			return err
		}
		if len(tests) == 0 {
			// No tests were selected to run. Explicitly return early instead
			// of needlessly calling dist test and telling it to run no tests.
			return nil
		}
	}

	// Invoke go tool dist test (with -json flag).
	jsonDumpFile := filepath.Join(spec.workdir, "dist.testjson")
	testCmd := spec.wrapTestCmd(ctx, spec.distTestCmd(ctx, gorootSrc, "", tests), jsonDumpFile)
	if err := cmdStepRun(ctx, "go tool dist test -json", testCmd, false, jsonDumpFile); err != nil {
		return attachTestsFailed(err)
	}
	return nil
}

func goDistTestList(ctx context.Context, spec *buildSpec, shard testShard) (tests []string, err error) {
	step, ctx := build.StartStep(ctx, "list tests")
	defer endStep(step, &err)

	// Run go tool dist test -list.
	listCmd := spec.distTestListCmd(ctx, spec.goroot)
	listOutput, err := cmdStepOutput(ctx, "go tool dist test -list", listCmd, false)
	if err != nil {
		return nil, err
	}

	// Parse the output—each line is a test name.
	scanner := bufio.NewScanner(bytes.NewReader(listOutput))
	for scanner.Scan() {
		tests = append(tests, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parsing test list from dist: %w", err)
	}

	// Determine which tests to run.
	if _, ok := spec.experiments["golang.shard_by_weight"]; ok {
		tests = shardTestsByWeight(tests, shard)
	} else {
		tests = shardTestsByHash(tests, shard)
	}

	// Log the tests we're going to run.
	testList := strings.Join(tests, "\n")
	if len(tests) == 0 {
		testList = "(no tests selected)"
	}
	if _, err := io.WriteString(step.Log("tests"), testList); err != nil {
		return nil, err
	}
	return tests, nil
}

// shardTestsByHash filters tests down based on shard. The algorithm it
// uses to do so splits tests across shards by hashing the names and using
// the hash to index into the set of shards.
func shardTestsByHash(tests []string, shard testShard) []string {
	var filtered []string
	for _, name := range tests {
		if shard.shouldRunTest(name) {
			filtered = append(filtered, name)
		}
	}
	return filtered
}

// shardTestsByWeight filters tests down based on shard. The algorithm it
// uses involves first identifying long-running ('weighted') tests and
// dividing them across shards, then picking the shard ID described by shard.
// It then takes the short tests and shards them by hash. This is intended
// to strike a balance between sharding reproducibility and build latency by
// sharding work more evenly.
func shardTestsByWeight(tests []string, shard testShard) []string {
	var shortTests []string
	var longTests []string
	longWeight := 0
	for _, name := range tests {
		if weight := testweights.GoDistTest(name); weight > 1 {
			longWeight += weight
			longTests = append(longTests, name)
		} else {
			shortTests = append(shortTests, name)
		}
	}
	target := longWeight / int(shard.nShards)
	s := 0
	var shardTotal int
	var shardBucket []string
	for _, name := range longTests {
		shardTotal += testweights.GoDistTest(name)
		if s == int(shard.shardID) {
			shardBucket = append(shardBucket, name)
		}
		if s != int(shard.nShards-1) && shardTotal > target {
			s++
			shardTotal = 0
		}
	}
	return append(shardBucket, shardTestsByHash(shortTests, shard)...)
}

// fetchSubrepo fetches a target golang.org/x repository.
//
// It returns an infrastructure error if used on the main Go repository.
func fetchSubrepo(ctx context.Context, spec *buildSpec, randomizeDir bool) (repoDir string, err error) {
	if isGoProject(spec.inputs.Project) {
		return "", infraErrorf("fetchSubrepo called for a main Go repo builder")
	}
	if randomizeDir {
		repoDir, err = os.MkdirTemp(spec.workdir, "targetrepo") // Use a non-predictable base directory name.
		if err != nil {
			return "", err
		}
	} else {
		repoDir = "x_" + spec.inputs.Project
	}
	if err := fetchRepo(ctx, spec.subrepoSrc, repoDir, spec.inputs); err != nil {
		return "", err
	}
	return repoDir, nil
}

// runSubrepoTests discovers modules inside repoDir to test,
// fetches their dependencies, and tests the modules.
//
// It returns an infrastructure error if used on the main Go repository.
func runSubrepoTests(ctx context.Context, spec *buildSpec, repoDir string, ports []*golangbuildpb.Port) (err error) {
	step, ctx := build.StartStep(ctx, "run tests")
	defer endStep(step, &err)

	if isGoProject(spec.inputs.Project) {
		return infraErrorf("runSubrepoTests called for a main Go repo builder")
	}

	// Discover all modules in the subrepo,
	// then select modules to be tested.
	allModules, err := repoToModules(ctx, spec, repoDir)
	if err != nil {
		return err
	}
	var modules []module // Modules to test.
	for _, m := range allModules {
		// When testing on release branches like "release-branch.go1.23", check
		// the release branch Go version meets the module's minimum requirement.
		// If it doesn't and 'GOTOOLCHAIN=local' is set, log it visibly and skip
		// instead of failing with something like:
		//
		//	go: go.mod requires go >= 1.23.0 (running go 1.22.7; GOTOOLCHAIN=local)
		//
		// We never do this kind of skip for Go tip, so each module will be tested
		// on at least one builder regardless of its go directive (to catch it being
		// accidentally too high of a value).
		//
		// See go.dev/issue/69332.
		if goBranchVersion, ok := strings.CutPrefix(spec.inputs.GoBranch, "release-branch."); ok &&
			environ.FromCtx(setupModuleEnv(ctx, m)).Get("GOTOOLCHAIN") == "local" &&
			goversion.Compare(goBranchVersion+".999", m.Minimum) < 0 {
			logSkippedModule(ctx, m.Path, fmt.Sprintf("skipping because module requires %s but this builder "+
				"is testing with GOTOOLCHAIN=local and local toolchain %s.x", m.Minimum, goBranchVersion))
			continue
		}

		// In all other cases, include m to be tested.
		modules = append(modules, m)
	}
	if len(modules) == 0 {
		// No modules to test were discovered. Return early to avoid needing to handle this
		// case of having nothing to test below and to avoid having meaningless empty steps
		// in the "Steps & Logs" section.
		return nil
	}
	{ // TODO(go.dev/issue/65917): Come back to this after followup discussion; this special case covers gopls v0.17.0 needs.
		if spec.inputs.GoBranch == "release-branch.go1.21" && spec.inputs.Project == "tools" {
			// A special case for x/tools on release-branch.go1.21:
			// test only the x/tools/gopls nested module and nothing else in x/tools.
			// This module should work via the default GOTOOLCHAIN=auto setting (see setupModuleEnv)
			// and https://go.dev/doc/toolchain#select. Other parts of x/tools might
			// not work, which is WAI since they're not supported per Go release policy.
			// See go.dev/issue/65917.
			modules = slices.DeleteFunc(modules, func(m module) bool {
				return m.Path != "golang.org/x/tools/gopls"
			})
			if len(modules) != 1 {
				return infraErrorf("got %d modules selected in x/tools @ 1.21, want 1: %v", len(modules), modules)
			}
		}
	}

	// Fetch module dependencies ahead of time, to mark temporary network errors as an infra
	// failures and because 'go test' may not have network access (see spec.inputs.NoNetwork).
	if err := fetchDependencies(ctx, spec, modules); err != nil {
		return err
	}

	// Test modules in this specific subrepo.
	// If testing any one nested module or port fails, keep going and report all at the end.
	if spec.inputs.CompileOnly {
		return compileTestsInParallel(ctx, spec, modules, ports)
	} else if len(ports) != 1 || !proto.Equal(ports[0], spec.inputs.Target) {
		return infraErrorf("testing multiple ports is only supported in CompileOnly mode")
	}
	var testErrors []error
	for _, m := range modules {
		ctx := setupModuleEnv(ctx, m)
		jsonDumpFile := filepath.Join(spec.workdir, "go.testjson")
		testCmd := spec.wrapTestCmd(ctx, spec.goCmd(ctx, m.RootDir, spec.goTestArgs("./...")...), jsonDumpFile)
		if err := cmdStepRun(ctx, fmt.Sprintf("test %s module", m.Path), testCmd, false, jsonDumpFile); err != nil {
			testErrors = append(testErrors, err)
		}
	}
	return attachTestsFailed(errors.Join(testErrors...))
}

func logSkippedModule(ctx context.Context, modulePath, skipReason string) {
	step, ctx := build.StartStep(ctx, fmt.Sprintf("skip testing %s module", modulePath))
	_, _ = io.WriteString(step.Log("skip reason"), skipReason)
	step.End(nil)
}

func compileTestsInParallel(ctx context.Context, spec *buildSpec, modules []module, ports []*golangbuildpb.Port) error {
	g := new(errgroup.Group)
	g.SetLimit(runtime.NumCPU())
	var testErrors = make([]error, len(ports)*len(modules))
	for i, p := range ports {
		i := i
		portContext := addPortEnv(ctx, p, "GOMAXPROCS="+fmt.Sprint(max(1, runtime.NumCPU()/(len(ports)*len(modules)))))
		for _, m := range modules {
			portContext := setupModuleEnv(portContext, m)
			stepName := fmt.Sprintf("test %s module", m.Path)
			if len(ports) > 1 || !proto.Equal(p, spec.inputs.Target) {
				stepName += fmt.Sprintf(" for %s", p)
			}
			// TODO(go.dev/issue/62067): Dump the raw JSON to a file and log it once `go build -json` is
			// available.
			testCmd := spec.wrapTestCmd(portContext, spec.goCmd(portContext, m.RootDir, spec.goTestArgs("./...")...), "")
			if spec.inputs.CompileOnly && compileOptOut(spec.inputs.Project, p, m.Path) {
				stepName += " (skipped)"
				testCmd = command(portContext, "echo", "(skipped)")
			}
			g.Go(func() error {
				testErrors[i] = cmdStepRun(portContext, stepName, testCmd, false)
				return nil
			})
		}
	}
	g.Wait()
	return errors.Join(testErrors...)
}

// A module is a Go module located on disk.
type module struct {
	RootDir string // Module root directory on disk.
	Path    string // Module path specified in go.mod.
	Minimum string // Minimum Go toolchain version for the module. With the "go" prefix included, like "go1.22.0".
}

// repoToModules discovers and reports modules in repoDir to be tested.
//
// It also moves nested modules such that their relative paths are not
// predictable. This is done to catch unintended cases where a test in
// a given module accidentally depends on something outside its module
// boundary.
func repoToModules(ctx context.Context, spec *buildSpec, repoDir string) (modules []module, err error) {
	step, ctx := build.StartStep(ctx, "discover modules")
	defer endInfraStep(step, &err) // Any failure in this function is an infrastructure failure.

	// Discover all modules that we wish to test. See go.dev/issue/32528.
	if err := filepath.WalkDir(repoDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (strings.HasPrefix(d.Name(), ".") || strings.HasPrefix(d.Name(), "_") || d.Name() == "testdata") {
			// Skip directories that we're not looking to support having testable modules in.
			// TODO(go.dev/issue/65267): Move nested untestable modules too.
			return fs.SkipDir
		}
		if goModFile := d.Name() == "go.mod" && !d.IsDir(); goModFile {
			modPath, goVersion, err := modPathAndGo(path)
			if err != nil {
				return err
			}
			modules = append(modules, module{
				RootDir: filepath.Dir(path),
				Path:    modPath,
				Minimum: goVersion,
			})
		}
		return nil
	}); err != nil {
		return nil, err
	}
	moduleList := fmt.Sprint(modules)
	if len(modules) == 0 {
		moduleList = "(no modules discovered)"
	}
	if _, err := io.WriteString(step.Log("modules"), moduleList); err != nil {
		return nil, err
	}

	keepNestedModsInsideRepo := map[string]bool{
		"tools":     true, // A local replace directive in x/tools/gopls as of 2023-06-08.
		"telemetry": true, // A local replace directive in x/telemetry/godev as of 2023-06-08.
		"exp":       true, // A local replace directive in x/exp/slog/benchmarks/{zap,zerolog}_benchmarks as of 2023-06-08.
		"oscar":     true, // A local module reference via go.work in x/oscar as of 2024-08-05.
	}
	if !keepNestedModsInsideRepo[spec.inputs.Project] || spec.experiment("golang.force_test_outside_repository") {
		// Move nested modules to directories that aren't predictably-relative to each other
		// to catch accidental reads across nested module boundaries. See go.dev/issue/34352.
		//
		// Sort modules by increasing nested-ness, and do this
		// in reverse order for all but the first (root) module.
		slices.SortFunc(modules, func(a, b module) int {
			return strings.Count(a.RootDir, string(filepath.Separator)) - strings.Count(b.RootDir, string(filepath.Separator))
		})
		for i := len(modules) - 1; i >= 1; i-- {
			randomDir, err := os.MkdirTemp(spec.workdir, "nestedmod")
			if err != nil {
				return nil, err
			}
			newDir := filepath.Join(randomDir, filepath.Base(randomDir)) // Use a non-predictable base directory name.
			if err := os.Rename(modules[i].RootDir, newDir); err != nil {
				return nil, err
			}
			modules[i].RootDir = newDir
		}
	}

	return modules, nil
}

// modPathAndGo reports the module path and Go version in the given go.mod file.
// The returned Go version has a "go" prefix included, like "go1.22.0".
func modPathAndGo(goModFile string) (modPath string, goVersion string, _ error) {
	b, err := os.ReadFile(goModFile)
	if err != nil {
		return "", "", err
	}
	f, err := modfile.ParseLax(goModFile, b, nil)
	if err != nil {
		return "", "", err
	} else if f.Module == nil {
		return "", "", fmt.Errorf("go.mod file %q has no module statement", goModFile)
	}
	if f.Go == nil {
		// If the go directive is missing, go 1.16 is implied.
		// See https://go.dev/ref/mod#go-mod-file-go.
		return f.Module.Mod.Path, "go1.16", nil
	}
	return f.Module.Mod.Path, "go" + f.Go.Version, nil
}

// goDistList uses 'go tool dist list' to get a list of all non-broken ports,
// excluding ones that definitely already have a pre-submit builder,
// and returns those that match the provided shard.
func goDistList(ctx context.Context, spec *buildSpec, shard testShard) (ports []*golangbuildpb.Port, err error) {
	step, ctx := build.StartStep(ctx, "list ports")
	defer endStep(step, &err)

	// Run go tool dist list -json.
	//
	// Notably, we leave out -broken flag to get only non-broken ports.
	listCmd := spec.distListCmd(ctx, spec.goroot)
	listOutput, err := cmdStepOutput(ctx, "go tool dist list -json", listCmd, false)
	if err != nil {
		return nil, err
	}

	// Parse the JSON output and collect available ports.
	var allPorts []struct {
		GOOS, GOARCH string
		FirstClass   bool
	}
	err = json.Unmarshal(listOutput, &allPorts)
	if err != nil {
		return nil, fmt.Errorf("parsing port list from dist: %w", err)
	}
	for _, p := range allPorts {
		if p.GOOS == "" || p.GOARCH == "" {
			return nil, fmt.Errorf("go tool dist list returned an invalid GOOS/GOARCH pair: %#v", p)
		}
		switch firstClassWithPre := p.FirstClass && (p.GOOS != "darwin" && p.GOARCH != "arm"); {
		case firstClassWithPre:
			// There's enough machine capacity and speed for almost
			// all first-class ports to have a pre-submit builder,
			// and there's not enough benefit to include them here.
			continue
		case p.GOOS == "ios" && p.GOARCH == "arm64":
			// TODO(go.dev/issue/61761): Add misc-compile coverage for the ios/arm64 port (iOS).
			continue
		case p.GOOS == "ios" && p.GOARCH == "amd64":
			// TODO(go.dev/issue/61760): Add misc-compile coverage for the ios/amd64 port (iOS Simulator).
			continue
		case p.GOOS == "android":
			// TODO(go.dev/issue/61762): Add misc-compile coverage for the GOOS=android ports (Android).
			continue
		}
		ports = append(ports, &golangbuildpb.Port{Goos: p.GOOS, Goarch: p.GOARCH})
	}
	// Split up the ports into buckets, and pick one for this shard.
	bucketSize := len(ports) / int(shard.nShards)
	if bucketSize*int(shard.nShards) < len(ports) {
		// Round up when the number of ports doesn't divide evenly by shard count.
		bucketSize++
	}
	i := min(bucketSize*int(shard.shardID), len(ports))
	j := min(bucketSize*int(shard.shardID+1), len(ports))
	ports = ports[i:j]

	portList := fmt.Sprint(ports)
	if len(ports) == 0 {
		portList = "(no ports selected)"
	}
	if _, err := io.WriteString(step.Log("ports"), portList); err != nil {
		return nil, err
	}
	return ports, nil
}

// compileOptOut is a policy function that reports whether the provided
// port and module pair is considered opted out of compile-only testing.
//
// TODO(dmitshur,heschi): Ideally we want to have policy configured in
// one place, more likely in main.star than here. If so, factor it out.
func compileOptOut(project string, p *golangbuildpb.Port, modulePath string) bool {
	const (
		optOut                           = true
		performCompileOnlyTestingAsUsual = false // Long name so that it stands out. It's the rare case here.
	)
	ps := p.Goos + "-" + p.Goarch
	switch project {
	case "benchmarks":
		if p.Goos == "plan9" {
			// Dependency "github.com/coreos/go-systemd/v22/journal" fails to build on Plan 9.
			return optOut
		}
		if p.Goarch == "wasm" {
			// Dependencies "github.com/blevesearch/mmap-go", "go.etcd.io/bbolt", and "github.com/coreos/go-systemd/v22/journal"
			// fail to build. Also "golang.org/x/benchmarks/driver" fails to build.
			return optOut
		}
	case "build":
		// build is a special repository for internal Go build infrastructure needs.
		// It relies only on real pre- and post-submit testing, not compile-only testing.
		if p.Goos == "darwin" {
			// Except darwin, which doesn't yet have pre-submit coverage,
			// so use compile-only coverage to help out.
			return performCompileOnlyTestingAsUsual
		}
		return optOut
	case "debug":
		if p.Goarch == "wasm" {
			// Dependency "github.com/chzyer/readline" fails to build.
			return optOut
		}
	case "exp":
		switch modulePath {
		case "golang.org/x/exp/event":
			if ps == "wasip1-wasm" {
				// Dependency "github.com/sirupsen/logrus" fails to build on wasip1/wasm.
				return optOut
			}
		case "golang.org/x/exp/shiny":
			switch ps {
			case "darwin-arm64", "darwin-amd64", "linux-mips64", "linux-mips64le",
				"linux-ppc64", "linux-ppc64le", "linux-s390x", "openbsd-amd64":
				return performCompileOnlyTestingAsUsual
			default:
				// x/exp/shiny fails to build on most cross-compile platforms, largely because
				// of x/mobile dependencies.
				return optOut
			}
		}
	case "mobile":
		// mobile fails to build on all cross-compile platforms. This is somewhat expected
		// given the nature of the repository. Leave this as a blanket policy for now.
		return optOut
	case "pkgsite":
		// See go.dev/issue/61341.
		if p.Goos == "plan9" {
			// Dependency "github.com/lib/pq" fails to build on Plan 9.
			return optOut
		}
		if ps == "wasip1-wasm" {
			// Dependency "github.com/lib/pq" fails to build on wasip1/wasm.
			return optOut
		}
	case "pkgsite-metrics":
		if ps == "wasip1-wasm" {
			// Dependency "github.com/lib/pq" fails to build on wasip1/wasm.
			return optOut
		}
		if ps == "aix-ppc64" || p.Goos == "plan9" {
			// Dependency "github.com/apache/thrift/lib/go/thrift" fails to build on aix/ppc64 and Plan 9.
			return optOut
		}
	case "vuln":
		if p.Goos == "plan9" {
			// Dependency "github.com/google/go-cmdtest" fails to build on Plan 9.
			return optOut
		}
	case "vulndb":
		if ps == "aix-ppc64" {
			// Dependency "github.com/go-git/go-billy/v5/osfs" fails to build on aix/ppc64.
			// See go.dev/issue/58308.
			return optOut
		}
		if ps == "wasip1-wasm" {
			// Dependency "github.com/go-git/go-billy/v5/osfs" fails to build on wasip1/wasm.
			return optOut
		}
		if p.Goos == "plan9" {
			// Dependency "github.com/cyphar/filepath-securejoin" fails to build on Plan 9.
			return optOut
		}
	}
	// The default policy decision is not to opt out.
	return performCompileOnlyTestingAsUsual
}
