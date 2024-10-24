// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dockerfile

import (
	"fmt"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"testing"
)

func TestResolve(t *testing.T) {
	t.Parallel()

	res := mapResolver{
		"img1:tag1":   "res1",
		"img2:latest": "lat",
	}

	call := func(in string) string {
		out, err := Resolve([]byte(in), res)
		assert.Loosely(t, err, should.BeNil)
		return string(out)
	}

	callErr := func(in string) error {
		_, err := Resolve([]byte(in), res)
		return err
	}

	ftt.Run("Pass through", t, func(t *ftt.Test) {
		same := func(in string) bool { return call(in) == in }

		assert.Loosely(t, same(``), should.BeTrue)
		assert.Loosely(t, same(`# Comment

And stuff

`), should.BeTrue)
		assert.Loosely(t, same(`
DIRECTIVE 1
FROMM 1
`), should.BeTrue)
	})

	ftt.Run("Resolves stuff", t, func(t *ftt.Test) {
		assert.Loosely(t, call(`FROM img1:tag1`), should.Equal(`FROM img1@res1`))
		assert.Loosely(t, call(`FROM img1:tag1   AS Blarg #  Zzz  zz`), should.Equal(`FROM img1@res1 AS Blarg # Zzz zz`))

		assert.Loosely(t, call(`
  FROM img1:tag1
  FROM imgZ@sha256:already_digest
  FROM scratch:wat
  FROM img2`), should.Equal(`
FROM img1@res1
FROM imgZ@sha256:already_digest
FROM scratch
FROM img2@lat`))

		// Use a previous stage as a new stage.
		// https://docs.docker.com/build/building/multi-stage/#use-a-previous-stage-as-a-new-stage
		assert.Loosely(t, call(`FROM img1:tag1 AS builder
FROM builder AS build1
FROM builder AS build2`), should.Equal(`FROM img1@res1 AS builder
FROM builder AS build1
FROM builder AS build2`))
	})

	ftt.Run("Errors", t, func(t *ftt.Test) {
		assert.Loosely(t, callErr(`from`), should.ErrLike(`line 1: expecting 'FROM <image>', got only FROM`))
		assert.Loosely(t, callErr(`from # blah`), should.ErrLike(`line 1: resolving "#:latest": no such tag`))
		assert.Loosely(t, callErr(`FROM base:${CODE_VERSION}`), should.ErrLike(`line 1: bad FROM reference "base:${CODE_VERSION}", ARGs in FROM are not supported by cloudbuildhelper`))
	})
}

// mapResolver implements Resolver via map.
type mapResolver map[string]string

func (m mapResolver) ResolveTag(image, tag string) (digest string, err error) {
	d, ok := m[fmt.Sprintf("%s:%s", image, tag)]
	if !ok {
		return "", fmt.Errorf("no such tag")
	}
	return d, nil
}
