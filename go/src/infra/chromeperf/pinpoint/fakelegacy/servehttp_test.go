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

package fakelegacy_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http/httptest"
	"sort"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/chromeperf/pinpoint"
	"infra/chromeperf/pinpoint/assertions"
	"infra/chromeperf/pinpoint/fakelegacy"
	"infra/chromeperf/pinpoint/proto"
	"infra/chromeperf/pinpoint/server"
)

// Path to the directory which contains templates for API responses.
const templateDir = "templates/"

func TestStaticUsage(t *testing.T) {
	const (
		user   = "user@example.com"
		jobID0 = "00000000000000"
		jobID1 = "11111111111111"
	)
	legacyName0 := pinpoint.LegacyJobName(jobID0)
	legacyName1 := pinpoint.LegacyJobName(jobID1)
	fake, err := fakelegacy.NewServer(
		templateDir,
		map[string]*fakelegacy.Job{
			jobID0: {
				ID:        jobID0,
				Status:    fakelegacy.CompletedStatus,
				UserEmail: user,
			},
			jobID1: {
				ID:        jobID1,
				Status:    fakelegacy.CompletedStatus,
				UserEmail: user,
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(fake.Handler())
	defer ts.Close()

	grpcPinpoint := server.New(ts.URL, ts.Client())

	ctx := context.Background()
	ftt.Run("GetJob should return known job", t, func(t *ftt.Test) {
		j, err := grpcPinpoint.GetJob(ctx, &proto.GetJobRequest{Name: legacyName0})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, j.Name, should.Equal(legacyName0))
		assert.Loosely(t, j.State, should.Equal(proto.Job_SUCCEEDED))
	})
	ftt.Run("GetJob should return NotFound for unknown job", t, func(t *ftt.Test) {
		_, err := grpcPinpoint.GetJob(ctx, &proto.GetJobRequest{Name: pinpoint.LegacyJobName("86753098675309")})
		assert.That(t, err, assertions.ShouldBeStatusError(codes.NotFound))
	})
	ftt.Run("ListJobs should return both known jobs", t, func(t *ftt.Test) {
		list, err := grpcPinpoint.ListJobs(ctx, &proto.ListJobsRequest{})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, list.Jobs, should.HaveLength(2))

		sort.Slice(list.Jobs, func(i, j int) bool {
			return list.Jobs[i].Name < list.Jobs[j].Name
		})
		assert.Loosely(t, list.Jobs[0].Name, should.Equal(legacyName0))
		assert.Loosely(t, list.Jobs[1].Name, should.Equal(legacyName1))
	})
}

func TestAddJob(t *testing.T) {
	ftt.Run("Given a fresh fakelegacy server", t, func(t *ftt.Test) {
		const userEmail = "user@example.com"

		fake, err := fakelegacy.NewServer(
			templateDir,
			map[string]*fakelegacy.Job{},
		)
		assert.Loosely(t, err, should.BeNil)
		ts := httptest.NewServer(fake.Handler())
		defer ts.Close()

		grpcPinpoint := server.New(ts.URL, ts.Client())

		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			server.EndpointsHeader: []string{
				base64.RawURLEncoding.EncodeToString(
					[]byte(fmt.Sprintf(`{"email": %q}`, userEmail)),
				),
			},
		})
		t.Run("Users can schedule a gtest benchmark", func(t *ftt.Test) {
			job, err := grpcPinpoint.ScheduleJob(ctx, &proto.ScheduleJobRequest{
				Job: &proto.JobSpec{
					Config: "some-config",
					Target: "some-target",
					Arguments: &proto.JobSpec_GtestBenchmark{
						GtestBenchmark: &proto.GTestBenchmark{
							Benchmark:   "benchmark",
							Test:        "test",
							Measurement: "measurement",
						},
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			name := job.Name

			t.Run("Users can immediately GetJob", func(t *ftt.Test) {
				job, err := grpcPinpoint.GetJob(ctx, &proto.GetJobRequest{Name: name})
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, job.Name, should.Equal(name))
				assert.Loosely(t, job.State, should.Equal(proto.Job_PENDING))
				assert.Loosely(t, job.CreatedBy, should.Equal(userEmail))
			})

			t.Run("The new job shows up in ListJobs", func(t *ftt.Test) {
				list, err := grpcPinpoint.ListJobs(ctx, &proto.ListJobsRequest{})
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, list.Jobs, should.HaveLength(1))
				assert.Loosely(t, list.Jobs[0].Name, should.Equal(name))
			})
		})
	})
}
