// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package module

import (
	"fmt"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"testing"
)

func TestModule(t *testing.T) {
	t.Parallel()

	ftt.Run("Extracts name", t, func(t *ftt.Test) {
		m, err := parseYAML([]byte(`service: zzz`))
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, m.Name, should.Equal("zzz"))

		m, err = parseYAML([]byte(`module: zzz`))
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, m.Name, should.Equal("zzz"))

		m, err = parseYAML([]byte(`stuff: blah`))
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, m.Name, should.Equal("default"))

		m, err = parseYAML([]byte(`module: 123`))
		assert.Loosely(t, err, should.ErrLike("not a string"))
	})

	ftt.Run("Process drops unsupported fields", t, func(t *ftt.Test) {
		m, err := parseYAML([]byte(`{
			"application": "app",
			"version": "ver",
			"module": "zzz",
			"luci_gae_vars": {"app-id": {"ZZZ": 123}}
		}`))
		assert.Loosely(t, err, should.BeNil)
		_, err = m.Process("app-id", nil)
		assert.Loosely(t, err, should.BeNil)

		blob, err := m.DumpYAML()
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, string(blob), should.Equal("service: zzz\n"))
	})

	ftt.Run("Process rejects bad luci_gae_vars section", t, func(t *ftt.Test) {
		cases := []string{
			`luci_gae_vars: 123`,
			`luci_gae_vars: {123: {}}`,
			`luci_gae_vars: {"zzz": 123}`,
			`luci_gae_vars: {"zzz": {123: "zzz"}}`,
		}
		for _, body := range cases {
			body := body
			t.Run(fmt.Sprintf("Case %q", body), func(t *ftt.Test) {
				m, err := parseYAML([]byte(body))
				assert.Loosely(t, err, should.BeNil)
				_, err = m.Process("app-id", nil)
				assert.Loosely(t, err, should.ErrLike("should"))
			})
		}
	})

	ftt.Run("Render vars", t, func(t *ftt.Test) {
		t.Run("Unsupported type in decl", func(t *ftt.Test) {
			decl := varsDecl{
				"app-id": {"VAR": 123.123},
			}
			_, _, err := renderVars(nil, "app-id", decl, nil)
			assert.Loosely(t, err, should.ErrLike(`variable "VAR" has unsupported type float64`))
		})

		t.Run("Inconsistent type in decl", func(t *ftt.Test) {
			decl := varsDecl{
				"app-id-1": {"VAR": "blah-1"},
				"app-id-2": {"VAR": 123},
			}
			_, _, err := renderVars(nil, "app-id", decl, nil)
			assert.Loosely(t, err, should.ErrLike(`variable "VAR" has ambiguous type`))
		})

		t.Run("Bad int var", func(t *ftt.Test) {
			decl := varsDecl{
				"app-id": {"VAR": 0},
			}
			_, _, err := renderVars(nil, "app-id", decl, map[string]string{"VAR": "haha"})
			assert.Loosely(t, err, should.ErrLike(`the value of variable "VAR" is expected to be an integer, got "haha"`))
		})

		t.Run("Bad bool var", func(t *ftt.Test) {
			decl := varsDecl{
				"app-id": {"VAR": false},
			}
			_, _, err := renderVars(nil, "app-id", decl, map[string]string{"VAR": "haha"})
			assert.Loosely(t, err, should.ErrLike(`the value of variable "VAR" is expected to be a boolean, got "haha"`))
		})

		t.Run("Substitutes", func(t *ftt.Test) {
			decl := varsDecl{
				"app-id": {
					"STR_VAR":  "blah-1",
					"INT_VAR":  123,
					"BOOL_VAR": false,
					"UNUSED":   "!!!",
				},
				"another-app-id": {
					"STR_VAR":  "blah-2",
					"INT_VAR":  456,
					"BOOL_VAR": true,
				},
			}
			v := map[string]interface{}{
				"str_key1": "blah ${STR_VAR}",
				"str_key2": "blah ${INT_VAR}",
				"str_key3": "blah ${BOOL_VAR}",
				"int_var":  "${INT_VAR}",
				"str_var":  "${STR_VAR}",
				"bool_var": "${BOOL_VAR}",
				"a bunch":  "${ANOTHER_VAR} ${STR_VAR}",
			}
			out, consumed, err := renderVars(v, "app-id", decl, map[string]string{
				"INT_VAR":     "42",   // will be converted to int
				"BOOL_VAR":    "TRUE", // will be converted to bool
				"ANOTHER_VAR": "zzz",
				"UNUSED_TOO":  "!!!",
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, out, should.Resemble(map[string]interface{}{
				"str_key1": "blah blah-1",
				"str_key2": "blah 42",
				"str_key3": "blah true",
				"int_var":  42, // yay, int
				"str_var":  "blah-1",
				"bool_var": true,
				"a bunch":  "zzz blah-1",
			}))
			assert.Loosely(t, consumed.ToSortedSlice(), should.Resemble([]string{
				"ANOTHER_VAR",
				"BOOL_VAR",
				"INT_VAR",
				"STR_VAR",
			}))
		})

		t.Run("Substitutes without declaration", func(t *ftt.Test) {
			v := map[string]interface{}{
				"key1": "blah ${VAR1}",
				"key2": "${VAR2}",
				"key3": "zzz ${UNDEFINED}",
			}
			out, consumed, err := renderVars(v, "app-id", nil, map[string]string{
				"VAR1": "zzz",
				"VAR2": "42",
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, out, should.Resemble(map[string]interface{}{
				"key1": "blah zzz",
				"key2": "42",               // undeclared variables are assumed to be strings
				"key3": "zzz ${UNDEFINED}", // totally ignores undefined variables
			}))
			assert.Loosely(t, consumed.ToSortedSlice(), should.Resemble([]string{
				"UNDEFINED",
				"VAR1",
				"VAR2",
			}))
		})

		t.Run("Recurses", func(t *ftt.Test) {
			v := map[string]interface{}{
				"top": "${VAR}",
				"dict": map[interface{}]interface{}{
					"deeper": map[interface{}]interface{}{
						"key": "${VAR}",
					},
					123: "huh",
				},
				"list": []interface{}{
					"${VAR}",
					[]interface{}{"${VAR}"},
				},
				"null": nil,
			}
			out, _, err := renderVars(v, "app-id", nil, map[string]string{"VAR": "zzz"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, out, should.Resemble(map[string]interface{}{
				"top": "zzz",
				"dict": map[interface{}]interface{}{
					"deeper": map[interface{}]interface{}{
						"key": "zzz",
					},
					123: "huh",
				},
				"list": []interface{}{
					"zzz",
					[]interface{}{"zzz"},
				},
				"null": nil,
			}))
		})

		t.Run("Undefined key in strict mode", func(t *ftt.Test) {
			decl := varsDecl{
				"app-id": {
					"VAR": "blah",
				},
			}
			v := map[string]interface{}{
				"top": "${VAR} ${ANOTHER} ${THIRD}",
			}
			_, _, err := renderVars(v, "app-id", decl, nil)
			assert.Loosely(t, err, should.ErrLike(`a value for variable "ANOTHER" is not provided`))
		})
	})
}
