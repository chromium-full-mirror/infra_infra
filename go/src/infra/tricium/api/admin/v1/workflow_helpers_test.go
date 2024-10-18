// Copyright 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package admin

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"infra/tricium/api/v1"
)

func TestGetNext(t *testing.T) {
	ftt.Run("Workflow with single worker, returns no successors", t, func(t *ftt.Test) {
		cw := "Single"
		wf := &Workflow{
			Workers: []*Worker{
				{
					Name: cw,
				},
			},
		}
		sw := wf.GetNext(cw)
		assert.Loosely(t, sw, should.BeNil)
	})

	ftt.Run("Workflow with succeeding workers, returns successors", t, func(t *ftt.Test) {
		cw := "First"
		s := "Next1"
		s2 := "Next2"
		wf := &Workflow{
			Workers: []*Worker{
				{
					Name: cw,
					Next: []string{
						s,
						s2,
					},
				},
				{
					Name: s,
				},
				{
					Name: s2,
				},
			},
		}
		sw := wf.GetNext(cw)
		assert.Loosely(t, sw, should.NotBeNil)
		assert.Loosely(t, len(sw), should.Equal(2))
		assert.Loosely(t, sw[0], should.Equal(s))
		assert.Loosely(t, sw[1], should.Equal(s2))
	})
}

func TestGetWithDescendants(t *testing.T) {
	ftt.Run("Workflow with single worker, returns worker", t, func(t *ftt.Test) {
		cw := "Single"
		wf := &Workflow{
			Workers: []*Worker{
				{
					Name: cw,
				},
			},
		}
		sw := wf.GetWithDescendants(cw)
		assert.Loosely(t, len(sw), should.Equal(1))
		assert.Loosely(t, sw[0], should.Equal(cw))
	})

	ftt.Run("Workflow with descending workers, returns worker with descendants", t, func(t *ftt.Test) {
		cw := "First"
		s := "Next1"
		s2 := "Next2"
		t1 := "Next1Next1"
		t2 := "Next1Next2"
		wf := &Workflow{
			Workers: []*Worker{
				{
					Name: cw,
					Next: []string{
						s,
						s2,
					},
				},
				{
					Name: s,
					Next: []string{
						t1,
						t2,
					},
				},
				{Name: s2},
				{Name: t1},
				{Name: t2},
			},
		}
		sw := wf.GetWithDescendants(cw)
		assert.Loosely(t, sw, should.Resemble([]string{cw, s, t1, t2, s2}))
	})
}

func TestRootWorkers(t *testing.T) {
	ftt.Run("Workflow with no root workers, returns no root workers", t, func(t *ftt.Test) {
		cw := "First"
		wf := &Workflow{
			Workers: []*Worker{
				{
					Name: cw,
				},
			},
		}
		rw := wf.RootWorkers()
		assert.Loosely(t, rw, should.BeNil)
	})
	ftt.Run("Workflow with root workers, returns root workers", t, func(t *ftt.Test) {
		w := "First"
		w2 := "Second"
		w3 := "Third"
		wf := &Workflow{
			Workers: []*Worker{
				{
					Name:  w,
					Needs: tricium.Data_GIT_FILE_DETAILS,
				},
				{
					Name:  w2,
					Needs: tricium.Data_FILES,
				},
				{
					Name:  w3,
					Needs: tricium.Data_GIT_FILE_DETAILS,
				},
			},
		}
		rw := wf.RootWorkers()
		assert.Loosely(t, rw, should.NotBeNil)
		assert.Loosely(t, len(rw), should.Equal(2))
		assert.Loosely(t, rw[0], should.Equal(w))
		assert.Loosely(t, rw[1], should.Equal(w3))
	})

}
