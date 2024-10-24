// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"testing"

	"go.chromium.org/luci/common/system/environ"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/tools/git/state"
)

func TestMain(t *testing.T) {
	t.Parallel()

	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to get self executable: %s", err)
	}

	ftt.Run(`Using a test setup`, t, func(t *ftt.Test) {
		c := context.Background()

		env := environ.New(nil)
		runMain := func(c context.Context, args ...string) int {
			args = append([]string{executable}, args...)
			return mainImpl(c, args, env, bytes.NewReader(nil), ioutil.Discard, ioutil.Discard)
		}

		t.Run(`When run in check mode, returns "1".`, func(t *ftt.Test) {
			env.Set(gitWrapperCheckENV, executable)
			assert.Loosely(t, runMain(c), should.Equal(1))
		})

		t.Run(`Can run local Git (must be in PATH)`, func(t *ftt.Test) {
			env = environ.System()
			systemGit, err := gitProbe.Locate(c, "", env)

			if err != nil {
				t.Skipf("Cannot find system Git; skipping Git test: %s", err)
			}

			t.Run(`With system Git`, func(t *ftt.Test) {
				t.Run(`"git version" returns 0.`, func(t *ftt.Test) {
					assert.Loosely(t, runMain(c, "version"), should.BeZero)
				})

				t.Run(`"git --clearly-an-invalid-flag" returns 129 (-1).`, func(t *ftt.Test) {
					assert.Loosely(t, runMain(c, "--clearly-an-invalid-flag"), should.Equal(129))
				})

				t.Run(`Returns wrapper error code if we can't find Git.`, func(t *ftt.Test) {
					env.Set("PATH", "")
					assert.Loosely(t, runMain(c, "version"), should.Equal(gitWrapperErrorReturnCode))
				})

				t.Run(`Can use the cached Git path, if configured.`, func(t *ftt.Test) {
					st := state.State{
						SelfPath: executable,
						GitPath:  systemGit,
					}
					env.Set(gitWrapperENV, st.ToENV())

					assert.Loosely(t, runMain(c, "version"), should.BeZero)
				})

				t.Run(`Will ignore the cached Git path, if "self" is invalid.`, func(t *ftt.Test) {
					st := state.State{
						SelfPath: "** DOES NOT EXIST **",
						GitPath:  systemGit,
					}
					env.Set(gitWrapperENV, st.ToENV())

					assert.Loosely(t, runMain(c, "version"), should.BeZero)
				})
			})
		})
	})
}
