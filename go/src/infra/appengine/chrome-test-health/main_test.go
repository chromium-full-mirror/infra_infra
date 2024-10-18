// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"testing"

	"cloud.google.com/go/civil"
	"google.golang.org/grpc/codes"

	. "go.chromium.org/luci/common/testing/assertions"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/convey"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/appengine/chrome-test-health/api"
)

type clientMock struct {
	lastListReq     *api.ListComponentsRequest
	lastFetchReq    *api.FetchTestMetricsRequest
	lastFetchDirReq *api.FetchDirectoryMetricsRequest
}

type coverageClientMock struct {
	lastGetProjectDefaultConfigReq           *api.GetProjectDefaultConfigRequest
	lastGetCoverageSummaryReq                *api.GetCoverageSummaryRequest
	lastGetAbsoluteCoverageDataOneYearReq    *api.GetAbsoluteCoverageDataOneYearRequest
	lastGetIncrementalCoverageDataOneYearReq *api.GetIncrementalCoverageDataOneYearRequest
}

func (cm *clientMock) UpdateSummary(_ context.Context, fromDate civil.Date, toDate civil.Date) error {
	return nil
}

func (cm *clientMock) ListComponents(ctx context.Context, req *api.ListComponentsRequest) (*api.ListComponentsResponse, error) {
	cm.lastListReq = req
	return &api.ListComponentsResponse{}, nil
}

func (cm *clientMock) FetchMetrics(ctx context.Context, req *api.FetchTestMetricsRequest) (*api.FetchTestMetricsResponse, error) {
	cm.lastFetchReq = req
	return &api.FetchTestMetricsResponse{}, nil
}

func (cm *clientMock) FetchDirectoryMetrics(ctx context.Context, req *api.FetchDirectoryMetricsRequest) (*api.FetchDirectoryMetricsResponse, error) {
	cm.lastFetchDirReq = req
	return &api.FetchDirectoryMetricsResponse{}, nil
}

func (ccm *coverageClientMock) GetProjectDefaultConfig(ctx context.Context, req *api.GetProjectDefaultConfigRequest) (*api.GetProjectDefaultConfigResponse, error) {
	ccm.lastGetProjectDefaultConfigReq = req
	return &api.GetProjectDefaultConfigResponse{}, nil
}

func (ccm *coverageClientMock) GetCoverageSummary(ctx context.Context, req *api.GetCoverageSummaryRequest) (*api.GetCoverageSummaryResponse, error) {
	ccm.lastGetCoverageSummaryReq = req
	return &api.GetCoverageSummaryResponse{}, nil
}

func (ccm *coverageClientMock) GetAbsoluteCoverageDataOneYear(
	ctx context.Context,
	req *api.GetAbsoluteCoverageDataOneYearRequest,
) (*api.GetAbsoluteCoverageDataOneYearResponse, error) {
	ccm.lastGetAbsoluteCoverageDataOneYearReq = req
	return &api.GetAbsoluteCoverageDataOneYearResponse{}, nil
}

func (ccm *coverageClientMock) GetIncrementalCoverageDataOneYear(
	ctx context.Context,
	req *api.GetIncrementalCoverageDataOneYearRequest,
) (*api.GetIncrementalCoverageDataOneYearResponse, error) {
	ccm.lastGetIncrementalCoverageDataOneYearReq = req
	return &api.GetIncrementalCoverageDataOneYearResponse{}, nil
}

func TestValidatePresence(t *testing.T) {
	t.Parallel()

	ftt.Run("Validate Presence", t, func(t *ftt.Test) {
		t.Run("Should be false for empty string", func(t *ftt.Test) {
			isPresent := validatePresence("   ")
			assert.Loosely(t, isPresent, should.BeFalse)
		})
		t.Run("Should be false for nil", func(t *ftt.Test) {
			isPresent := validatePresence(nil)
			assert.Loosely(t, isPresent, should.BeFalse)
		})
		t.Run("Should be true", func(t *ftt.Test) {
			isPresent := validatePresence("test")
			assert.Loosely(t, isPresent, should.BeTrue)
		})
	})
}

func TestValidateFormat(t *testing.T) {
	t.Parallel()

	ftt.Run("Validate Format", t, func(t *ftt.Test) {
		t.Run("Should be false", func(t *ftt.Test) {
			isValidFormat := validateFormat("test4", "^(test1|test2|test3)$")
			assert.Loosely(t, isValidFormat, should.BeFalse)
		})
		t.Run("Should be true", func(t *ftt.Test) {
			isValidFormat := validateFormat("test1", "^(test1|test2|test3)$")
			assert.Loosely(t, isValidFormat, should.BeTrue)
		})
	})
}

func TestUpdateDailySummary(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ftt.Run("DailySummary", t, func(t *ftt.Test) {
		mock := &clientMock{}

		srv := &testResourcesServer{
			Client: mock,
		}
		t.Run("Valid request", func(t *ftt.Test) {
			request := &api.UpdateMetricsTableRequest{
				FromDate: "2023-01-01",
				ToDate:   "2023-01-02",
			}
			resp, err := srv.UpdateMetricsTable(ctx, request)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
		})
		t.Run("Bad from date request", func(t *ftt.Test) {
			request := &api.UpdateMetricsTableRequest{
				FromDate: "asdf",
				ToDate:   "2023-01-02",
			}
			resp, err := srv.UpdateMetricsTable(ctx, request)

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("Bad to date request", func(t *ftt.Test) {
			request := &api.UpdateMetricsTableRequest{
				FromDate: "2023-01-01",
				ToDate:   "asdf",
			}
			resp, err := srv.UpdateMetricsTable(ctx, request)

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("Missing from date", func(t *ftt.Test) {
			request := &api.UpdateMetricsTableRequest{
				ToDate: "2023-01-01",
			}
			resp, err := srv.UpdateMetricsTable(ctx, request)

			assert.Loosely(t, err, should.ErrLike("from_date"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("Missing to date", func(t *ftt.Test) {
			request := &api.UpdateMetricsTableRequest{
				FromDate: "2023-01-01",
			}
			resp, err := srv.UpdateMetricsTable(ctx, request)

			assert.Loosely(t, err, should.ErrLike("to_date"))
			assert.Loosely(t, resp, should.BeNil)
		})
	})

}

func TestListComponents(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ftt.Run("ListComponents", t, func(t *ftt.Test) {
		mock := &clientMock{}

		srv := &testResourcesServer{
			Client: mock,
		}
		request := &api.ListComponentsRequest{}
		srv.ListComponents(ctx, request)

		assert.Loosely(t, request, should.Match(mock.lastListReq))
	})
}

func TestFetchMetrics(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ftt.Run("FetchMetrics", t, func(t *ftt.Test) {
		mock := &clientMock{}

		srv := &testResourcesServer{
			Client: mock,
		}
		t.Run("Valid request", func(t *ftt.Test) {
			request := &api.FetchTestMetricsRequest{
				Components: []string{"some>component"},
				Period:     api.Period_DAY,
				Dates:      []string{"2023-01-01"},
				Metrics:    []api.MetricType{api.MetricType_NUM_RUNS},
				Filter:     "filter:this",
				PageOffset: 1,
				PageSize:   10,
				Sort: &api.SortBy{
					Metric:    api.SortType_SORT_NUM_RUNS,
					Ascending: true,
				},
			}
			resp, err := srv.FetchTestMetrics(ctx, request)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, mock.lastFetchReq, should.Match(request))
		})
		t.Run("Missing dates", func(t *ftt.Test) {
			request := &api.FetchTestMetricsRequest{
				Components: []string{"some>component"},
				Period:     api.Period_DAY,
				Metrics:    []api.MetricType{api.MetricType_NUM_RUNS},
				Filter:     "filter:this",
				PageOffset: 1,
				PageSize:   10,
				Sort: &api.SortBy{
					Metric:    api.SortType_SORT_NUM_RUNS,
					Ascending: true,
				},
			}
			resp, err := srv.FetchTestMetrics(ctx, request)

			assert.Loosely(t, err, should.ErrLike("dates"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("Missing metrics", func(t *ftt.Test) {
			request := &api.FetchTestMetricsRequest{
				Components: []string{"some>component"},
				Period:     api.Period_DAY,
				Dates:      []string{"2023-01-01"},
				Filter:     "filter:this",
				PageOffset: 1,
				PageSize:   10,
				Sort: &api.SortBy{
					Metric:    api.SortType_SORT_NUM_RUNS,
					Ascending: true,
				},
			}
			resp, err := srv.FetchTestMetrics(ctx, request)

			assert.Loosely(t, err, should.ErrLike("metrics"))
			assert.Loosely(t, resp, should.BeNil)
		})
	})
}

func TestFetchFileMetrics(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ftt.Run("FetchFileMetrics", t, func(t *ftt.Test) {
		mock := &clientMock{}

		srv := &testResourcesServer{
			Client: mock,
		}
		t.Run("Valid request", func(t *ftt.Test) {
			request := &api.FetchDirectoryMetricsRequest{
				Components: []string{"some>component"},
				Period:     api.Period_DAY,
				Dates:      []string{"2023-01-01"},
				Metrics:    []api.MetricType{api.MetricType_NUM_RUNS},
				Filter:     "filter:this",
				ParentIds:  []string{"/"},
				Sort: &api.SortBy{
					Metric:    api.SortType_SORT_NUM_RUNS,
					Ascending: true,
				},
			}
			resp, err := srv.FetchDirectoryMetrics(ctx, request)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, mock.lastFetchDirReq, should.Match(request))
		})
		t.Run("Missing dates", func(t *ftt.Test) {
			request := &api.FetchDirectoryMetricsRequest{
				Components: []string{"some>component"},
				Period:     api.Period_DAY,
				Metrics:    []api.MetricType{api.MetricType_NUM_RUNS},
				Filter:     "filter:this",
				ParentIds:  []string{"/"},
				Sort: &api.SortBy{
					Metric:    api.SortType_SORT_NUM_RUNS,
					Ascending: true,
				},
			}
			resp, err := srv.FetchDirectoryMetrics(ctx, request)

			assert.Loosely(t, err, should.ErrLike("dates"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("Missing parentId", func(t *ftt.Test) {
			request := &api.FetchDirectoryMetricsRequest{
				Components: []string{"some>component"},
				Period:     api.Period_DAY,
				Dates:      []string{"2023-01-01"},
				Metrics:    []api.MetricType{api.MetricType_NUM_RUNS},
				Filter:     "filter:this",
				Sort: &api.SortBy{
					Metric:    api.SortType_SORT_NUM_RUNS,
					Ascending: true,
				},
			}
			resp, err := srv.FetchDirectoryMetrics(ctx, request)

			assert.Loosely(t, err, should.ErrLike("parent_id"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("Missing metrics", func(t *ftt.Test) {
			request := &api.FetchDirectoryMetricsRequest{
				Components: []string{"some>component"},
				Period:     api.Period_DAY,
				Dates:      []string{"2023-01-01"},
				Filter:     "filter:this",
				ParentIds:  []string{"/"},
				Sort: &api.SortBy{
					Metric:    api.SortType_SORT_NUM_RUNS,
					Ascending: true,
				},
			}
			resp, err := srv.FetchDirectoryMetrics(ctx, request)

			assert.Loosely(t, err, should.ErrLike("metrics"))
			assert.Loosely(t, resp, should.BeNil)
		})
	})
}

func TestGetCoverageSummary(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ftt.Run("GetCoverageSummary", t, func(t *ftt.Test) {
		mock := &coverageClientMock{}
		srv := &coverageServer{
			Client: mock,
		}
		request := &api.GetCoverageSummaryRequest{
			GitilesHost:     "chromium.googlesource.com",
			GitilesProject:  "chromium/src",
			GitilesRef:      "refs/heads/main",
			GitilesRevision: "03d4e64771cbc97f3ca5e4bbe85490d7cf909a0a",
			UnitTestsOnly:   false,
			Path:            "//chrome/browser/display_capture/",
			Bucket:          "ci",
			Builder:         "linux-code-coverage",
		}
		t.Run("Valid request", func(t *ftt.Test) {
			resp, err := srv.GetCoverageSummary(ctx, request)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, mock.lastGetCoverageSummaryReq, should.Match(request))
		})
		t.Run("Missing gitiles host", func(t *ftt.Test) {
			req := request
			req.GitilesHost = ""
			resp, err := srv.GetCoverageSummary(ctx, request)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("Gitiles Host is a required argument"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("Missing gitiles project", func(t *ftt.Test) {
			req := request
			req.GitilesProject = ""
			resp, err := srv.GetCoverageSummary(ctx, request)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("Gitiles Project is a required argument"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("Missing gitiles ref", func(t *ftt.Test) {
			req := request
			req.GitilesRef = ""
			resp, err := srv.GetCoverageSummary(ctx, request)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("Gitiles Ref is a required argument"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("Missing gitiles revision", func(t *ftt.Test) {
			req := request
			req.GitilesRevision = ""
			resp, err := srv.GetCoverageSummary(ctx, request)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("Gitiles Revision is a required argument"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("Missing gitiles both path and components", func(t *ftt.Test) {
			req := request
			req.Path = ""
			req.Components = []string{}
			resp, err := srv.GetCoverageSummary(ctx, request)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("Either path or components should be specified"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("Both path and components specified", func(t *ftt.Test) {
			req := request
			req.Path = "//"
			req.Components = []string{"C1", "C2"}
			resp, err := srv.GetCoverageSummary(ctx, request)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("Either path or components should be specified not both"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("Invalid Builder", func(t *ftt.Test) {
			req := request
			req.Builder = ""
			resp, err := srv.GetCoverageSummary(ctx, request)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("Builder is a required argument"))
			assert.Loosely(t, resp, should.BeNil)

			req.Builder = "linux-code-coverage&123"
			resp, err = srv.GetCoverageSummary(ctx, request)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("Builder is not provided in required format"))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("Invalid Bucket", func(t *ftt.Test) {
			req := request
			req.Bucket = ""
			resp, err := srv.GetCoverageSummary(ctx, request)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("Bucket is a required argument"))
			assert.Loosely(t, resp, should.BeNil)

			req.Bucket = "ci#121"
			resp, err = srv.GetCoverageSummary(ctx, request)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("Bucket is not provided in required format"))
			assert.Loosely(t, resp, should.BeNil)
		})
	})
}

func TestGetAbsoluteCoverageDataOneYear(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mock := &coverageClientMock{}
	srv := &coverageServer{
		Client: mock,
	}
	request := &api.GetAbsoluteCoverageDataOneYearRequest{
		Paths:         []string{"//p1/p2/"},
		Components:    []string{"C1", "C2"},
		UnitTestsOnly: true,
		Bucket:        "ci",
		Builder:       "linux-code-coverage",
	}

	ftt.Run("Should pass", t, func(t *ftt.Test) {
		req := request
		_, err := srv.GetAbsoluteCoverageDataOneYear(ctx, req)
		assert.Loosely(t, err, should.BeNil)
	})

	ftt.Run("Should fail", t, func(t *ftt.Test) {
		t.Run("Missing required params", func(t *ftt.Test) {
			t.Run("Missing both paths and components", func(t *ftt.Test) {
				req := request
				req.Paths = []string{}
				req.Components = []string{}
				resp, err := srv.GetAbsoluteCoverageDataOneYear(ctx, req)
				assert.Loosely(t, err, should.NotBeNil)
				assert.Loosely(t, err, should.ErrLike("Either paths or components should be specified"))
				assert.Loosely(t, resp, should.BeNil)
			})
			t.Run("Missing bucket", func(t *ftt.Test) {
				req := request
				req.Bucket = ""
				resp, err := srv.GetAbsoluteCoverageDataOneYear(ctx, req)
				assert.Loosely(t, err, should.NotBeNil)
				assert.Loosely(t, err, should.ErrLike("Bucket is a required argument"))
				assert.Loosely(t, resp, should.BeNil)
			})
			t.Run("Missing builder", func(t *ftt.Test) {
				req := request
				req.Builder = ""
				resp, err := srv.GetAbsoluteCoverageDataOneYear(ctx, req)
				assert.Loosely(t, err, should.NotBeNil)
				assert.Loosely(t, err, should.ErrLike("Builder is a required argument"))
				assert.Loosely(t, resp, should.BeNil)
			})
		})

		t.Run("Invalid params", func(t *ftt.Test) {
			t.Run("Invalid Builder", func(t *ftt.Test) {
				req := request
				req.Builder = "a___$$$b"
				resp, err := srv.GetAbsoluteCoverageDataOneYear(ctx, req)
				assert.Loosely(t, err, should.NotBeNil)
				assert.Loosely(t, err, should.ErrLike("Builder is not provided in required format"))
				assert.Loosely(t, resp, should.BeNil)
			})
			t.Run("Invalid Bucket", func(t *ftt.Test) {
				req := request
				req.Builder = "linux-code-coverage"
				req.Bucket = "a___$$$b"
				resp, err := srv.GetAbsoluteCoverageDataOneYear(ctx, req)
				assert.Loosely(t, err, should.NotBeNil)
				assert.Loosely(t, err, should.ErrLike("Bucket is not provided in required format"))
				assert.Loosely(t, resp, should.BeNil)
			})
		})
	})
}

func TestGetIncrementalCoverageDataOneYear(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mock := &coverageClientMock{}
	srv := &coverageServer{
		Client: mock,
	}
	request := &api.GetIncrementalCoverageDataOneYearRequest{
		Paths: []string{"//p1/p2/"},
	}

	ftt.Run("Should fail", t, func(t *ftt.Test) {
		t.Run("Missing paths", func(t *ftt.Test) {
			req := request
			req.Paths = []string{}
			resp, err := srv.GetIncrementalCoverageDataOneYear(ctx, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("Paths should be specified"))
			assert.Loosely(t, resp, should.BeNil)
		})

		t.Run("Path not relative to project root", func(t *ftt.Test) {
			req := request
			req.Paths = []string{"/a/b/"}
			resp, err := srv.GetIncrementalCoverageDataOneYear(ctx, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("Path /a/b/ is not relative to root, it should start with //"))
			assert.Loosely(t, resp, should.BeNil)
		})
	})
}

func TestGetProjectDefaultConfig(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ftt.Run("GetProjectDefaultConfig", t, func(t *ftt.Test) {
		mock := &coverageClientMock{}

		srv := &coverageServer{
			Client: mock,
		}
		t.Run("Valid request", func(t *ftt.Test) {
			request := &api.GetProjectDefaultConfigRequest{
				LuciProject: "chromium",
			}
			resp, err := srv.GetProjectDefaultConfig(ctx, request)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, mock.lastGetProjectDefaultConfigReq, should.Match(request))
		})
		t.Run("Invalid argument Project", func(t *ftt.Test) {
			request := &api.GetProjectDefaultConfigRequest{
				LuciProject: "chromium src",
			}
			resp, err := srv.GetProjectDefaultConfig(ctx, request)

			assert.Loosely(t, err, should.ErrLike("Argument Project is invalid"))
			assert.Loosely(t, err, convey.Adapt(ShouldHaveAppStatus)(codes.InvalidArgument))
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("Missing project", func(t *ftt.Test) {
			request := &api.GetProjectDefaultConfigRequest{}
			resp, err := srv.GetProjectDefaultConfig(ctx, request)

			assert.Loosely(t, err, should.ErrLike("project"))
			assert.Loosely(t, resp, should.BeNil)
		})
	})
}

func TestPathRelativeToRoot(t *testing.T) {
	t.Parallel()

	ftt.Run("Should be true when path starts with //", t, func(t *ftt.Test) {
		isRel := pathRelativeToRoot("//a/b/")
		assert.Loosely(t, isRel, should.BeTrue)
	})

	ftt.Run("Should be false when path doesn't start with //", t, func(t *ftt.Test) {
		isRel := pathRelativeToRoot("/a/b/")
		assert.Loosely(t, isRel, should.BeFalse)
	})
}
