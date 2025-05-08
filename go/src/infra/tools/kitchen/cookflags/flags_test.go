// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cookflags

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

var flagTestCases = []struct {
	flags       []string
	cf          CookFlags
	errParse    any
	errValidate any
}{
	{
		flags:       []string{},
		errValidate: "-checkout-dir doesn't exist",
	},

	{
		flags: []string{
			"-checkout-dir", ".",
			"-recipe", "yep",
			"-properties", `{"some": "thing"}`, "-properties-file", "bar",
		},
		errValidate: "only one of -properties or -properties-file",
	},

	{
		flags: []string{
			"-checkout-dir", ".",
			"-recipe", "cool_recipe",
			"-call-update-build",
		},
		errValidate: `-call-update-build requires -buildbucket-hostname`,
	},
	{
		flags: []string{
			"-checkout-dir", ".",
			"-recipe", "cool_recipe",
			"-buildbucket-hostname", "buildbucket.example.com",
			"-call-update-build",
		},
		errValidate: `-call-update-build requires a valid -buildbucket-build-id`,
	},
}

func TestFlags(t *testing.T) {
	t.Parallel()

	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	r := strings.NewReplacer(
		cwd, "$CWD",
		"\\", "/",
	)

	ftt.Run("Flags", t, func(t *ftt.Test) {
		cf := CookFlags{}
		fs := flag.NewFlagSet("test_flags", flag.ContinueOnError)
		fs.Usage = func() {}

		t.Run("can register them", func(t *ftt.Test) {
			cf.Register(fs)

			t.Run("and parse some flags", func(t *ftt.Test) {
				for _, tc := range flagTestCases {
					t.Run(fmt.Sprintf("%v", tc.flags), func(t *ftt.Test) {
						assert.Loosely(t, fs.Parse(tc.flags), should.ErrLike(tc.errParse))
						if tc.errParse == nil {
							if tc.errValidate == nil {
								assert.Loosely(t, cf.Dump(), should.Match(tc.flags))
								data, err := json.Marshal(cf)
								assert.Loosely(t, err, should.BeNil)
								cf2 := &CookFlags{}
								assert.Loosely(t, json.Unmarshal(data, cf2), should.BeNil)
								assert.Loosely(t, &cf, should.Match(cf2))
							}
							assert.Loosely(t, cf.Normalize(), should.ErrLike(tc.errValidate))
							if tc.errValidate == nil {
								cf.TempDir = r.Replace(cf.TempDir)
								assert.Loosely(t, cf, should.Match(tc.cf))
							}
						}
					})
				}
			})
		})
	})
}
