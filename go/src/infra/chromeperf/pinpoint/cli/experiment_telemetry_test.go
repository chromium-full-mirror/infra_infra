// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cli

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	grpc "google.golang.org/grpc"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/chromeperf/pinpoint/proto"
)

const testPriority = 42
const testInitialAttemptCount int32 = 22

type startedJob struct {
	Benchmark           string
	Configuration       string
	InitialAttemptCount int32
	Priority            int32
	Story               string
	StoryTags           []string
}

type fakePinpointClient struct {
	Jobs []startedJob
	mu   sync.Mutex
}

func less(x, y startedJob) bool {
	if x.Benchmark != y.Benchmark {
		return x.Benchmark < y.Benchmark
	} else if x.Configuration != y.Configuration {
		return x.Configuration < y.Configuration
	} else if x.Story != y.Story {
		return x.Story < y.Story
	} else if len(x.StoryTags) != len(y.StoryTags) {
		return len(x.StoryTags) < len(y.StoryTags)
	} else {
		for i := range x.StoryTags {
			if x.StoryTags[i] != y.StoryTags[i] {
				return x.StoryTags[i] < y.StoryTags[i]
			}
		}
	}
	return false
}

func (c *fakePinpointClient) ScheduleJob(ctx context.Context, in *proto.ScheduleJobRequest, opts ...grpc.CallOption) (*proto.Job, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := new(proto.Job)
	out.Name = "jobs/legacy-4242"
	started_job := startedJob{
		Benchmark:           in.Job.GetTelemetryBenchmark().Benchmark,
		Configuration:       in.Job.Config,
		Story:               in.Job.GetTelemetryBenchmark().GetStory(),
		Priority:            in.Job.Priority,
		InitialAttemptCount: in.Job.InitialAttemptCount,
	}
	if in.Job.GetTelemetryBenchmark().GetStoryTags() != nil {
		started_job.StoryTags = in.Job.GetTelemetryBenchmark().GetStoryTags().StoryTags
	} else {
		started_job.StoryTags = []string{}
	}
	c.Jobs = append(c.Jobs, started_job)
	return out, nil
}

func (c *fakePinpointClient) GetJob(ctx context.Context, in *proto.GetJobRequest, opts ...grpc.CallOption) (*proto.Job, error) {
	return nil, nil
}

func (c *fakePinpointClient) ListJobs(ctx context.Context, in *proto.ListJobsRequest, opts ...grpc.CallOption) (*proto.ListJobsResponse, error) {
	return nil, nil
}

func (c *fakePinpointClient) CancelJob(ctx context.Context, in *proto.CancelJobRequest, opts ...grpc.CallOption) (*proto.Job, error) {
	return nil, nil
}

func TestGetTarget(t *testing.T) {
	t.Parallel()
	ftt.Run("GetTarget should return different values for different bot cfgs", t, func(t *ftt.Test) {
		assert.Loosely(t, getTarget("somenewtarget"), should.Equal("performance_test_suite"))
		assert.Loosely(t, getTarget("android-go-wembley-perf"), should.Equal("performance_test_suite_android_trichrome_chrome_google_bundle"))
		assert.Loosely(t, getTarget("android-pixel4_webview-perf"), should.Equal("performance_webview_test_suite"))
	})
}

func TestBatchKickoff(t *testing.T) {
	t.Parallel()
	ftt.Run("A batch config should kick off a set of jobs", t, func(t *ftt.Test) {
		batch_experiments := []telemetryBatchExperiment{
			{
				Benchmark: "desktop",
				Configs:   []string{"linux", "win"},
				Stories:   []string{"dsA", "dsB"},
				StoryTags: []string{},
			},
			{
				Benchmark: "mobile",
				Configs:   []string{"pixel"},
				Stories:   []string{"msA"},
				StoryTags: []string{"tagA", "tagB"},
			},
		}
		runner := experimentTelemetryRun{}
		experiment := proto.Experiment{}

		c := &fakePinpointClient{}

		var err error
		runner.baseCommandRun.workDir, err = ioutil.TempDir("", "tmp")
		runner.initialAttemptCount = int(testInitialAttemptCount)
		runner.priority = int(testPriority)
		assert.Loosely(t, err, should.BeNil)
		jobs, err := runBatchJob(&runner, context.Background(), os.Stdout, c, "batch", batch_experiments, &experiment)
		assert.Loosely(t, err, should.BeNil)

		expected := []startedJob{
			{
				Benchmark:           "desktop",
				Configuration:       "linux",
				InitialAttemptCount: testInitialAttemptCount,
				Priority:            testPriority,
				Story:               "dsA",
				StoryTags:           []string{},
			},
			{
				Benchmark:           "desktop",
				Configuration:       "linux",
				InitialAttemptCount: testInitialAttemptCount,
				Priority:            testPriority,
				Story:               "dsB",
				StoryTags:           []string{},
			},
			{
				Benchmark:           "desktop",
				Configuration:       "win",
				InitialAttemptCount: testInitialAttemptCount,
				Priority:            testPriority,
				Story:               "dsA",
				StoryTags:           []string{},
			},
			{
				Benchmark:           "desktop",
				Configuration:       "win",
				InitialAttemptCount: testInitialAttemptCount,
				Priority:            testPriority,
				Story:               "dsB",
				StoryTags:           []string{},
			},
			{
				Benchmark:           "mobile",
				Configuration:       "pixel",
				InitialAttemptCount: testInitialAttemptCount,
				Priority:            testPriority,
				Story:               "msA",
				StoryTags:           []string{},
			},
			{
				Benchmark:           "mobile",
				Configuration:       "pixel",
				InitialAttemptCount: testInitialAttemptCount,
				Priority:            testPriority,
				Story:               "",
				StoryTags:           []string{"tagA", "tagB"},
			},
		}
		assert.Loosely(t, cmp.Equal(c.Jobs, expected, cmpopts.SortSlices(less)), should.BeTrue)
		assert.Loosely(t, len(jobs), should.Equal(6))

		// Check the jobs file
		jobs_filename := filepath.Join(runner.baseCommandRun.workDir, "batch.txt")
		_, err = os.Stat(jobs_filename)
		assert.Loosely(t, err, should.BeNil)
		content, err := ioutil.ReadFile(jobs_filename)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cmp.Equal(string(content), "4242\n4242\n4242\n4242\n4242\n4242\n"), should.BeTrue)
	})
}

func TestCLIFlagOverriding(t *testing.T) {
	t.Parallel()
	ftt.Run("CLI args must override presets", t, func(t *ftt.Test) {
		batch_experiments := []telemetryBatchExperiment{
			{
				Benchmark:   "desktop",
				Configs:     []string{"linux", "win"},
				Stories:     []string{"dsA", "dsB"},
				StoryTags:   []string{},
				Measurement: "FCP",
			},
			{
				Benchmark:   "mobile",
				Configs:     []string{"pixel"},
				Stories:     []string{"msA"},
				StoryTags:   []string{"tagA", "tagB"},
				Measurement: "LCP",
			},
		}
		runner := experimentTelemetryRun{}
		runner.configurations = []string{"config1", "config2"}
		runner.stories = []string{"story1, story2"}
		runner.storyTags = []string{"tag1", "tag2"}
		runner.measurement = "measurement"
		runner.benchmark = "benchmark"
		applyFlags(&runner, &batch_experiments, []string{"extra_arg1", "extra_arg2"})
		expected := []telemetryBatchExperiment{
			{
				Benchmark:   "benchmark",
				Configs:     []string{"config1", "config2"},
				Stories:     []string{"story1, story2"},
				StoryTags:   []string{"tag1", "tag2"},
				Measurement: "measurement",
				ExtraArgs:   []string{"extra_arg1", "extra_arg2"},
			},
			{
				Benchmark:   "benchmark",
				Configs:     []string{"config1", "config2"},
				Stories:     []string{"story1, story2"},
				StoryTags:   []string{"tag1", "tag2"},
				Measurement: "measurement",
				ExtraArgs:   []string{"extra_arg1", "extra_arg2"},
			},
		}
		fmt.Println(cmp.Diff(batch_experiments, expected))
		assert.Loosely(t, cmp.Equal(batch_experiments, expected), should.BeTrue)
	})

	ftt.Run("A single CLI override must not impact other preset params", t, func(t *ftt.Test) {
		batch_experiments := []telemetryBatchExperiment{
			{
				Benchmark:   "desktop",
				Configs:     []string{"linux", "win"},
				Stories:     []string{"dsA", "dsB"},
				StoryTags:   []string{},
				Measurement: "FCP",
			},
			{
				Benchmark:   "mobile",
				Configs:     []string{"pixel"},
				Stories:     []string{"msA"},
				StoryTags:   []string{"tagA", "tagB"},
				Measurement: "LCP",
			},
		}
		runner := experimentTelemetryRun{}
		runner.configurations = []string{"config1", "config2"}
		applyFlags(&runner, &batch_experiments, nil)
		expected := []telemetryBatchExperiment{
			{
				Benchmark:   "desktop",
				Configs:     []string{"config1", "config2"},
				Stories:     []string{"dsA", "dsB"},
				StoryTags:   []string{},
				Measurement: "FCP",
			},
			{
				Benchmark:   "mobile",
				Configs:     []string{"config1", "config2"},
				Stories:     []string{"msA"},
				StoryTags:   []string{"tagA", "tagB"},
				Measurement: "LCP",
			},
		}
		fmt.Println(cmp.Diff(batch_experiments, expected))
		assert.Loosely(t, cmp.Equal(batch_experiments, expected), should.BeTrue)
	})
}

func TestGetTelemetryBatchExperiments(t *testing.T) {
	t.Parallel()

	ftt.Run("Zero experiments are generated from no preset and incomplete CLI flags", t, func(t *ftt.Test) {
		p := preset{}
		runner := experimentTelemetryRun{}
		runner.configurations = []string{"config1", "config2"}
		runner.benchmark = "benchmark"
		actual, _ := getTelemetryBatchExperiments(&runner, nil, p)
		expected := []telemetryBatchExperiment{}
		assert.Loosely(t, actual, should.Match(expected))
	})

	ftt.Run("A valid experiment is generated from only CLI flags", t, func(t *ftt.Test) {
		p := preset{}
		runner := experimentTelemetryRun{}
		runner.configurations = []string{"config1", "config2"}
		runner.stories = []string{"story1, story2"}
		runner.storyTags = []string{"tag1", "tag2"}
		runner.measurement = "measurement"
		runner.benchmark = "benchmark"
		actual, _ := getTelemetryBatchExperiments(&runner, nil, p)
		expected := []telemetryBatchExperiment{
			{
				Benchmark:   "benchmark",
				Configs:     []string{"config1", "config2"},
				Stories:     []string{"story1, story2"},
				StoryTags:   []string{"tag1", "tag2"},
				Measurement: "measurement",
			},
		}
		assert.Loosely(t, actual, should.Match(expected))
	})

	ftt.Run("A valid experiment is generated from a single-run preset and CLI flags", t, func(t *ftt.Test) {
		experiment := telemetryExperimentJobSpec{
			Benchmark:   "benchmark",
			Config:      "cfg",
			Measurement: "measurement",
			ExtraArgs:   []string{"arg1"},
		}
		experiment.StorySelection.Story = "story"
		p := preset{
			TelemetryExperiment: &experiment,
		}
		runner := experimentTelemetryRun{}
		runner.configurations = []string{"config1", "config2"}
		actual, _ := getTelemetryBatchExperiments(&runner, nil, p)
		expected := []telemetryBatchExperiment{
			{
				Benchmark:   "benchmark",
				Configs:     []string{"config1", "config2"},
				Stories:     []string{"story"},
				Measurement: "measurement",
				ExtraArgs:   []string{"arg1"},
			},
		}
		assert.Loosely(t, actual, should.Match(expected))
	})

	ftt.Run("A valid experiment is generated from a multi-run preset and CLI flags", t, func(t *ftt.Test) {
		batch_experiments := []telemetryBatchExperiment{
			{
				Benchmark:   "desktop",
				Configs:     []string{"linux", "win"},
				Stories:     []string{"dsA", "dsB"},
				StoryTags:   []string{},
				Measurement: "FCP",
			},
			{
				Benchmark:   "mobile",
				Configs:     []string{"pixel"},
				Stories:     []string{"msA"},
				StoryTags:   []string{"tagA", "tagB"},
				Measurement: "LCP",
			},
		}
		p := preset{
			TelemetryBatchExperiment: &batch_experiments,
		}
		runner := experimentTelemetryRun{}
		runner.configurations = []string{"config1", "config2"}
		actual, _ := getTelemetryBatchExperiments(&runner, nil, p)
		expected := []telemetryBatchExperiment{
			{
				Benchmark:   "desktop",
				Configs:     []string{"config1", "config2"},
				Stories:     []string{"dsA", "dsB"},
				StoryTags:   []string{},
				Measurement: "FCP",
			},
			{
				Benchmark:   "mobile",
				Configs:     []string{"config1", "config2"},
				Stories:     []string{"msA"},
				StoryTags:   []string{"tagA", "tagB"},
				Measurement: "LCP",
			},
		}
		assert.Loosely(t, actual, should.Match(expected))
	})
}
