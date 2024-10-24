// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package git

import (
	"context"
	"math"
	"testing"

	"infra/rts"
	"infra/rts/presubmit/eval"
	evalpb "infra/rts/presubmit/eval/proto"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestEvalStrategy(t *testing.T) {
	t.Parallel()

	ftt.Run(`apply`, t, func(t *ftt.Test) {
		ctx := context.Background()

		s := &SelectionStrategy{Graph: &Graph{}}
		s.Graph.ensureInitialized()

		applyChanges := func(changes []fileChange) {
			err := s.Graph.apply(changes, 100)
			assert.Loosely(t, err, should.BeNil)
		}

		applyChanges([]fileChange{
			{Path: "a", Status: 'A'},
		})
		applyChanges([]fileChange{
			{Path: "a", Status: 'M'},
			{Path: "b", Status: 'M'},
		})
		applyChanges([]fileChange{
			{Path: "b", Status: 'M'},
			{Path: "c/d", Status: 'A'},
		})
		applyChanges([]fileChange{
			{Path: "unreachable", Status: 'A'},
		})

		assertAffectedness := func(in eval.Input, expectedDistance float64) {
			t.Helper()
			out := &eval.Output{
				TestVariantAffectedness: make([]rts.Affectedness, 1),
			}
			err := s.SelectEval(ctx, in, out)
			assert.Loosely(t, err, should.BeNil, truth.LineContext())
			af := out.TestVariantAffectedness[0]
			if math.IsInf(expectedDistance, 1) {
				assert.Loosely(t, math.IsInf(af.Distance, 1), should.BeTrue, truth.LineContext())
			} else {
				assert.Loosely(t, af.Distance, should.AlmostEqual(
					expectedDistance, 0.00000000000001), truth.LineContext())
			}
		}

		t.Run(`a -> b`, func(t *ftt.Test) {
			in := eval.Input{
				ChangedFiles: []*evalpb.SourceFile{
					{Path: "//a"},
				},
				TestVariants: []*evalpb.TestVariant{
					{FileName: "//b"},
				},
			}
			assertAffectedness(in, -math.Log(0.5))
		})

		t.Run(`a -> unrechable`, func(t *ftt.Test) {
			in := eval.Input{
				ChangedFiles: []*evalpb.SourceFile{
					{Path: "//a"},
				},
				TestVariants: []*evalpb.TestVariant{
					{FileName: "//unreachable"},
				},
			}
			assertAffectedness(in, math.Inf(1))
		})

		t.Run(`Unknown test`, func(t *ftt.Test) {
			in := eval.Input{
				ChangedFiles: []*evalpb.SourceFile{
					{Path: "//a"},
				},
				TestVariants: []*evalpb.TestVariant{
					{FileName: "//unknown"},
				},
			}
			assertAffectedness(in, 0)
		})

		t.Run(`New test`, func(t *ftt.Test) {
			in := eval.Input{
				ChangedFiles: []*evalpb.SourceFile{
					{Path: "//new_test"},
				},
				TestVariants: []*evalpb.TestVariant{
					{FileName: "//new_test"},
				},
			}
			assertAffectedness(in, 0)
		})

		t.Run(`One of tests is unknown`, func(t *ftt.Test) {
			in := eval.Input{
				ChangedFiles: []*evalpb.SourceFile{
					{Path: "//a"},
				},
				TestVariants: []*evalpb.TestVariant{
					{FileName: "//b"},
					{FileName: "//unknown"},
				},
			}
			out := &eval.Output{
				TestVariantAffectedness: make([]rts.Affectedness, 2),
			}
			err := s.SelectEval(ctx, in, out)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, out.TestVariantAffectedness[0].Distance, should.AlmostEqual(
				-math.Log(0.5), 0.00000000000001,
			))
			assert.Loosely(t, out.TestVariantAffectedness[1].Distance, should.BeZero)
		})

		t.Run(`Test without a file name`, func(t *ftt.Test) {
			in := eval.Input{
				ChangedFiles: []*evalpb.SourceFile{
					{Path: "//a"},
				},
				TestVariants: []*evalpb.TestVariant{
					{},
				},
			}
			assertAffectedness(in, 0)
		})

		t.Run(`Unknown changed file`, func(t *ftt.Test) {
			in := eval.Input{
				ChangedFiles: []*evalpb.SourceFile{
					{Path: "//unknown"},
				},
				TestVariants: []*evalpb.TestVariant{
					{FileName: "//b"},
				},
			}
			assertAffectedness(in, math.Inf(1))
		})
	})
}
