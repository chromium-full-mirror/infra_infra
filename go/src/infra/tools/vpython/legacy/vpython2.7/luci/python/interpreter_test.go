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

package python

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"go.chromium.org/luci/common/system/environ"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

const testGetVersionENV = "_VPYTHON_TEST_GET_VERSION"

// Setup for TestInterpreterGetVersion.
func init() {
	if v, ok := os.LookupEnv(testGetVersionENV); ok {
		rc := 0
		if _, err := os.Stdout.WriteString(v); err != nil {
			rc = 1
		}
		os.Exit(rc)
	}
}

func TestInterpreter(t *testing.T) {
	t.Parallel()

	self, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to get executable name: %s", err)
	}

	ftt.Run(`A Python interpreter`, t, func(t *ftt.Test) {
		c := context.Background()

		i := Interpreter{
			Python: self,
		}

		t.Run(`Testing IsolatedCommand`, func(t *ftt.Test) {
			cmd := i.MkIsolatedCommand(c, NoTarget{}, "foo", "bar")
			defer cmd.Cleanup()
			assert.Loosely(t, cmd.Args, should.Resemble([]string{self, "-B", "-E", "-s", "foo", "bar"}))
		})

	})
}

func TestInterpreterGetVersion(t *testing.T) {
	t.Parallel()

	// For this test, we use our test binary as the executable and install an
	// "init" hook to get it to dump its version.
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to get executable: %s", err)
	}
	versionString := ""

	versionSuccesses := []struct {
		output string
		vers   Version
	}{
		{"2.7.1", Version{2, 7, 1}},
		{"2.7.1+local string", Version{2, 7, 1}},
		{"3", Version{3, 0, 0}},
		{"3.1.2.3.4", Version{3, 1, 2}},
	}

	versionFailures := []struct {
		output string
		err    string
	}{
		{"", "unknown version output"},
		{"wat", "non-canonical Python version string: \"wat\""},
	}

	ftt.Run(`Testing Interpreter.GetVersion`, t, func(t *ftt.Test) {
		c := context.Background()

		i := Interpreter{
			Python: self,

			fileCacheDisabled: true,
			testCommandHook: func(cmd *exec.Cmd) {
				env := environ.New(nil)
				env.Set(testGetVersionENV, versionString)
				cmd.Env = env.Sorted()
			},
		}

		for _, tc := range versionSuccesses {
			t.Run(fmt.Sprintf(`Can successfully parse %q => %s`, tc.output, tc.vers), func(t *ftt.Test) {
				versionString = tc.output

				vers, err := i.GetVersion(c)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, vers, should.Resemble(tc.vers))

				cachedVers, err := i.GetVersion(c)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, cachedVers, should.Resemble(vers))
			})
		}

		for _, tc := range versionFailures {
			t.Run(fmt.Sprintf(`Will fail to parse %q (%s)`, tc.output, tc.err), func(t *ftt.Test) {
				versionString = tc.output

				_, err := i.GetVersion(c)
				assert.Loosely(t, err, should.ErrLike(tc.err))
			})
		}
	})
}

func TestIsolateEnvironment(t *testing.T) {
	t.Parallel()

	ftt.Run(`Testing IsolateEnvironment`, t, func(t *ftt.Test) {
		t.Run(`Will return nil if the environment is nil.`, func(t *ftt.Test) {
			assert.Loosely(t, func() { IsolateEnvironment(nil, false) }, should.NotPanic)
		})

		t.Run(`Will remove environment variables`, func(t *ftt.Test) {
			env := environ.New([]string{"FOO=", "BAR=", "PYTHONPATH=a:b:c", "PYTHONHOME=/foo/bar", "PYTHONNOUSERSITE=0"})

			t.Run(`When keepPythonPath is false`, func(t *ftt.Test) {
				IsolateEnvironment(&env, false)
				assert.Loosely(t, env.Sorted(), should.Resemble([]string{"BAR=", "FOO=", "PYTHONNOUSERSITE=1"}))
			})

			t.Run(`When keepPythonPath is true`, func(t *ftt.Test) {
				IsolateEnvironment(&env, true)
				assert.Loosely(t, env.Sorted(), should.Resemble([]string{"BAR=", "FOO=", "PYTHONNOUSERSITE=1", "PYTHONPATH=a:b:c"}))
			})
		})
	})
}
