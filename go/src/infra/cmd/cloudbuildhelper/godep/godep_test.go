// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package godep

import (
	"testing"

	"golang.org/x/mod/modfile"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

const testGoMod = `
module test/module

go 1.21
toolchain go1.21.6

require (
	direct/dep1 v1.4
	direct/dep2 v0.0.0-20210112150236-f10218a38794
	direct/dep1/deeper v1.5
	direct/unused1 v0.5
	direct/unused2 v0.6
	direct/replaced1 v1.0
	direct/replaced2 v0.0.0-20230103053340-8a57daa72e32
)

require (
	indirect/dep1 v0.3.0   // indirect
	indirect/unused v0.4.0 // indirect
)

exclude direct/unused1 v0.2

replace (
	direct/replaced1 => direct/replaced1 v1.5
	direct/replaced2 => ../local/dir
)
`

func TestDeps(t *testing.T) {
	t.Parallel()

	prepTestDeps := func() *Deps {
		deps := NewDeps(loadTestGoMod())

		assert.Loosely(t, deps.Add("direct/dep1/pkg1", "direct/dep1", "1.20"), should.BeNil)
		assert.Loosely(t, deps.Add("direct/dep1/pkg1", "direct/dep1", "1.20"), should.BeNil) // dup is fine
		assert.Loosely(t, deps.Add("direct/dep1/pkg2", "direct/dep1", "1.20"), should.BeNil)

		assert.Loosely(t, deps.Add("direct/dep2", "direct/dep2", ""), should.BeNil)
		assert.Loosely(t, deps.Add("direct/dep2/pkg", "direct/dep2", ""), should.BeNil)

		assert.Loosely(t, deps.Add("indirect/dep1/pkg1", "indirect/dep1", ""), should.BeNil)

		assert.Loosely(t, deps.Add("direct/replaced1/pkg1", "direct/replaced1", "1.19"), should.BeNil)
		assert.Loosely(t, deps.Add("direct/replaced1/pkg2", "direct/replaced1", "1.19"), should.BeNil)

		assert.Loosely(t, deps.Add("direct/replaced2/pkg1", "direct/replaced2", "1.19"), should.BeNil)
		assert.Loosely(t, deps.Add("direct/replaced2/pkg2", "direct/replaced2", "1.19"), should.BeNil)

		return deps
	}

	expectedSavedDeps := SerializedState{
		GoMod: []byte(`module test/module

go 1.21

toolchain go1.21.6

require (
	direct/dep1 v1.4.0
	direct/dep2 v0.0.0-20210112150236-f10218a38794
	direct/replaced1 v1.0.0
	direct/replaced2 v0.0.0-20230103053340-8a57daa72e32
	indirect/dep1 v0.3.0
)

replace direct/replaced1 => direct/replaced1 v1.5.0

replace direct/replaced2 => ../local/dir
`),
		ModulesTxt: []byte(`# direct/dep1 v1.4.0
## explicit; go 1.20
direct/dep1/pkg1
direct/dep1/pkg2
# direct/dep2 v0.0.0-20210112150236-f10218a38794
## explicit
direct/dep2
direct/dep2/pkg
# direct/replaced1 v1.0.0 => direct/replaced1 v1.5.0
## explicit; go 1.19
direct/replaced1/pkg1
direct/replaced1/pkg2
# direct/replaced2 v0.0.0-20230103053340-8a57daa72e32 => ../local/dir
## explicit; go 1.19
direct/replaced2/pkg1
direct/replaced2/pkg2
# indirect/dep1 v0.3.0
## explicit
indirect/dep1/pkg1
# direct/replaced1 => direct/replaced1 v1.5.0
# direct/replaced2 => ../local/dir
`),
	}

	ftt.Run("Adding and saving", t, func(t *ftt.Test) {
		deps := prepTestDeps()

		saved, err := deps.Save()
		assert.Loosely(t, err, should.BeNil)

		assert.Loosely(t, string(saved.GoMod), should.Equal(string(expectedSavedDeps.GoMod)))
		assert.Loosely(t, string(saved.ModulesTxt), should.Equal(string(expectedSavedDeps.ModulesTxt)))
	})

	ftt.Run("Loading", t, func(t *ftt.Test) {
		deps := NewDeps(loadTestGoMod())
		assert.Loosely(t, deps.Load(expectedSavedDeps), should.BeNil)

		saved, err := deps.Save()
		assert.Loosely(t, err, should.BeNil)

		assert.Loosely(t, string(saved.GoMod), should.Equal(string(expectedSavedDeps.GoMod)))
		assert.Loosely(t, string(saved.ModulesTxt), should.Equal(string(expectedSavedDeps.ModulesTxt)))
	})

	ftt.Run("AddDep errors", t, func(t *ftt.Test) {
		deps := NewDeps(loadTestGoMod())

		// Wrong module name prefix.
		assert.Loosely(t, deps.Add("direct/dep2/pkg", "direct/dep2", ""), should.BeNil)
		assert.Loosely(t, deps.Add("direct/dep2/pkg", "direct/dep1", ""), should.ErrLike("not in module"))
		assert.Loosely(t, deps.Add("direct/dep11/pkg", "direct/dep1", ""), should.ErrLike("not in module"))

		// Package "switching" modules.
		assert.Loosely(t, deps.Add("direct/dep1/deeper", "direct/dep1/deeper", ""), should.BeNil)
		assert.Loosely(t, deps.Add("direct/dep1/deeper", "direct/dep1", ""), should.ErrLike("conflicting modules"))

		// Package "switching" go version.
		assert.Loosely(t, deps.Add("indirect/dep1/pkg", "indirect/dep1", "1.20"), should.BeNil)
		assert.Loosely(t, deps.Add("indirect/dep1/pkg", "indirect/dep1", ""), should.ErrLike("conflicting go version"))

		// Missing go.mod reference.
		assert.Loosely(t, deps.Add("unknown/pkg", "unknown", ""), should.ErrLike("not present in go.mod"))
	})
}

func loadTestGoMod() *modfile.File {
	f, err := modfile.Parse("go.mod", []byte(testGoMod), nil)
	if err != nil {
		panic(err)
	}
	return f
}
