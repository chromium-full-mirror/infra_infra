// Copyright 2021 The Chromium Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package render

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/chromeperf/pinpoint/proto"
)

func TestJobRenderingLegacyURL(t *testing.T) {
	ftt.Run("Given a Job proto", t, func(t *ftt.Test) {
		j := &proto.Job{
			Name:           "",
			State:          0,
			CreatedBy:      "",
			CreateTime:     timestamppb.New(time.Now().Add(-time.Hour)),
			LastUpdateTime: timestamppb.Now(),
			JobSpec: &proto.JobSpec{
				MonorailIssue: &proto.MonorailIssue{
					Project: "chromium",
					IssueId: 1234,
				},
			},
			CancellationReason: "",
			Results:            nil,
		}
		t.Run("When we have a monorail issue", func(t *ftt.Test) {
			s := renderMonorailIssue(j)
			assert.Loosely(t, s, should.Equal("https://bugs.chromium.org/p/chromium/issues/detail?id=1234"))
		})
		t.Run("When we have a legacy ID", func(t *ftt.Test) {
			j.Name = "jobs/legacy-1234567"
			t.Run("Then we can generate a URL for the job", func(t *ftt.Test) {
				u, err := legacyJobURL(j)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, u, should.Equal("https://pinpoint-dot-chromeperf.appspot.com/job/1234567"))
			})
			t.Run("Then we can generate a ID for the job", func(t *ftt.Test) {
				u, err := JobID(j)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, u, should.Equal("1234567"))
			})
		})
		t.Run("When the legacy service does not have a trailing /", func(t *ftt.Test) {
			j.Name = "jobs/legacy-1234"
			t.Run("Then we can generate a valid URL for the job", func(t *ftt.Test) {
				u, err := legacyJobURL(j)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, u, should.Equal("https://pinpoint-dot-chromeperf.appspot.com/job/1234"))
			})
			t.Run("Then we can generate a valid ID for the job", func(t *ftt.Test) {
				u, err := JobID(j)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, u, should.Equal("1234"))
			})
		})
		t.Run("When we have a non-legacy ID", func(t *ftt.Test) {
			uID, err := uuid.NewRandom()
			assert.Loosely(t, err, should.BeNil)
			j.Name = uID.String()
			assert.Loosely(t, j.Name, should.NotEqual(""))
			t.Run("Then we cannot generate a valid URL for the job", func(t *ftt.Test) {
				u, err := legacyJobURL(j)
				assert.Loosely(t, u, should.BeEmpty)
				assert.Loosely(t, err, should.NotBeNil)
			})
		})
	})
}
