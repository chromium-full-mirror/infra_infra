// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package backend

import (
	"testing"

	"infra/appengine/arquebus/app/config"
	monorail "infra/monorailv2/api/api_proto"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestAssignee(t *testing.T) {
	t.Parallel()
	assignerID := "test-assigner"

	ftt.Run("findAssigneeAndCCs", t, func(t *ftt.Test) {
		c := createTestContextWithTQ()

		// create sample assigner and tasks.
		assigner := createAssigner(c, t, assignerID)
		tasks := triggerScheduleTaskHandler(c, t, assignerID)
		assert.Loosely(t, tasks, should.NotBeNil)
		task := tasks[0]

		t.Run("works with UserSource_Email", func(t *ftt.Test) {
			t.Run("for assignees", func(t *ftt.Test) {
				assigner.AssigneesRaw = createRawUserSources(
					emailUserSource("oncall1@test.com"),
				)
				assigner.CCsRaw = createRawUserSources()
				assignee, ccs, err := findAssigneeAndCCs(c, assigner, task)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, assignee, should.Resemble(monorailUser("oncall1@test.com")))
				assert.Loosely(t, ccs, should.BeNil)
			})

			t.Run("for ccs", func(t *ftt.Test) {
				assigner.AssigneesRaw = createRawUserSources()
				assigner.CCsRaw = createRawUserSources(
					emailUserSource("secondary1@test.com"),
					emailUserSource("secondary2@test.com"),
				)
				assignee, ccs, err := findAssigneeAndCCs(c, assigner, task)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, assignee, should.BeNil)
				assert.Loosely(t, ccs[0], should.Resemble(monorailUser("secondary1@test.com")))
				assert.Loosely(t, ccs[1], should.Resemble(monorailUser("secondary2@test.com")))
			})
		})

		t.Run("works with UserSource_Rotation", func(t *ftt.Test) {
			t.Run("for assignees", func(t *ftt.Test) {
				assigner.AssigneesRaw = createRawUserSources(
					rotationUserSource("Rotation 1", config.Oncall_PRIMARY),
				)
				assigner.CCsRaw = createRawUserSources()
				assignee, ccs, err := findAssigneeAndCCs(c, assigner, task)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, assignee, should.Resemble(monorailUser("r1pri@example.com")))
				assert.Loosely(t, ccs, should.BeNil)
			})

			t.Run("for ccs", func(t *ftt.Test) {
				assigner.AssigneesRaw = createRawUserSources()
				assigner.CCsRaw = createRawUserSources(
					rotationUserSource("Rotation 1", config.Oncall_SECONDARY),
				)
				assignee, ccs, err := findAssigneeAndCCs(c, assigner, task)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, assignee, should.BeNil)
				assert.Loosely(t, ccs, should.HaveLength(2))
				assert.Loosely(t, ccs[0], should.Resemble(monorailUser("r1sec1@example.com")))
				assert.Loosely(t, ccs[1], should.Resemble(monorailUser("r1sec2@example.com")))
			})
		})

		t.Run("pick the first available one as the assignee", func(t *ftt.Test) {
			t.Run("with multiple UserSource_Emails", func(t *ftt.Test) {
				assigner.AssigneesRaw = createRawUserSources(
					emailUserSource("oncall1@test.com"),
					emailUserSource("oncall2@test.com"),
					emailUserSource("oncall3@test.com"),
				)
				assigner.CCsRaw = createRawUserSources()

				// UserRef with email is considered always available.
				assignee, ccs, err := findAssigneeAndCCs(c, assigner, task)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, assignee, should.Resemble(monorailUser("oncall1@test.com")))
				assert.Loosely(t, ccs, should.BeNil)
			})

			t.Run("with multiple UserSource_Rotations", func(t *ftt.Test) {
				assigner.AssigneesRaw = createRawUserSources(
					rotationUserSource("Rotation 1", config.Oncall_PRIMARY),
					rotationUserSource("Rotation 2", config.Oncall_PRIMARY),
					rotationUserSource("Rotation 3", config.Oncall_PRIMARY),
				)
				assigner.CCsRaw = createRawUserSources()
				assignee, ccs, err := findAssigneeAndCCs(c, assigner, task)
				assert.Loosely(t, err, should.BeNil)
				// it should be the primary of rotation1
				assert.Loosely(t, assignee, should.Resemble(monorailUser("r1pri@example.com")))
				assert.Loosely(t, ccs, should.BeNil)
			})

			t.Run("with a mix of available and unavailable UserSource_Rotations", func(t *ftt.Test) {
				assigner.AssigneesRaw = createRawUserSources(
					// Rotation 3 is unavailable.
					rotationUserSource("Rotation 3", config.Oncall_PRIMARY),
					rotationUserSource("Rotation 2", config.Oncall_PRIMARY),
					rotationUserSource("Rotation 1", config.Oncall_PRIMARY),
				)
				assigner.CCsRaw = createRawUserSources()
				assignee, ccs, err := findAssigneeAndCCs(c, assigner, task)
				assert.Loosely(t, err, should.BeNil)
				// it should be the primary of Rotation 2, as Rotation 3 is
				// not available.
				assert.Loosely(t, assignee, should.Resemble(monorailUser("r2pri@example.com")))
				assert.Loosely(t, ccs, should.BeNil)
			})
		})

		t.Run("CCs includes users from all the listed sources", func(t *ftt.Test) {
			assigner.AssigneesRaw = createRawUserSources()
			assigner.CCsRaw = createRawUserSources(
				rotationUserSource("Rotation 1", config.Oncall_SECONDARY),
				rotationUserSource("Rotation 2", config.Oncall_SECONDARY),
				emailUserSource("oncall1@test.com"),
			)

			assignee, ccs, err := findAssigneeAndCCs(c, assigner, task)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, assignee, should.BeNil)
			// ccs should be the secondaries of Rotation 1 and 2
			// and oncall1@test.com.
			var expected []*monorail.UserRef
			for _, user := range sampleRotationProxyRotations["Rotation 1"].Shifts[0].Oncalls[1:] {
				expected = append(expected, monorailUser(user.Email))
			}
			for _, user := range sampleRotationProxyRotations["Rotation 2"].Shifts[0].Oncalls[1:] {
				expected = append(expected, monorailUser(user.Email))
			}
			expected = append(expected, monorailUser("oncall1@test.com"))
			assert.Loosely(t, ccs, should.Resemble(expected))
		})
	})
}
