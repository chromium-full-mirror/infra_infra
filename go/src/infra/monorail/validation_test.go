// Copyright 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package monorail

import (
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"testing"
)

func TestValidation(t *testing.T) {
	t.Parallel()

	ftt.Run("Validation", t, func(t *ftt.Test) {
		type Validatable interface {
			Validate() error
		}
		good := func(v Validatable) {
			assert.Loosely(t, v.Validate(), should.BeNil)
		}
		bad := func(v Validatable) {
			assert.Loosely(t, v.Validate(), should.NotBeNil)
		}

		t.Run("IssueRef", func(t *ftt.Test) {
			good(&IssueRef{ProjectId: "chromium", IssueId: 1})
			bad((*IssueRef)(nil))
			bad(&IssueRef{})
			bad(&IssueRef{ProjectId: "chromium"})
			bad(&IssueRef{IssueId: 1})
		})

		t.Run("AtomPerson", func(t *ftt.Test) {
			good(&AtomPerson{Name: "bob"})
			bad((*AtomPerson)(nil))
			bad(&AtomPerson{})
		})

		t.Run("Issue", func(t *ftt.Test) {
			good(&Issue{Status: StatusStarted, ProjectId: "chromium"})
			good(&Issue{
				Summary:     "Write tests for monorail client",
				Author:      &AtomPerson{Name: "seanmccullough@chromium.org"},
				Owner:       &AtomPerson{Name: "nodir@chromium.org"},
				Status:      StatusStarted,
				Cc:          []*AtomPerson{{Name: "agable@chromium.org"}},
				Description: "We should keep our code coverage high, so write tests",
				Components:  []string{"Infra"},
				Labels:      []string{"M-53"},
				ProjectId:   "chromium",
			})

			bad((*Issue)(nil))
			bad(&Issue{})
			bad(&Issue{
				Status:    StatusStarted,
				BlockedOn: []*IssueRef{{}},
			})
			bad(&Issue{
				Status: StatusStarted,
				Cc:     []*AtomPerson{{Name: "a"}, {Name: "a"}},
			})
			bad(&Issue{
				Status:     StatusStarted,
				Components: []string{""},
			})
			bad(&Issue{
				Status: StatusStarted,
				Labels: []string{""},
			})
			bad(&Issue{
				Status: StatusStarted,
				Owner:  &AtomPerson{},
			})
		})
		t.Run("InsertIssueRequest", func(t *ftt.Test) {
			good(&InsertIssueRequest{
				Issue: &Issue{
					ProjectId:   "chromium",
					Summary:     "Write tests for monorail client",
					Author:      &AtomPerson{Name: "seanmccullough@chromium.org"},
					Owner:       &AtomPerson{Name: "nodir@chromium.org"},
					Status:      StatusStarted,
					Cc:          []*AtomPerson{{Name: "agable@chromium.org"}},
					Description: "We should keep our code coverage high, so write tests",
					Components:  []string{"Infra"},
					Labels:      []string{"M-53"},
				},
			})

			bad(&InsertIssueRequest{})
			bad(&InsertIssueRequest{
				Issue: &Issue{
					ProjectId: "chromium",
					Status:    StatusStarted,
					Id:        1,
				},
			})
		})
	})
}
