// Copyright 2017 The LUCI Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package venv

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/common/sync/parallel"
	"go.chromium.org/luci/common/system/environ"
	"go.chromium.org/luci/common/system/filesystem"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/convey"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/tools/vpython/legacy/vpython2.7/luci/api/vpython"
	"go.chromium.org/infra/tools/vpython/legacy/vpython2.7/luci/python"
)

var verbose = flag.Bool("test.gologger", false, "Enable Go logging.")

func testContext() context.Context {
	c := context.Background()
	if *verbose {
		c = gologger.StdConfig.Use(c)
		c = logging.SetLevel(c, logging.Debug)
	}
	return c
}

type resolvedInterpreter struct {
	py      *python.Interpreter
	version python.Version
}

func resolveFromPath(vers python.Version) *resolvedInterpreter {
	c := testContext()
	py, err := python.Find(c, vers, nil)
	if err != nil {
		return nil
	}
	if err := filesystem.AbsPath(&py.Python); err != nil {
		panic(err)
	}

	ri := resolvedInterpreter{
		py: py,
	}
	if ri.version, err = ri.py.GetVersion(c); err != nil {
		panic(err)
	}
	return &ri
}

var (
	pythonGeneric = resolveFromPath(python.Version{})
	python27      = resolveFromPath(python.Version{Major: 2, Minor: 7, Patch: 0})
	python3       = resolveFromPath(python.Version{Major: 3, Minor: 0, Patch: 0})
)

func TestResolvePythonInterpreter(t *testing.T) {
	t.Parallel()

	// This test is run for each resolved Python interpreter found on the host
	// system.
	testPythonInterpreter := func(t *ftt.Test, ctx context.Context, ri *resolvedInterpreter, vers string) {
		cfg := Config{}
		s := vpython.Spec{
			PythonVersion: vers,
		}

		t.Run(`Can resolve interpreter version`, func(t *ftt.Test) {
			assert.Loosely(t, cfg.resolvePythonInterpreter(ctx, &s), should.BeNil)
			assert.Loosely(t, cfg.si.Python, should.Equal(ri.py.Python))

			vers, err := python.ParseVersion(s.PythonVersion)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, vers.IsSatisfiedBy(ri.version), should.BeTrue)
		})

		t.Run(`Can resolve Runtime information`, func(t *ftt.Test) {
			r, err := ri.py.GetRuntime(ctx)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, r.Path, should.NotEqual(""))
			assert.Loosely(t, r.Prefix, should.NotEqual(""))
			assert.Loosely(t, r.Hash, should.NotEqual(""))
		})

		t.Run(fmt.Sprintf(`Fails when Python 9999 is requested, but a Python %s interpreter is forced.`, vers), func(t *ftt.Test) {
			cfg.UnversionedPython = []string{ri.py.Python}
			s.PythonVersion = "9999"
			assert.Loosely(t, cfg.resolvePythonInterpreter(ctx, &s), should.ErrLike("none of ["))
			assert.Loosely(t, cfg.resolvePythonInterpreter(ctx, &s), should.ErrLike("] matched specification"))
		})
	}

	ftt.Run(`Resolving a Python interpreter`, t, func(t *ftt.Test) {
		ctx := testContext()

		// Tests to run if we have Python 2.7 installed.
		if python27 != nil {
			t.Run(`When Python 2.7 is requested`, func(c *ftt.Test) {
				testPythonInterpreter(c, ctx, python27, "2.7")
			})
		}

		// Tests to run if we have Python 2.7 and a generic Python installed.
		if pythonGeneric != nil && python27 != nil {
			// Our generic Python resolves to a known version, so we can proceed.
			t.Run(`When no Python version is specified, spec resolves to generic.`, func(t *ftt.Test) {
				cfg := Config{}
				s := vpython.Spec{}
				assert.Loosely(t, cfg.resolvePythonInterpreter(ctx, &s), should.BeNil)
				assert.Loosely(t, cfg.si.Python, should.Equal(pythonGeneric.py.Python))

				vers, err := python.ParseVersion(s.PythonVersion)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, vers.IsSatisfiedBy(pythonGeneric.version), should.BeTrue)
			})
		}

		// Tests to run if we have Python 3 installed.
		if python3 != nil {
			t.Run(`When Python 3 is requested`, func(t *ftt.Test) {
				testPythonInterpreter(t, ctx, python3, "3")
			})
		}
	})

	ftt.Run(`Resolving interpreter from multiple options`, t, func(t *ftt.Test) {
		ctx := testContext()

		pythons := []*resolvedInterpreter{}
		tryAddPython := func(majorVersion, minorVersion int) {
			ri := resolveFromPath(python.Version{Major: majorVersion, Minor: minorVersion, Patch: 0})

			// Ignore interpreters without '.' in the filename. Such binaries
			// are usually just symlinks to binaries named with the specific
			// minor version (e.g. "python3.9"), and if we don't ignore them we
			// can end up with duplicate entries.
			if ri != nil && strings.Contains(ri.py.Python, ".") {
				pythons = append(pythons, ri)
			}
		}

		tryAddPython(2, 7)
		for v := 5; v < 20; v += 1 {
			tryAddPython(3, v)
		}

		if len(pythons) < 2 {
			// These tests aren't meaningful if we don't have at least two
			// Python interpreters to test with.
			return
		}

		t.Run(`First interpreter in slice is selected by default`, func(t *ftt.Test) {
			for i := range pythons {
				cfgPythons := SliceFlag{pythons[i].py.Python}
				for j := range pythons {
					if i != j {
						cfgPythons = append(cfgPythons, pythons[j].py.Python)
					}
				}

				cfg := Config{UnversionedPython: cfgPythons}

				// No Python version specified in the spec, so we should default
				// to the first interpreter from the config.
				s := vpython.Spec{}
				assert.Loosely(t, cfg.resolvePythonInterpreter(ctx, &s), should.BeNil)
				assert.Loosely(t, cfg.si.Python, should.Equal(cfgPythons[0]))
			}
		})

		t.Run(`Spec selects matching Python interpreter from config`, func(t *ftt.Test) {
			cfgPythons := SliceFlag{}
			for _, ri := range pythons {
				cfgPythons = append(cfgPythons, ri.py.Python)
			}

			for _, ri := range pythons {
				cfg := Config{UnversionedPython: cfgPythons}

				// Python version specified in the spec, so we should select
				// the matching interpreter from the config.
				s := vpython.Spec{
					PythonVersion: fmt.Sprintf("%d.%d", ri.version.Major, ri.version.Minor),
				}
				assert.Loosely(t, cfg.resolvePythonInterpreter(ctx, &s), should.BeNil)
				assert.Loosely(t, cfg.si.Python, should.Equal(ri.py.Python))
			}
		})
	})
}

type setupCheckManifest struct {
	Interpreter string `json:"interpreter"`
	Pants       string `json:"pants"`
	Shirt       string `json:"shirt"`
}

func testVirtualEnvWith(t *testing.T, ri *resolvedInterpreter) {
	t.Parallel()

	if ri == nil {
		t.Skipf("No python interpreter found.")
	}

	tl, err := loadTestEnvironment(testContext(), t)
	if err != nil {
		t.Fatalf("could not set up test loader for %q: %s", ri.py.Python, err)
	}

	ftt.Run(`Testing the VirtualEnv`, t, func(t *ftt.Test) {
		tdir := t.TempDir()
		defer func() {
			assert.Loosely(t, filesystem.RemoveAll(tdir), should.BeNil)
		}()
		c := testContext()

		// Load the bootstrap wheels for the next part of the test.
		assert.Loosely(t, tl.ensureWheels(c, t, ri.py, tdir), should.BeNil)

		config := Config{
			BaseDir:    tdir,
			MaxHashLen: 4,
			SetupEnv:   environ.System(),
			Package: vpython.Spec_Package{
				Name:    "foo/bar/virtualenv",
				Version: "unresolved",
			},
			UnversionedPython: []string{ri.py.Python},
			Loader:            tl,
			Spec: &vpython.Spec{
				Wheel: []*vpython.Spec_Package{
					{Name: "foo/bar/shirt", Version: "unresolved"},
					{Name: "foo/bar/pants", Version: "unresolved"},
				},
			},
		}

		t.Run(`Testing Setup`, func(t *ftt.Test) {
			config.FailIfLocked = true
			err := With(c, config, func(c context.Context, v *Env) error {
				testScriptTarget := python.ScriptTarget{
					Path: filepath.Join(testDataDir, "setup_check.py"),
				}
				checkOut := filepath.Join(tdir, "output.json")
				cmd := v.Interpreter().MkIsolatedCommand(c, testScriptTarget, "--json-output", checkOut)
				defer cmd.Cleanup()
				cmd.Dir = "" // we want cwd
				assert.Loosely(t, cmd.Run(), should.BeNil)

				var m setupCheckManifest
				assert.Loosely(t, loadJSON(checkOut, &m), should.BeNil)
				assert.Loosely(t, m.Interpreter, should.HavePrefix(v.Root))
				assert.Loosely(t, m.Pants, should.HavePrefix(v.Root))
				assert.Loosely(t, m.Shirt, should.HavePrefix(v.Root))
				assert.Loosely(t, v.Environment, should.NotBeNil)

				// We should be able to load its environment stamp.
				v.Environment = nil
				assert.Loosely(t, v.AssertCompleteAndLoad(), should.BeNil)
				assert.Loosely(t, v.Environment, should.NotBeNil)
				assert.Loosely(t, len(v.Environment.Pep425Tag), should.BeGreaterThan(0))
				assert.Loosely(t, v.Environment.Spec, should.NotBeNil)
				assert.Loosely(t, len(v.Environment.Spec.Wheel), should.Equal(len(config.Spec.Wheel)))
				assert.Loosely(t, v.Environment.Spec.Virtualenv, should.NotBeNil)
				assert.Loosely(t, v.Environment.Spec.PythonVersion, should.NotEqual(""))

				return nil
			})
			assert.Loosely(t, err, should.BeNil)

			// We should be able to delete it.
			v, err := config.makeEnv(c, nil)
			assert.Loosely(t, err, should.BeNil)

			assert.Loosely(t, v.Delete(c), should.BeNil)
			assert.Loosely(t, v.Root, convey.Adapt(shouldNotExist)())
			assert.Loosely(t, v.lockPath, convey.Adapt(shouldNotExist)())
		})

		t.Run(`Testing new environment setup race`, func(t *ftt.Test) {
			const workers = 4

			envs := make([]*vpython.Environment, workers)
			err := parallel.FanOutIn(func(taskC chan<- func() error) {
				for i := range workers {
					taskC <- func() error {
						return With(c, config, func(c context.Context, v *Env) error {
							// Has successfully loaded an Environment.
							envs[i] = v.Environment

							// Can use the Python interpreter.
							if _, err := v.Interpreter().GetVersion(c); err != nil {
								return err
							}
							return nil
						})
					}
				}
			})
			assert.Loosely(t, err, should.BeNil)

			// All Environments must be equal.
			var archetype *vpython.Environment
			for _, env := range envs {
				if archetype == nil {
					archetype = env
				} else {
					assert.Loosely(t, env, should.Resemble(archetype))
				}
			}
		})
	})
}

func TestVirtualEnv(t *testing.T) {
	// TODO(crbug/1060092): fix and re-enable the test.
	t.Skip("flaky, see https://crbug.com/1060092")
	t.Parallel()

	for _, tc := range []struct {
		name string
		ri   *resolvedInterpreter
	}{
		{"python27", python27},
		{"python3", python3},
	} {
		t.Run(fmt.Sprintf(`Testing Virtualenv for: %s`, tc.name), func(t *testing.T) {
			testVirtualEnvWith(t, tc.ri)
		})
	}
}

func loadJSON(path string, dst interface{}) error {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return errors.Annotate(err, "failed to open file").Err()
	}
	if err := json.Unmarshal(content, dst); err != nil {
		return errors.Annotate(err, "failed to unmarshal JSON").Err()
	}
	return nil
}

func shouldNotExist(actual interface{}, expected ...interface{}) string {
	path := actual.(string)
	switch _, err := os.Stat(path); {
	case err == nil:
		return fmt.Sprintf("Path %q should not exist, but it does.", path)
	case os.IsNotExist(err):
		return ""
	default:
		return fmt.Sprintf("Couldn't check if %q exists: %s", path, err)
	}
}
