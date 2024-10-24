// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package backend

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"

	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/appengine/arquebus/app/config"
	rotationproxy "infra/appengine/rotation-proxy/proto"
	monorail "infra/monorailv2/api/api_proto"
)

func TestSearchAndUpdateIssues(t *testing.T) {
	t.Parallel()
	assignerID := "test-assigner"

	ftt.Run("searchAndUpdateIssues", t, func(t *ftt.Test) {
		c := createTestContextWithTQ()

		// create a sample assigner with tasks.
		assigner := createAssigner(c, t, assignerID)
		assigner.AssigneesRaw = createRawUserSources(
			rotationUserSource("Rotation 1", config.Oncall_PRIMARY),
		)
		assigner.CCsRaw = createRawUserSources()
		tasks := triggerScheduleTaskHandler(c, t, assignerID)
		assert.Loosely(t, tasks, should.NotBeNil)
		task := tasks[0]

		var sampleIssues []*monorail.Issue
		for i := 0; i < 20; i++ {
			sampleIssues = append(sampleIssues, &monorail.Issue{
				ProjectName: "test", LocalId: uint32(i),
			})
		}
		mockGetAndListIssues(c, sampleIssues...)

		t.Run("issues with opt-out label are filtered in search", func(t *ftt.Test) {
			countOptOptLabel := func(query string) int {
				assigner.IssueQueryRaw, _ = proto.Marshal(&config.IssueQuery{
					Q: query,
				})
				_, err := searchAndUpdateIssues(c, assigner, task)
				assert.Loosely(t, err, should.BeNil)
				req := getListIssuesRequest(c)
				assert.Loosely(t, req, should.NotBeNil)
				return strings.Count(req.Query, fmt.Sprintf("-label:%s", OptOutLabel))
			}
			assert.Loosely(t, countOptOptLabel("ABC"), should.Equal(1))
			assert.Loosely(t, countOptOptLabel("ABC OR "), should.Equal(1))
			assert.Loosely(t, countOptOptLabel("ABC OR"), should.Equal(1))
			assert.Loosely(t, countOptOptLabel("ABC DEF"), should.Equal(1))
			assert.Loosely(t, countOptOptLabel(" OR ABC"), should.Equal(1))
			assert.Loosely(t, countOptOptLabel("OR ABC DEF"), should.Equal(1))
			assert.Loosely(t, countOptOptLabel("ABC OR DEF"), should.Equal(2))
			assert.Loosely(t, countOptOptLabel("ABC OR DEF OR FOO"), should.Equal(3))
		})

		t.Run("issues are updated", func(t *ftt.Test) {
			nUpdated, err := searchAndUpdateIssues(c, assigner, task)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, nUpdated, should.Equal(len(sampleIssues)))

			for _, issue := range sampleIssues {
				req := getIssueUpdateRequest(c, issue.ProjectName, issue.LocalId)
				assert.Loosely(t, req, should.NotBeNil)
				assert.Loosely(t,
					req.Delta.OwnerRef.DisplayName, should.Equal(
						findPrimaryOncall(sampleRotationProxyRotations["Rotation 1"].Shifts[0]).DisplayName,
					))
			}
		})

		t.Run("no issues are updated", func(t *ftt.Test) {
			mockGetAndListIssues(
				c, &monorail.Issue{ProjectName: "test", LocalId: 123},
			)

			t.Run("if no oncaller is available", func(t *ftt.Test) {
				// simulate an oncall with empty shifts.
				mockRotation(c, "Rotation 1", &rotationproxy.Rotation{})

				// nUpdated should be 0
				nUpdated, err := searchAndUpdateIssues(c, assigner, task)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, nUpdated, should.BeZero)
			})

			t.Run("if no assignees and ccs set in config", func(t *ftt.Test) {
				assigner.AssigneesRaw = createRawUserSources()
				assigner.CCsRaw = createRawUserSources()

				// nUpdated should be 0
				nUpdated, err := searchAndUpdateIssues(c, assigner, task)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, nUpdated, should.BeZero)
			})

			t.Run("if no delta was found", func(t *ftt.Test) {
				issue := &monorail.Issue{
					ProjectName: "test", LocalId: 123,
					OwnerRef: nil,
					CcRefs:   []*monorail.UserRef{},
				}

				t.Run("when the owners are the same", func(t *ftt.Test) {
					assigner.AssigneesRaw = createRawUserSources(emailUserSource("foo@example.org"))
					assigner.CCsRaw = createRawUserSources()
					issue.OwnerRef = monorailUser("foo@example.org")

					// This is to ensure that searchAndUpdateIssues() determines
					// if the issue already has the intended owner by
					// UserRefs.DisplayName.
					//
					// Arquebus creates a monorail.UserRef{} with the intended
					// owner, specified in the config. However, it doesn't
					// invoke GetUser() to retrieve the UserID of the email
					// address, and, therefore, the UserId of the UserRef from
					// config is always 0, and searchAndUpdateIssues() should
					// perform the check, based on the email address, not UserID.
					issue.OwnerRef.UserId = 123
					mockGetAndListIssues(c, issue)

					nUpdated, err := searchAndUpdateIssues(c, assigner, task)
					assert.Loosely(t, err, should.BeNil)
					assert.Loosely(t, nUpdated, should.BeZero)
				})

				t.Run("when the user is already in the cc list.", func(t *ftt.Test) {
					assigner.AssigneesRaw = createRawUserSources()
					assigner.CCsRaw = createRawUserSources(emailUserSource("bar@example.net"))
					issue.CcRefs = append(issue.CcRefs, monorailUser("bar@example.net"))
					issue.CcRefs[0].UserId = 123
					mockGetAndListIssues(c, issue)

					nUpdated, err := searchAndUpdateIssues(c, assigner, task)
					assert.Loosely(t, err, should.BeNil)
					assert.Loosely(t, nUpdated, should.BeZero)
				})
			})

			t.Run("if dry-run is set", func(t *ftt.Test) {
				assigner.IsDryRun = true
				assigner.AssigneesRaw = createRawUserSources(
					emailUserSource("foo@example.org"),
				)
				assigner.CCsRaw = createRawUserSources(
					emailUserSource("bar@example.net"),
				)
				mockGetAndListIssues(
					c, &monorail.Issue{ProjectName: "test", LocalId: 123},
				)
				nUpdated, err := searchAndUpdateIssues(c, assigner, task)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, nUpdated, should.BeZero)
			})
		})

		t.Run("search response contains stale data", func(t *ftt.Test) {
			// These are to ensure that Arquebus makes a decision for issue
			// updates, based on the latest status of the issues that are found
			// in search responses.
			t.Run("the issue no longer exists", func(t *ftt.Test) {
				// mock GetIssues() without any issue objects.
				mockGetIssues(c)
				nUpdated, err := searchAndUpdateIssues(c, assigner, task)
				// NotFound should not result in searchAndUpdateIssues() failed.
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, nUpdated, should.BeZero)
			})
			t.Run("there is an owner already", func(t *ftt.Test) {
				assigner.AssigneesRaw = createRawUserSources(
					emailUserSource("foo@example.org"),
				)
				assigner.CCsRaw = createRawUserSources()
				// mock ListIssues() with an unassigned issue.
				mockListIssues(
					c, &monorail.Issue{ProjectName: "test", LocalId: 123},
				)
				// mock GetIssue() with an owner.
				mockGetIssues(
					c, &monorail.Issue{
						ProjectName: "test", LocalId: 123,
						OwnerRef: monorailUser("foo@example.org"),
					},
				)
				// Therefore, an update shouldn't be made.
				nUpdated, err := searchAndUpdateIssues(c, assigner, task)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, nUpdated, should.BeZero)
			})
		})

		t.Run("an issue update is throttled", func(t *ftt.Test) {
			clk := testclock.New(time.Unix(testclock.TestTimeUTC.Unix(), 0).UTC())
			c = clock.Set(c, clk)

			// Perform issue updates with new owners, first.
			nUpdated, err := searchAndUpdateIssues(c, assigner, task)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, nUpdated, should.Equal(len(sampleIssues)))

			// Advance the time by half of the throttle duration, and re-perform
			// issue updates. Note that samplesIssues still have no owners, and
			// therefore, reinvoking searchAndUpdateIssue() would have resulted
			// in the same number of issue updates generated. However, due to
			// the throttling duration, searchAndUpdateIssues() shouldn't
			// generate any issue updates.
			clk.Add(issueUpdateThrottleDuration / 2)
			nUpdated, err = searchAndUpdateIssues(c, assigner, task)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, nUpdated, should.BeZero)

			// Advnace the time again. Now, it's out of the throttling window,
			// and issue updates should be generated.
			clk.Add(issueUpdateThrottleDuration)
			nUpdated, err = searchAndUpdateIssues(c, assigner, task)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, nUpdated, should.Equal(len(sampleIssues)))
		})

		t.Run("the status and assignee remain the same, if there is no intended assignee", func(t *ftt.Test) {
			// Mock an issue with an assignee and status.
			si := &monorail.Issue{
				ProjectName: "test", LocalId: 123,
				StatusRef: &monorail.StatusRef{
					Status:    "Assigned",
					MeansOpen: true,
				},
				OwnerRef: monorailUser("foo@example.org"),
			}
			mockGetAndListIssues(c, si)

			t.Run("because it's outside of the oncall hours", func(t *ftt.Test) {
				// Mock a rotation with empty shifts.
				mockRotation(c, "Rotation 1", &rotationproxy.Rotation{})
				// nUpdated should be 0
				nUpdated, err := searchAndUpdateIssues(c, assigner, task)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, nUpdated, should.BeZero)
			})

			t.Run("because the config doesn't have assignee", func(t *ftt.Test) {
				// This rotation only cc-es the oncaller into the issue.
				assigner.CCsRaw = createRawUserSources(
					rotationUserSource("Rotation 1", config.Oncall_PRIMARY),
				)
				assigner.AssigneesRaw = createRawUserSources()

				// nUpdated should be 1 for the new cc-ed oncall.
				nUpdated, err := searchAndUpdateIssues(c, assigner, task)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, nUpdated, should.Equal(1))

				// The IssueDelta{} should only contain a change for CC, but
				// not for the owner and status.
				req := getIssueUpdateRequest(c, si.ProjectName, si.LocalId)
				assert.Loosely(t, req, should.NotBeNil)
				assert.Loosely(t, req.Delta.OwnerRef, should.BeNil)
				assert.Loosely(t, req.Delta.Status, should.BeNil)
				assert.Loosely(t, req.Delta.CcRefsAdd, should.NotBeNil)
			})
		})
	})
}
