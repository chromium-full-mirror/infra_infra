// Copyright 2018 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"strings"
	"testing"

	"go.chromium.org/luci/common/flag/stringmapflag"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestFlagParse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		expect string
		input  *cmdBundle
	}{
		{`no repos`, `no repos specified`, &cmdBundle{}},
		{`empty URL`, `repo URL is blank`, &cmdBundle{
			reposInput: stringmapflag.Value{"": "value"},
		}},
	}

	ftt.Run(`Test bad flag parsing`, t, func(t *ftt.Test) {
		for _, tc := range cases {
			t.Run(tc.name, func(t *ftt.Test) {
				assert.Loosely(t, tc.input.parseFlags(), should.ErrLike(tc.expect))
			})
		}
	})
}

func TestRepoInputParsing(t *testing.T) {
	t.Parallel()

	badCases := []struct {
		name   string
		expect string
		input  map[string]string
	}{
		{`bad URL`, `invalid URL escape`, map[string]string{
			"#%%%%%": "value"}},
		{`URL with scheme`, `must not include scheme`, map[string]string{
			"https://foo.bar": "value"}},
		{`repo with .git`, `must not end with .git`, map[string]string{
			"foo.bar/repo.git": "value"}},
		{`repo with slash`, `must not end with slash`, map[string]string{
			"foo.bar/repo/": "value"}},
		{`Bad ref`, `must start with 'refs/'`, map[string]string{
			"foo.bar/repo": "value,something"}},
		{`Bad revision`, `bad revision`, map[string]string{
			"foo.bar/repo": "value,refs/something"}},
		{`Bad length`, `wrong length`, map[string]string{
			"foo.bar/repo": "f00b45"}},
	}

	ftt.Run(`Test bad parseRepoInput`, t, func(t *ftt.Test) {
		for _, tc := range badCases {
			t.Run(tc.name, func(t *ftt.Test) {
				_, err := parseRepoInput(tc.input)
				assert.Loosely(t, err, should.ErrLike(tc.expect))
			})
		}
	})

	goodCases := []struct {
		name   string
		input  map[string]string
		expect map[string]fetchSpec
	}{
		{`basic repo`,
			map[string]string{"foo.bar/repo": ""},
			map[string]fetchSpec{"foo.bar/repo": {"FETCH_HEAD", "HEAD"}}},

		{`basic repo with ref`,
			map[string]string{"foo.bar/repo": "FETCH_HEAD,refs/thingy"},
			map[string]fetchSpec{"foo.bar/repo": {"FETCH_HEAD", "refs/thingy"}}},

		{`basic repo with rev`,
			map[string]string{"foo.bar/repo": strings.Repeat("deadbeef", 5)},
			map[string]fetchSpec{"foo.bar/repo": {strings.Repeat("deadbeef", 5), "HEAD"}}},

		{`basic repo with rev+ref`,
			map[string]string{"foo.bar/repo": strings.Repeat("deadbeef", 5) + ",refs/foobar"},
			map[string]fetchSpec{"foo.bar/repo": {strings.Repeat("deadbeef", 5), "refs/foobar"}}},

		{`basic repo with FETCH_HEAD+HEAD (silly but correct)`,
			map[string]string{"foo.bar/repo": "FETCH_HEAD,HEAD"},
			map[string]fetchSpec{"foo.bar/repo": {"FETCH_HEAD", "HEAD"}}},
	}

	ftt.Run(`Test good parseRepoInput`, t, func(t *ftt.Test) {
		for _, tc := range goodCases {
			t.Run(tc.name, func(t *ftt.Test) {
				ret, err := parseRepoInput(tc.input)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, ret, should.Resemble(tc.expect))
			})
		}
	})

}
