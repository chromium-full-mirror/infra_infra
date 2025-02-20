// Copyright 2020 The Chromium Authors.
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

package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/chromeperf/pinpoint"
	"infra/chromeperf/pinpoint/assertions"
	"infra/chromeperf/pinpoint/proto"
)

const bufSize = 1024 * 1024

func registerPinpointServer(t testing.TB, srv *pinpointServer) func(context.Context, string) (net.Conn, error) {
	t.Helper()

	l := bufconn.Listen(bufSize)
	t.Cleanup(func() { l.Close() })
	s := grpc.NewServer()
	t.Cleanup(func() { s.Stop() })
	dialer := func(context.Context, string) (net.Conn, error) {
		return l.Dial()
	}
	proto.RegisterPinpointServer(s, srv)
	go func() {
		if err := s.Serve(l); err != nil {
			log.Fatalf("Server startup failed.")
		}
	}()
	return dialer
}

type requestRecorder struct {
	*httptest.Server

	// List of URLs of received requests during recording.
	urls      []*url.URL
	recording bool
}

// startRecord begins the storage of URLs executed on the test server. It
// returns a function which can be used to retrieve results. Note that the
// returned function must be called in order to clean up.
func (rr *requestRecorder) startRecord() (done func() []*url.URL) {
	if rr.recording {
		panic("Invalid state: startRecorder called before prior recording was finished")
	}
	rr.recording = true
	rr.urls = nil
	return func() []*url.URL {
		rr.recording = false
		return rr.urls
	}
}

func startFakeLegacyServer(t testing.TB, httpResponses map[string]string) *requestRecorder {
	ret := new(requestRecorder)
	ret.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ret.recording {
			ret.urls = append(ret.urls, r.URL)
		}

		resp, found := httpResponses[r.URL.Path]
		if !found {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintln(w, "Not found")
			return
		}
		fmt.Fprintln(w, resp)
	}))
	t.Cleanup(ret.Server.Close)
	return ret
}

func TestServerService(t *testing.T) {
	ctx := context.Background()
	ftt.Run("Given a grpc server without a client", t, func(t *ftt.Test) {
		dialer := registerPinpointServer(t, &pinpointServer{})

		t.Run("When we connect to the Pinpoint service", func(t *ftt.Test) {
			conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(dialer), grpc.WithInsecure())
			assert.Loosely(t, err, should.BeNil)
			t.Cleanup(func() { conn.Close() })
			client := proto.NewPinpointClient(conn)

			t.Run("Then requests to ScheduleJob will fail with 'misconfigured service'", func(t *ftt.Test) {
				_, err := client.ScheduleJob(ctx, &proto.ScheduleJobRequest{})
				assert.Loosely(t, err, should.NotBeNil)
				assert.Loosely(t, err.Error(), should.ContainSubstring("misconfigured service"))
			})

		})
	})

	ftt.Run("Given a grpc server with a legacy client not behind the ESP", t, func(t *ftt.Test) {
		ts := startFakeLegacyServer(t, nil)
		log.Printf("legacy service = %s", ts.URL)
		dialer := registerPinpointServer(t, &pinpointServer{legacyPinpointService: ts.URL, LegacyClient: &http.Client{}})

		t.Run("When we connect to the Pinpoint service", func(t *ftt.Test) {
			conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(dialer), grpc.WithInsecure())
			assert.Loosely(t, err, should.BeNil)
			defer conn.Close()
			client := proto.NewPinpointClient(conn)
			t.Run("Then requests to ScheduleJob will fail with 'missing required auth header'", func(t *ftt.Test) {
				_, err := client.ScheduleJob(ctx, &proto.ScheduleJobRequest{})
				assert.Loosely(t, err, assertions.ShouldBeStatusError(codes.PermissionDenied))
				assert.Loosely(t, err.Error(), should.ContainSubstring("missing required auth header"))
			})
		})
	})
}

func TestGetJob(t *testing.T) {
	t.Parallel()
	definedJobExperimentJSON, err := os.ReadFile("testdata/defined-job-experiment.json")
	if err != nil {
		t.Fatal(err)
	}

	const (
		definedJobID   = "11423cdd520000"
		definedJobName = "jobs/legacy-" + definedJobID
	)
	ts := startFakeLegacyServer(t, map[string]string{
		"/api/job/" + definedJobID: string(definedJobExperimentJSON),
	})
	defer ts.Close()
	log.Printf("legacy service = %s", ts.URL)

	ctx := context.Background()
	ftt.Run("Given a grpc server with a client", t, func(t *ftt.Test) {
		dialer := registerPinpointServer(t, &pinpointServer{legacyPinpointService: ts.URL, LegacyClient: &http.Client{}})

		conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(dialer), grpc.WithInsecure())
		assert.Loosely(t, err, should.BeNil)
		defer conn.Close()
		client := proto.NewPinpointClient(conn)

		t.Run("When we attempt to get a defined job", func(t *ftt.Test) {
			j, err := client.GetJob(ctx, &proto.GetJobRequest{
				Name: definedJobName,
			})

			t.Run("Then we find details in the response proto", func(t *ftt.Test) {
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, j.Name, should.Equal(definedJobName))
			})
		})

		t.Run("When we attempt to get an undefined job", func(t *ftt.Test) {
			_, err := client.GetJob(ctx, &proto.GetJobRequest{
				Name: "jobs/legacy-02",
			})
			t.Run("Then we get an error in the gRPC request", func(t *ftt.Test) {
				assert.Loosely(t, err, assertions.ShouldBeStatusError(codes.NotFound))
			})

		})

		t.Run("When we attempt to provide an ill-defined legacy id", func(t *ftt.Test) {
			_, err := client.GetJob(ctx, &proto.GetJobRequest{
				Name: "jobs/legacy-",
			})
			t.Run("Then we get an error in the gRPC request", func(t *ftt.Test) {
				assert.Loosely(t, err, assertions.ShouldBeStatusError(codes.InvalidArgument))
			})
		})

		t.Run("When we attempt go get an experiment job with results", func(t *ftt.Test) {
			j, err := client.GetJob(ctx, &proto.GetJobRequest{
				Name: definedJobName,
			})
			t.Run("Then we find the results in the response", func(t *ftt.Test) {
				assert.Loosely(t, err, should.BeNil)
				exp := j.JobSpec.GetExperiment()
				assert.Loosely(t, exp, should.NotBeNil)
				assert.Loosely(t, exp.BaseCommit.GitHash, should.Equal("0d8952cfc50b039bf50320c9d3db82b164f3e549"))
				assert.Loosely(t, exp.ExperimentPatch.Change, should.Equal(2560197))
				assert.Loosely(t, exp.ExperimentPatch.Patchset, should.Equal(12))
			})
		})
	})
}

func TestCancelJob(t *testing.T) {
	t.Parallel()
	definedJobExperimentJSON, err := os.ReadFile("testdata/defined-job-experiment.json")
	if err != nil {
		t.Fatal(err)
	}

	const (
		definedJobID   = "11423cdd520000"
		definedJobName = "jobs/legacy-" + definedJobID
	)
	ts := startFakeLegacyServer(t, map[string]string{
		"/api/job/" + definedJobID: string(definedJobExperimentJSON),
		"/api/job/cancel":          "",
	})
	defer ts.Close()
	log.Printf("legacy service = %s", ts.URL)

	ctx := context.Background()
	authorizedCtx := metadata.NewOutgoingContext(ctx, metadata.MD{
		EndpointsHeader: []string{
			base64.RawURLEncoding.EncodeToString([]byte(`{"email": "anonymous-user@example.com"}`)),
		},
	})
	ftt.Run("Given a grpc server with a client", t, func(t *ftt.Test) {
		dialer := registerPinpointServer(t, &pinpointServer{legacyPinpointService: ts.URL, LegacyClient: &http.Client{}})

		conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(dialer), grpc.WithInsecure())
		assert.Loosely(t, err, should.BeNil)
		defer conn.Close()
		client := proto.NewPinpointClient(conn)

		t.Run("with an un-authenticated connection", func(t *ftt.Test) {
			t.Run("We fail to cancel the job", func(t *ftt.Test) {
				_, err := client.CancelJob(ctx, &proto.CancelJobRequest{Name: definedJobName, Reason: "because"})
				assert.Loosely(t, err, assertions.ShouldBeStatusError(codes.PermissionDenied))
			})
		})

		t.Run("with an authenticated connection", func(t *ftt.Test) {
			t.Run("with an authorized connection", func(t *ftt.Test) {
				ctx := authorizedCtx
				t.Run("We fail to cancel the Job without a reason", func(t *ftt.Test) {
					_, err := client.CancelJob(ctx, &proto.CancelJobRequest{Name: definedJobName})
					assert.Loosely(t, err, assertions.ShouldBeStatusError(codes.InvalidArgument))
				})
				t.Run("We fail to cancel the Job without a name", func(t *ftt.Test) {
					_, err := client.CancelJob(ctx, &proto.CancelJobRequest{Reason: "because"})
					assert.Loosely(t, err, assertions.ShouldBeStatusError(codes.InvalidArgument))
				})
				t.Run("We can cancel a job with name and reason", func(t *ftt.Test) {
					j, err := client.CancelJob(ctx, &proto.CancelJobRequest{Name: definedJobName, Reason: "because"})
					assert.Loosely(t, err, should.BeNil)
					assert.Loosely(t, j.Name, should.Equal(definedJobName))
				})
				t.Run("We fail to cancel a missing job", func(t *ftt.Test) {
					_, err := client.CancelJob(ctx, &proto.CancelJobRequest{Name: "doesnt-exist", Reason: "because"})
					assert.Loosely(t, err, assertions.ShouldBeStatusError(codes.InvalidArgument))
				})
			})
		})
	})
}

func TestListJob(t *testing.T) {
	t.Parallel()
	listResponseJSON, err := os.ReadFile("testdata/example-list-response.json")
	if err != nil {
		t.Fatal(err)
	}

	ts := startFakeLegacyServer(t, map[string]string{
		"/api/jobs": string(listResponseJSON),
	})
	defer ts.Close()

	ctx := context.Background()
	ftt.Run("Given a grpc server with a client", t, func(t *ftt.Test) {
		dialer := registerPinpointServer(t, &pinpointServer{legacyPinpointService: ts.URL, LegacyClient: &http.Client{}})

		conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(dialer), grpc.WithInsecure())
		assert.Loosely(t, err, should.BeNil)
		defer conn.Close()
		client := proto.NewPinpointClient(conn)

		t.Run("listing results is successful", func(t *ftt.Test) {
			_, err := client.ListJobs(ctx, &proto.ListJobsRequest{})
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("filters in the RPC request make it to the legacy service", func(t *ftt.Test) {
			const filter = "THE FILTER"

			getURLs := ts.startRecord()
			_, err := client.ListJobs(ctx, &proto.ListJobsRequest{Filter: filter})
			urls := getURLs()
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, len(urls), should.Equal(1))
			assert.Loosely(t, urls[0].String(), should.ContainSubstring(url.QueryEscape(filter)))
		})
	})
}

func TestScheduleJob(t *testing.T) {
	t.Parallel()

	const jobID = "1234567890abcd"
	jobName := pinpoint.LegacyJobName(jobID)
	ts := startFakeLegacyServer(t, map[string]string{
		"/api/new": fmt.Sprintf(`{"jobId": %q}`, jobID),
	})
	defer ts.Close()

	ctx := context.Background()
	authorizedCtx := metadata.NewOutgoingContext(ctx, metadata.MD{
		EndpointsHeader: []string{
			base64.RawURLEncoding.EncodeToString([]byte(`{"email": "user@example.com"}`)),
		},
	})
	ftt.Run("Given a grpc server with a client", t, func(t *ftt.Test) {
		dialer := registerPinpointServer(t, &pinpointServer{legacyPinpointService: ts.URL, LegacyClient: &http.Client{}})

		conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(dialer), grpc.WithInsecure())
		assert.Loosely(t, err, should.BeNil)
		defer conn.Close()
		client := proto.NewPinpointClient(conn)

		t.Run("without authentication, ScheduleJob fails", func(t *ftt.Test) {
			_, err := client.ScheduleJob(ctx, &proto.ScheduleJobRequest{})
			assert.Loosely(t, err, assertions.ShouldBeStatusError(codes.PermissionDenied))
		})
		t.Run("with authentication", func(t *ftt.Test) {
			ctx := authorizedCtx

			t.Run("without appropriate arguments, ScheduleJob fails", func(t *ftt.Test) {
				_, err := client.ScheduleJob(ctx, &proto.ScheduleJobRequest{})
				assert.Loosely(t, err, assertions.ShouldBeStatusError(codes.InvalidArgument))
			})

			t.Run("with correct GTestBenchmark arguments, ScheduleJob succeeds", func(t *ftt.Test) {
				j, err := client.ScheduleJob(ctx, &proto.ScheduleJobRequest{
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
				assert.Loosely(t, j.Name, should.Equal(jobName))
			})

			t.Run("with extra args for a Telemetry job, ScheduleJob succeeds", func(t *ftt.Test) {
				j, err := client.ScheduleJob(ctx, &proto.ScheduleJobRequest{
					Job: &proto.JobSpec{
						Config: "some-config",
						Target: "some-target",
						Arguments: &proto.JobSpec_TelemetryBenchmark{
							TelemetryBenchmark: &proto.TelemetryBenchmark{
								Benchmark: "benchmark",
								StorySelection: &proto.TelemetryBenchmark_Story{
									Story: "some-story",
								},
								Measurement: "measurement",
								ExtraArgs:   []string{"--browser", "some-browser"},
							},
						},
					},
				})
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, j.Name, should.Equal(jobName))
			})
		})
	})
}
