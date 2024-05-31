// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	clouddatastore "cloud.google.com/go/datastore"
	"google.golang.org/api/option"

	"go.chromium.org/luci/auth"
	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/api/gerrit"
	lucierrors "go.chromium.org/luci/common/errors"
	gerritpb "go.chromium.org/luci/common/proto/gerrit"
	"go.chromium.org/luci/common/system/environ"
	"go.chromium.org/luci/gae/impl/cloud"
	"go.chromium.org/luci/hardcoded/chromeinfra"
	"go.chromium.org/luci/luciexe/build"
	sauth "go.chromium.org/luci/server/auth"

	"infra/experimental/golangbuild/golangbuildpb"
)

// buildSpec specifies what a single build will begin doing.
type buildSpec struct {
	auth *auth.Authenticator

	builderName        string
	bucket             string
	workdir            string
	goroot             string
	gopath             string
	gocacheDir         string
	goplscacheDir      string
	priority           int32
	golangbuildVersion string

	inputs *golangbuildpb.Inputs

	goSrc      *sourceSpec // the Go repo spec
	subrepoSrc *sourceSpec // the x/ repo spec, or nil if inputs.Project == "go"
	invokedSrc *sourceSpec // the commit/change we were invoked with

	invocation string // current ResultDB invocation

	experiments map[string]struct{}
}

func deriveBuildSpec(ctx context.Context, cwd string, experiments map[string]struct{}, st *build.State, inputs *golangbuildpb.Inputs) (*buildSpec, error) {
	authenticator := createAuthenticator(ctx)

	// Build the sourceSpec we were invoked with.
	gitilesCommit := st.Build().GetInput().GetGitilesCommit()
	var gerritChange *bbpb.GerritChange
	if changes := st.Build().GetInput().GetGerritChanges(); len(changes) > 1 {
		return nil, fmt.Errorf("no support for multiple GerritChanges")
	} else if len(changes) != 0 {
		gerritChange = changes[0]
	}
	var changedProject, changedBranch string
	switch {
	case gerritChange != nil && gitilesCommit != nil:
		return nil, fmt.Errorf("only a Gerrit change or a Gitiles commit is supported, not both")
	case gerritChange == nil && gitilesCommit != nil:
		changedProject = gitilesCommit.Project
		changedBranch = refToBranch(gitilesCommit.Ref)
	case gerritChange != nil && gitilesCommit == nil:
		changedProject = gerritChange.Project
		hc, err := authenticator.Client()
		if err != nil {
			return nil, fmt.Errorf("authenticator.Client: %w", err)
		}
		gc, err := gerrit.NewRESTClient(hc, gerritChange.Host, true)
		if err != nil {
			return nil, fmt.Errorf("gerrit.NewRESTClient: %w", err)
		}
		changeInfo, err := gc.GetChange(ctx, &gerritpb.GetChangeRequest{
			Number:  gerritChange.Change,
			Project: gerritChange.Project,
		})
		if err != nil {
			return nil, fmt.Errorf("gc.GetChange: %w", err)
		}
		changedBranch = changeInfo.Branch
	default:
		return nil, fmt.Errorf("no commit or change specified for build and test")
	}
	if !isGoProject(inputs.Project) && changedProject != inputs.Project && !isGoProject(changedProject) {
		// This case is something like a "build" commit for an "image" build, which
		// doesn't make any sense.
		return nil, fmt.Errorf("unexpected change and project pairing: %s vs. %s", changedProject, inputs.Project)
	}

	// Figure out what our Go and subrepo commits are, but retain
	// which one we were invoked with.
	invokedSrc := &sourceSpec{
		project: changedProject,
		branch:  changedBranch,
		commit:  gitilesCommit,
		change:  gerritChange,
		// TODO(mknyszek): Cherry-pick and/or rebase change(s) onto branch if it's
		// not a dry-run. Currently all we have are dry-runs, and we need to be
		// careful that child builds (build mode and test mode) also understand that
		// it's a dry run. Passing that information would be complicated and hacky
		// at present, but in the future go.chromium.org/luci/luciexe/build/cv could
		// be extended to expose the full CV proto, which we should then pass onto
		// child builds before changing this.
		cherryPick: false,
	}

	// For now, the internal repository is only supported for invokedSrc, so hard-code
	// publicGoHost below for lookups related to invokedSrc.
	//
	// TODO: In the future, we will probably have subrepos in the security repository.
	// Which version of Go will we want to test against in that case?

	var goSrc, subrepoSrc *sourceSpec
	var err error
	if isGoProject(invokedSrc.project) {
		goSrc = invokedSrc
		if !isGoProject(inputs.Project) {
			// We're testing the tip of inputs.Project's main branch against a Go commit.
			subrepoSrc, err = sourceForBranch(ctx, authenticator, publicGoHost, inputs.Project, mainBranch)
			if err != nil {
				return nil, fmt.Errorf("sourceForBranch: %w", err)
			}
		}
		if inputs.GoCommit != "" {
			return nil, fmt.Errorf("GoCommit can be set only when invoked in a project other than 'go'")
		}
	} else {
		subrepoSrc = invokedSrc
		if inputs.GoCommit == "" {
			// We're testing the tip of inputs.GoBranch against a commit to inputs.Project.
			goSrc, err = sourceForBranch(ctx, authenticator, publicGoHost, "go", inputs.GoBranch)
			if err != nil {
				return nil, fmt.Errorf("sourceForBranch: %w", err)
			}
		} else {
			// We're testing a commit on inputs.GoBranch that was already selected for us.
			goSrc, err = sourceForGoBranchAndCommit(publicGoHost, inputs.GoBranch, inputs.GoCommit)
			if err != nil {
				return nil, fmt.Errorf("sourceForGoBranchAndCommit: %w", err)
			}
		}
	}

	// Validate BuildersToTriggerAfterToolchainBuild invariant.
	if !isGoProject(inputs.Project) && len(inputs.GetCoordMode().GetBuildersToTriggerAfterToolchainBuild()) != 0 {
		return nil, fmt.Errorf("specified builders to trigger for unsupported project")
	}

	if inputs.NoNetwork {
		// Return a helpful error if the system requirements
		// for disabling external network are unmet.
		if inputs.Host.Goos != "linux" {
			return nil, fmt.Errorf("NoNetwork is not supported on %q", inputs.Host.Goos)
		}
		for _, cmd := range [...]string{"unshare", "ip", "sh"} {
			if _, err := exec.LookPath(cmd); err != nil {
				return nil, fmt.Errorf("NoNetwork needs %s in $PATH: %w", cmd, err)
			}
		}
	}

	priority := st.Build().GetInfra().GetSwarming().GetPriority()
	if priority == 0 {
		beCfg := st.Build().GetInfra().GetBackend().GetConfig()
		if beCfg != nil {
			for k, v := range beCfg.AsMap() {
				if k == "priority" {
					if p, ok := v.(float64); ok {
						priority = int32(p)
					}
				}
			}
		}
	}

	return &buildSpec{
		auth:               authenticator,
		builderName:        st.Build().GetBuilder().GetBuilder(),
		bucket:             st.Build().GetBuilder().GetBucket(),
		workdir:            cwd,
		goroot:             filepath.Join(cwd, "goroot"),
		gopath:             filepath.Join(cwd, "gopath"),
		gocacheDir:         filepath.Join(cwd, "gocache"),
		goplscacheDir:      filepath.Join(cwd, "goplscache"),
		priority:           priority,
		golangbuildVersion: st.Build().GetExe().GetCipdVersion(),
		inputs:             inputs,
		invocation:         st.Build().GetInfra().GetResultdb().GetInvocation(),
		goSrc:              goSrc,
		subrepoSrc:         subrepoSrc,
		invokedSrc:         invokedSrc,
		experiments:        experiments,
	}, nil
}

func (b *buildSpec) setEnv(ctx context.Context) context.Context {
	return setupEnv(ctx, b.inputs, b.builderName, b.goroot, b.gopath, b.gocacheDir, b.goplscacheDir)
}

func setupEnv(ctx context.Context, inputs *golangbuildpb.Inputs, builderName, goroot, gopath, gocacheDir, goplscacheDir string) context.Context {
	env := environ.FromCtx(ctx)
	env.Load(inputs.Env)
	env.Set("GOOS", inputs.Target.Goos)
	env.Set("GOARCH", inputs.Target.Goarch)
	env.Set("GOHOSTOS", inputs.Host.Goos)
	env.Set("GOHOSTARCH", inputs.Host.Goarch)
	env.Set("GOROOT_BOOTSTRAP", filepath.Join(toolsRoot(ctx), "go_bootstrap"))
	env.Set("GOPATH", gopath) // Explicitly set to an empty per-build directory, to avoid reusing the implicit default one.
	env.Set("GOBIN", "")
	env.Set("GOROOT", "")           // Clear GOROOT because it's possible someone has one set locally, e.g. for luci-go development.
	env.Set("GOTOOLCHAIN", "local") // golangbuild scope includes selecting the exact Go toolchain version, so always use that local one.
	env.Set("GOCACHE", gocacheDir)
	env.Set("GOPLSCACHE", goplscacheDir)
	env.Set("GO_BUILDER_NAME", builderName)
	if inputs.LongTest {
		env.Set("GO_TEST_SHORT", "0") // Tell 'dist test' to operate in longtest mode. See go.dev/issue/12508.
	}
	// Use our tools before the system tools. Notably, use raw Git rather than the Chromium wrapper.
	env.Set("PATH", fmt.Sprintf("%v%c%v", filepath.Join(toolsRoot(ctx), "bin"), os.PathListSeparator, env.Get("PATH")))

	if inputs.Host.Goos == "windows" {
		env.Set("GOBUILDEXIT", "1") // On Windows, emit exit codes from .bat scripts. See go.dev/issue/9799.
		ccPath := filepath.Join(toolsRoot(ctx), "cc/windows/gcc64/bin")
		if inputs.Target.Goarch == "386" { // Not obvious whether this should check host or target. As of writing they never differ.
			ccPath = filepath.Join(toolsRoot(ctx), "cc/windows/gcc32/bin")
		}
		env.Set("PATH", fmt.Sprintf("%v%c%v", env.Get("PATH"), os.PathListSeparator, ccPath))
	}
	if inputs.Target.Goarch == "wasm" {
		// Add go_*_wasm_exec and the appropriate Wasm runtime to PATH.
		env.Set("PATH", fmt.Sprintf("%v%c%v", filepath.Join(goroot, "misc/wasm"), os.PathListSeparator, env.Get("PATH")))
		switch {
		case inputs.Target.Goos == "js":
			env.Set("PATH", fmt.Sprintf("%v%c%v", filepath.Join(toolsRoot(ctx), "nodejs/bin"), os.PathListSeparator, env.Get("PATH")))
		case inputs.Target.Goos == "wasip1" && env.Get("GOWASIRUNTIME") == "wasmtime",
			inputs.Target.Goos == "wasip1" && env.Get("GOWASIRUNTIME") == "wazero":
			env.Set("PATH", fmt.Sprintf("%v%c%v", filepath.Join(toolsRoot(ctx), env.Get("GOWASIRUNTIME")), os.PathListSeparator, env.Get("PATH")))
		}
	}
	if inputs.ClangVersion != "" {
		// Set up clang (and other LLVM tools, like llvm-symbolizer) in PATH. Then, set CC to clang and CXX to clang++ to actually use it.
		clangBin := filepath.Join(toolsRoot(ctx), "clang", "bin")
		env.Set("PATH", fmt.Sprintf("%v%c%v", env.Get("PATH"), os.PathListSeparator, clangBin))
		env.Set("CC", "clang")
		env.Set("CXX", "clang++")
	}
	if inputs.TestTimeoutScale != 0 {
		// Set the test timeout scale, which is understood by `go tool dist`.
		env.Set("GO_TEST_TIMEOUT_SCALE", strconv.Itoa(int(inputs.TestTimeoutScale)))
	}

	return env.SetInCtx(ctx)
}

// setupModuleEnv applies module-specific environment variables.
// These should be discussed in advance, and kept to a minimum.
func setupModuleEnv(ctx context.Context, m module) context.Context {
	switch m.Path {
	default:
		return ctx
	case "golang.org/x/tools/gopls":
		// setupEnv currently overrides GOTOOLCHAIN to local to catch and report
		// what happens with the exact local toolchain it selected. Experiment with
		// not doing that in one module for reasons motivated in go.dev/issue/67749.
		env := environ.FromCtx(ctx)
		env.Remove("GOTOOLCHAIN")
		return env.SetInCtx(ctx)
	}
}

func addEnv(ctx context.Context, add ...string) context.Context {
	env := environ.FromCtx(ctx)
	for _, e := range add {
		env.SetEntry(e)
	}
	return env.SetInCtx(ctx)
}

func addPortEnv(ctx context.Context, target *golangbuildpb.Port, extraEnv ...string) context.Context {
	return addEnv(ctx, append(extraEnv, "GOOS="+target.Goos, "GOARCH="+target.Goarch)...)
}

// goTestArgs returns go command arguments that test the specified import path patterns.
func (b *buildSpec) goTestArgs(patterns ...string) []string {
	args := []string{"test", "-json"}
	if b.inputs.CompileOnly {
		return append(append(args, "-c", "-o", os.DevNull), patterns...)
	}
	if !b.inputs.LongTest {
		args = append(args, "-short")
	}
	if b.inputs.RaceMode {
		args = append(args, "-race")
	}
	if b.inputs.TestTimeoutScale != 0 {
		timeout := time.Duration(b.inputs.TestTimeoutScale) * (10 * time.Minute)
		args = append(args, fmt.Sprintf("-timeout=%s", timeout))
	}
	return append(args, patterns...)
}

// distTestListCmd returns an exec.Cmd for executing `go tool dist test -list`.
//
// dir is the directory to run the command from.
//
// It automatically applies additional dist flags based on the buildSpec (e.g. -race).
func (b *buildSpec) distTestListCmd(ctx context.Context, dir string) *exec.Cmd {
	args := []string{"tool", "dist", "test", "-list"}
	args = append(args, b.distTestFlags()...)
	return b.goCmd(ctx, dir, args...)
}

// distTestCmd returns an exec.Cmd for executing `go tool dist test`.
//
// dir is the directory to run the command from.
//
// runRx and testNames are optional, mutually exclusive flags that select
// a subset of dist tests to run using dist test's command-line interface.
// (See 'go tool dist test -help'.)
//
// If json is true, passes the -json flag, producing `go test -json`-compatible output.
// Note: -json is not supported before Go 1.21.
//
// TODO(go.dev/issue/59990): Delete the json argument when it becomes always true.
//
// It automatically applies additional dist flags based on the buildSpec (e.g. -race).
func (b *buildSpec) distTestCmd(ctx context.Context, dir, runRx string, testNames []string, json bool) *exec.Cmd {
	args := []string{"tool", "dist", "test"}
	if json {
		args = append(args, "-json")
	}
	args = append(args, b.distTestFlags()...)
	if runRx != "" {
		args = append(args, "-run="+runRx)
	}
	args = append(args, testNames...)
	return b.goCmd(ctx, dir, args...)
}

// distTestFlags returns just the flags that we should pass to `go tool dist test`
// based on the spec.
func (b *buildSpec) distTestFlags() []string {
	var args []string
	if b.inputs.CompileOnly {
		args = append(args, "-compile-only")
	}
	if b.inputs.LongTest {
		// dist test doesn't have a flag to control longtest mode,
		// so this is handled in buildSpec.setEnv instead of here.
	}
	if b.inputs.RaceMode {
		args = append(args, "-race")
	}
	return args
}

// distListCmd returns an exec.Cmd for executing `go tool dist list -json`.
// dir is the directory to run the command from.
func (b *buildSpec) distListCmd(ctx context.Context, dir string) *exec.Cmd {
	return b.goCmd(ctx, dir, "tool", "dist", "list", "-json")
}

const cloudProject = "golang-ci-luci"

func (b *buildSpec) installDatastoreClient(ctx context.Context) (context.Context, error) {
	// TODO(mknyszek): Enable auth only when not in a fake build.
	ts, err := b.auth.TokenSource()
	if err != nil {
		return nil, lucierrors.Annotate(err, "failed to initialize the token source").Err()
	}
	client, err := clouddatastore.NewClient(ctx, cloudProject, option.WithTokenSource(ts))
	if err != nil {
		return nil, lucierrors.Annotate(err, "failed to instantiate the datastore client").Err()
	}
	cfg := &cloud.ConfigLite{
		ProjectID: cloudProject,
		DS:        client,
	}
	return cfg.Use(ctx), nil
}

// wrapTestCmd wraps cmd with 'rdb' and 'result_adapter' to send test results
// to ResultDB. cmd must be a test command that emits a JSON stream in the
// https://go.dev/cmd/test2json#hdr-Output_Format format. If dumpJSONFile is
// non-empty, the raw JSON output is written to that file.
//
// If external network access is to be disabled, cmd is also prefixed with 'unshare'.
//
// It edits cmd in place but for convenience also returns cmd back to its caller.
func (b *buildSpec) wrapTestCmd(ctx context.Context, cmd *exec.Cmd, dumpJSONFile string) *exec.Cmd {
	if b.inputs.NoNetwork {
		// Disable external network access for the test command.
		// Permit internal loopback access.
		cmd.Args = []string{
			"unshare", "--net", "--map-root-user", "--",
			"sh", "-c", "ip link set dev lo up && " + strings.Join(cmd.Args, " "),
		}
	}

	// Compute all the test tags and variants we want to send to ResultDB.
	rdbArgs := b.rdbStreamArgs(ctx)

	// Assemble args and update the command.
	//
	// Note: result_adapter is invoked with the flag -v=false, which means that the output
	// it logs should correspond to "go test" output in non-verbose mode. This is generally
	// much easier to read for humans and maintains compatibility with watchflakes rules that
	// match on the entire log. Full structured test output still gets sent to ResultDB, even
	// with this flag set.
	cmd.Path = toolPath(ctx, "rdb")
	args := []string{cmd.Path, "stream"}
	args = append(args, rdbArgs...)
	args = append(args, "--", toolPath(ctx, "result_adapter"), "go", "-v=false")
	if dumpJSONFile != "" {
		args = append(args, "-dump-json", dumpJSONFile)
	}
	args = append(args, "--")
	args = append(args, cmd.Args...)
	cmd.Args = args
	return cmd
}

func (b *buildSpec) rdbStreamArgs(ctx context.Context) []string {
	rdbArgs := []string{
		"-var", fmt.Sprintf("goos:%s", b.inputs.Target.Goos),
		"-var", fmt.Sprintf("goarch:%s", b.inputs.Target.Goarch),
		"-var", fmt.Sprintf("host_goos:%s", b.inputs.Host.Goos),
		"-var", fmt.Sprintf("host_goarch:%s", b.inputs.Host.Goarch),
		"-var", fmt.Sprintf("builder:%s", b.builderName),
		"-var", fmt.Sprintf("go_branch:%s", b.inputs.GoBranch),
		"-tag", fmt.Sprintf("bootstrap_version:%s", b.inputs.BootstrapVersion),
	}
	if b.inputs.RaceMode {
		rdbArgs = append(rdbArgs, "-tag", "run_mod:race")
	}
	if b.inputs.LongTest {
		rdbArgs = append(rdbArgs, "-tag", "run_mod:longtest")
	}
	if b.inputs.XcodeVersion != "" {
		rdbArgs = append(rdbArgs, "-tag", "xcode_version:"+b.inputs.XcodeVersion)
	}
	if b.inputs.NodeVersion != "" {
		rdbArgs = append(rdbArgs, "-tag", "node_version:"+b.inputs.NodeVersion)
	}
	if botID, ok := environ.FromCtx(ctx).Lookup("SWARMING_BOT_ID"); ok {
		rdbArgs = append(rdbArgs, "-tag", "swarming_bot_id:"+botID)
	} else {
		rdbArgs = append(rdbArgs, "-tag", "swarming_bot_id:unknown")
	}

	// If we don't have an invocation in the build already, create one. This can happen
	// if the builder isn't defined such that buildbucket creates an invocation for us,
	// or if we're running golangbuild in an environment without an invocation (for example,
	// LUCIEXE_FAKEBUILD). It's fine to create multiple invocations in these contexts.
	if b.invocation == "" {
		// N.B. There's a realm for every buildbucket bucket, which is where invocations get
		// created by default. Do the same here.
		rdbArgs = append(rdbArgs, "-new", "-realm", fmt.Sprintf("golang:%s", b.bucket))
	}
	return rdbArgs
}

func goScriptCmd(ctx context.Context, goroot, script string) *exec.Cmd {
	dir := filepath.Join(goroot, "src")
	cmd := command(ctx, filepath.FromSlash("./"+script))
	cmd.Dir = dir
	return cmd
}

// goCmd creates a command for running 'go {args}' in dir.
func (b *buildSpec) goCmd(ctx context.Context, dir string, args ...string) *exec.Cmd {
	return goCmd(ctx, b.goroot, dir, args...)
}

// goCmd creates a command for running 'go {args}' for the toolchain at goroot in dir.
func goCmd(ctx context.Context, goroot, dir string, args ...string) *exec.Cmd {
	env := environ.FromCtx(ctx)
	// Ensure the go binary found in PATH is the same as the one we're about to execute.
	env.Set("PATH", fmt.Sprintf("%v%c%v", filepath.Join(goroot, "bin"), os.PathListSeparator, env.Get("PATH")))
	cmd := command(env.SetInCtx(ctx), filepath.Join(goroot, "bin", "go"), args...)
	cmd.Dir = dir
	return cmd
}

func (b *buildSpec) experiment(ex string) bool {
	_, ok := b.experiments[ex]
	return ok
}

func command(ctx context.Context, bin string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = environ.FromCtx(ctx).Sorted()
	return cmd
}

func refToBranch(ref string) string {
	const branchRefPrefix = "refs/heads/"
	if strings.HasPrefix(ref, branchRefPrefix) {
		return ref[len(branchRefPrefix):]
	}
	return ""
}

// isGoProject reports whether a project should be treated as the main Go repo.
// For the moment, we develop security fixes in a project named golang/go-private.
func isGoProject(name string) bool {
	return name == "go" || name == "golang/go-private"
}

func createAuthenticator(ctx context.Context) *auth.Authenticator {
	authOpts := chromeinfra.SetDefaultAuthOptions(auth.Options{
		Scopes: append([]string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/gerritcodereview",
		}, sauth.CloudOAuthScopes...),
	})
	return auth.NewAuthenticator(ctx, auth.SilentLogin, authOpts)
}
