// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vpython

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"go.chromium.org/luci/common/system/environ"
	"go.chromium.org/luci/common/system/exitcode"
	"go.chromium.org/luci/common/system/filesystem"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

const (
	testDataDir          = "testdata"
	testMainRunScriptENV = "_VPYTHON_MAIN_TEST_RUN_SCRIPT"

	behaveExactlyLikeVpythonENV = "_VPYTHON_MAIN_TEST_PASSTHROUGH"
	vpythonTestBinaryEnv        = "_VPYTHON_MAIN_TEST_BINARY"
)

var vpythonDebug = flag.Bool("vpython.debug", false, "Enable vpython debug otuput.")
var vpythonTestCase = flag.String("vpython.testcase", "", "Run a specific test case. Useful for debugging.")

func init() {
	// Do we need to behave exactly like vpython.exe?
	env := environ.System()
	if env.Get(behaveExactlyLikeVpythonENV) != "" {
		Main(false)
	}
}

func TestMainFunc(t *testing.T) {
	t.Skip("requires python2")

	self, err := os.Executable()
	if err != nil {
		t.Fatalf("could not get executable path: %s", err)
	}
	os.Setenv(vpythonTestBinaryEnv, self)

	// Are we a spawned subprocess of TestMainFunc?
	env := environ.System()
	if v := env.Get(testMainRunScriptENV); v != "" {
		os.Exit(testMainRunDelegate(self, v))
		return
	}

	testCases := loadTestCases(t, self)

	// Execute each test case in parallel.
	for _, tc := range testCases {
		if *vpythonTestCase == "" || tc.name == *vpythonTestCase {
			t.Run(tc.name, func(t *testing.T) {
				tc.run(t, env.Clone())
			})
		}
	}
}

func runStartupOverhead(b *testing.B, spec string) {
	self, err := os.Executable()
	if err != nil {
		b.Fatalf("could not get executable path: %s", err)
	}
	os.Setenv(vpythonTestBinaryEnv, self)

	td, err := os.MkdirTemp(b.TempDir(), "vpython")
	if err != nil {
		b.Fatalf("could not get executable path: %s", err)
	}
	defer func() {
		if err := filesystem.RemoveAll(td); err != nil {
			b.Logf("Failed to remove test dir %q: %s", td, err)
		}
	}()

	c, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	tc := testCase{
		self:   self,
		name:   "bench_nop",
		script: filepath.Join(testDataDir, "bench_nop.py"),
		spec:   filepath.Join(testDataDir, spec),
	}

	// Vpython environment is lazy setup after first called
	tdc := tc.getDelegateCommand(c, td, environ.System())
	_ = tdc.Run(b)
	_ = tdc.Wait(b)

	b.ResetTimer()
	for range b.N {
		tdc := tc.getDelegateCommand(c, td, environ.System())
		_ = tdc.Run(b)
		_ = tdc.Wait(b)
	}
}

func BenchmarkStartupOverheadEmptyWheel(b *testing.B) {
	runStartupOverhead(b, "empty.vpython")
}

func BenchmarkStartupOverheadCertifiWheel(b *testing.B) {
	runStartupOverhead(b, "bench_certifi.vpython")
}

type testDelegateParams struct {
	Args []string
}

type testDelegateCommand struct {
	*exec.Cmd

	output bytes.Buffer
	tc     *testCase
	params testDelegateParams
	env    environ.Env
}

func (tdc *testDelegateCommand) prepare() {
	tdc.env.Set(testMainRunScriptENV, encodeEnvironmentParam(&tdc.params))
	tdc.Env = tdc.env.Sorted()
}

func (tdc *testDelegateCommand) Start() error {
	tdc.prepare()
	return tdc.Cmd.Start()
}

func (tdc *testDelegateCommand) Run(t testing.TB) error {
	tdc.prepare()
	err := tdc.Cmd.Run()
	t.Logf("Test output for %q:\n%s", tdc.tc.name, tdc.output.Bytes())
	return err
}

func (tdc *testDelegateCommand) Wait(t testing.TB) error {
	err := tdc.Cmd.Wait()
	t.Logf("Test output for %q:\n%s", tdc.tc.name, tdc.output.Bytes())
	return err
}

func (tdc *testDelegateCommand) CheckOutput(t testing.TB) bool {
	matches := bytes.Equal(
		bytes.ReplaceAll(tdc.output.Bytes(), []byte("\r\n"), []byte("\n")),
		tdc.tc.output)
	if !matches {
		t.Errorf("Outputs do not match. Expected:\n%s", tdc.tc.output)
	}
	return matches
}

type testCase struct {
	self   string
	name   string
	script string
	output []byte
	spec   string
}

func loadTestCases(t *testing.T, self string) []testCase {
	var testCases []testCase
	testCaseErrors := 0
	fis, err := os.ReadDir(testDataDir)
	if err != nil {
		t.Fatalf("could not read test directory %q: %v", testDataDir, err)
	}
	for _, fi := range fis {
		ext := filepath.Ext(fi.Name())
		if ext != ".py" {
			continue
		}

		script := filepath.Join(testDataDir, fi.Name())
		base := script[:len(script)-len(ext)]
		outputPath := base + ".output"

		content, err := os.ReadFile(outputPath)
		if err != nil && !os.IsNotExist(err) {
			t.Errorf("could not load output for %q at %q: %v", script, outputPath, err)
			testCaseErrors++
			continue
		}

		specPath := script + ".vpython"
		if _, err := os.Stat(specPath); os.IsNotExist(err) {
			specPath = filepath.Join(testDataDir, "empty.vpython")
		}

		testCases = append(testCases, testCase{
			self:   self,
			name:   filepath.Base(base),
			script: script,
			output: content,
			spec:   specPath,
		})
	}
	switch {
	case testCaseErrors > 0:
		t.Fatalf("errors encountered while loading test cases")
	case len(testCases) == 0:
		t.Fatalf("no test cases found")
	}

	return testCases
}

func (tc *testCase) String() string { return tc.name }

func (tc *testCase) getDelegateCommand(c context.Context, root string, env environ.Env) *testDelegateCommand {
	args := []string{
		"-vpython-root", root,
	}
	if *vpythonDebug {
		args = append(args, "-vpython-log-level", "debug")
	}
	if tc.spec != "" {
		args = append(args, "-vpython-spec", tc.spec)
	}
	args = append(args, "-u", tc.script)

	tdc := testDelegateCommand{
		tc: tc,
		params: testDelegateParams{
			Args: args,
		},
		env: env,
	}

	tdc.Cmd = exec.CommandContext(c, tdc.tc.self, "-test.run", "^TestMainFunc$")
	tdc.Stdout = &tdc.output
	tdc.Stderr = os.Stderr

	return &tdc
}

func (tc *testCase) run(t *testing.T, env environ.Env) {
	t.Parallel()

	ftt.Run(fmt.Sprintf(`Testing %q`, tc), t, func(t *ftt.Test) {
		td, err := os.MkdirTemp(t.TempDir(), "vpython")
		assert.Loosely(t, err, should.BeNil)
		defer func() {
			if err := filesystem.RemoveAll(td); err != nil {
				t.Logf("Failed to remove test dir %q: %s", td, err)
			}
		}()

		// Make sure the process is killed via defer.
		c, cancelFunc := context.WithCancel(context.Background())
		defer cancelFunc()

		switch tc.name {
		case "test_signals":
			// Signal forwarding is not supported on Windows.
			if runtime.GOOS != "windows" {
				tc.runTestSignals(c, t, td, env)
			}
		case "test_exit_code":
			tc.runCommon(c, t, td, env, 42)
		case "test_bypass":
			tc.runBypass(c, t, td, env)
		default:
			tc.runCommon(c, t, td, env, 0)
		}
	})
}

func (tc *testCase) runCommon(c context.Context, t testing.TB, td string, env environ.Env, exitCode int) {
	tdc := tc.getDelegateCommand(c, td, env)

	err := tdc.Run(t)
	if rc, ok := exitcode.Get(err); ok {
		assert.Loosely(t, rc, should.Equal(exitCode))
	} else {
		assert.Loosely(t, err, should.BeNil)
	}
	assert.Loosely(t, tdc.CheckOutput(t), should.BeTrue)
}

func (tc *testCase) runTestSignals(c context.Context, t testing.TB, td string, env environ.Env) {
	// We set up a mechanism for our subprocess to signal to us that it has
	// established its signal handlers and is ready for testing.
	//
	// The simplest cross-platform way to do this is to create a flag file, have
	// the subprocess delete it when it's ready, and poll for its deletion.
	signalFilePath := filepath.Join(td, "started.flag")
	if err := filesystem.Touch(signalFilePath, time.Time{}, 0664); err != nil {
		t.Fatalf("Could not create signal file %q: %v", signalFilePath, err)
	}

	tdc := tc.getDelegateCommand(c, td, env)
	tdc.params.Args = append(tdc.params.Args, signalFilePath)

	if err := tdc.Start(); err != nil {
		t.Fatalf("Failed to start subprocess: %s", err)
	}

	t.Logf("Waiting for signal file to disappear...")
	subprocessStarted := false
	for !subprocessStarted {
		// If our Context times out without the file being deleted, clean up.
		select {
		case <-c.Done():
			return
		default:
		}

		switch _, err := os.Stat(signalFilePath); {
		case err == nil:
			time.Sleep(10 * time.Millisecond)
		case os.IsNotExist(err):
			t.Log("Signal file has been removed.")
			subprocessStarted = true
		default:
			t.Fatalf("Error checking for signal file: %v", err)
		}
	}

	t.Log("Sending signal...")
	if err := tdc.Process.Signal(os.Interrupt); err != nil {
		t.Log("Failed to signal process")
	}

	assert.Loosely(t, tdc.Wait(t), should.BeNil)
	assert.Loosely(t, tdc.CheckOutput(t), should.BeTrue)
}

func (tc *testCase) runBypass(c context.Context, t testing.TB, td string, env environ.Env) {
	env.Set(BypassENV, BypassSentinel)
	tdc := tc.getDelegateCommand(c, td, env)

	assert.Loosely(t, tdc.Run(t), should.BeNil)
	assert.Loosely(t, tdc.CheckOutput(t), should.BeTrue)
}

func testMainRunDelegate(self, v string) int {
	var p testDelegateParams
	if err := decodeEnvironmentParam(v, &p); err != nil {
		log.Fatalf("could not decode environment param %q: %s", v, err)
	}

	argv := make([]string, 1, len(p.Args)+1)
	argv[0] = self
	argv = append(argv, p.Args...)

	c := context.Background()
	return mainImpl(c, argv, environ.System(), false)
}

func encodeEnvironmentParam(i any) string {
	d, err := json.Marshal(i)
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(d)
}

func decodeEnvironmentParam(v string, i any) error {
	d, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return err
	}
	return json.Unmarshal(d, i)
}
