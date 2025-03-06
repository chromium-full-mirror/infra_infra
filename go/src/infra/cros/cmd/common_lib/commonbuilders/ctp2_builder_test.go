// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commonbuilders_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/infra/proto/go/chromiumos"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/luciexe/build"

	builders "go.chromium.org/infra/cros/cmd/common_lib/commonbuilders"
)

func MockManifestFetcher(ctx context.Context, gcsPath string) (string, error) {
	if strings.Contains(gcsPath, "public-manifest") {
		return "PUBLIC", nil
	} else {
		return "PRIVATE", nil
	}
}

func ConstructCtpv2Req(reqTov2Map map[string]*testapi.CTPRequest) *testapi.CTPv2Request {
	reqs := []*testapi.CTPRequest{}
	for _, v2Req := range reqTov2Map {
		reqs = append(reqs, v2Req)
	}
	return &testapi.CTPv2Request{Requests: reqs}
}

func TestCTPv1Tov2Translation(t *testing.T) {
	ftt.Run("Single Translation", t, func(t *ftt.Test) {
		requests := map[string]*test_platform.Request{
			"r1": getCTPv1Request("board", "model", "board-release/R123.0.0", "suite", "", "", false, false),
		}
		v2RequestMap, _, _ := builders.NewCTPV2FromV1WithCustomManifestFetcher(context.Background(), requests, MockManifestFetcher, &build.State{}).BuildRequest()
		result := ConstructCtpv2Req(v2RequestMap)

		assert.Loosely(t, result.GetRequests(), should.HaveLength(1))
		assert.Loosely(t, result.GetRequests()[0].GetScheduleTargets(), should.HaveLength(1))
		assert.Loosely(t, result.GetRequests()[0].GetScheduleTargets()[0].GetTargets(), should.HaveLength(1))
		assert.Loosely(t, result.GetRequests()[0].GetScheduleTargets()[0].GetTargets()[0].GetHwTarget().GetLegacyHw().GetBoard(), should.Equal("board"))
		assert.Loosely(t, result.GetRequests()[0].GetScheduleTargets()[0].GetTargets()[0].GetHwTarget().GetLegacyHw().GetModel(), should.Equal("model"))
		assert.Loosely(t, result.GetRequests()[0].GetScheduleTargets()[0].GetTargets()[0].GetSwTarget().GetLegacySw().GetBuild(), should.Equal("release"))
		assert.Loosely(t, result.GetRequests()[0].GetScheduleTargets()[0].GetTargets()[0].GetSwTarget().GetLegacySw().GetGcsPath(), should.Equal("gs://chromeos-image-archive/board-release/R123.0.0"))
		assert.Loosely(t, result.GetRequests()[0].GetScheduleTargets()[0].GetTargets()[0].GetSwTarget().GetLegacySw().GetVariant(), should.BeEmpty)
	})

	ftt.Run("Multi Translation, no grouping", t, func(t *ftt.Test) {
		requests := map[string]*test_platform.Request{
			"r1": getCTPv1Request("board", "model", "board-release/R123.0.0", "suite", "", "", false, false),
			"r2": getCTPv1Request("board", "model", "board-release/R124.0.0", "suite", "", "", false, false),
		}
		v2RequestMap, _, _ := builders.NewCTPV2FromV1WithCustomManifestFetcher(context.Background(), requests, MockManifestFetcher, &build.State{}).BuildRequest()
		result := ConstructCtpv2Req(v2RequestMap)

		assert.Loosely(t, result.GetRequests(), should.HaveLength(2))
		assert.Loosely(t, result.GetRequests()[0].GetScheduleTargets(), should.HaveLength(1))
		assert.Loosely(t, result.GetRequests()[1].GetScheduleTargets(), should.HaveLength(1))
		assert.Loosely(t, result.GetRequests()[0].GetScheduleTargets()[0].GetTargets(), should.HaveLength(1))
		assert.Loosely(t, result.GetRequests()[1].GetScheduleTargets()[0].GetTargets(), should.HaveLength(1))

		request1 := result.GetRequests()[0]
		request2 := result.GetRequests()[1]
		// Because the original requests map is flattened into a slice, the
		// order of the result.GetRequests() slice is nondeterministic. The
		// following code ensures the values of request1 and request2 is
		// deterministic by always assigning the request with version R123 to
		// request1 and the request with version R124 to request2.
		if request1.GetScheduleTargets()[0].GetTargets()[0].GetSwTarget().GetLegacySw().GetGcsPath() != "gs://chromeos-image-archive/board-release/R123.0.0" {
			swap := request1
			request1 = request2
			request2 = swap
		}
		assert.Loosely(t, request1.GetSchedulerInfo().GetScheduler(), should.Equal(testapi.SchedulerInfo_SCHEDUKE))
		assert.Loosely(t, request2.GetSchedulerInfo().GetScheduler(), should.Equal(testapi.SchedulerInfo_SCHEDUKE))
		target1 := request1.GetScheduleTargets()[0].GetTargets()[0]
		target2 := request2.GetScheduleTargets()[0].GetTargets()[0]
		assert.Loosely(t, target1.GetSwTarget().GetLegacySw().GetGcsPath(), should.Equal("gs://chromeos-image-archive/board-release/R123.0.0"))
		assert.Loosely(t, target2.GetSwTarget().GetLegacySw().GetGcsPath(), should.Equal("gs://chromeos-image-archive/board-release/R124.0.0"))
	})

	ftt.Run("Multi Translation, grouping", t, func(t *ftt.Test) {
		requests := map[string]*test_platform.Request{
			"r1": getCTPv1Request("board", "model", "board-release/R123.0.0", "suite", "", "", false, false),
			"r2": getCTPv1Request("board", "model2", "board-release/R123.0.0", "suite", "", "", false, false),
		}
		v2RequestMap, _, _ := builders.NewCTPV2FromV1WithCustomManifestFetcher(context.Background(), requests, MockManifestFetcher, &build.State{}).BuildRequest()
		result := ConstructCtpv2Req(v2RequestMap)

		assert.Loosely(t, result.GetRequests(), should.HaveLength(1))
		assert.Loosely(t, result.GetRequests()[0].GetScheduleTargets(), should.HaveLength(2))
		assert.Loosely(t, result.GetRequests()[0].GetScheduleTargets()[0].GetTargets(), should.HaveLength(1))
		assert.Loosely(t, result.GetRequests()[0].GetScheduleTargets()[1].GetTargets(), should.HaveLength(1))
		target1 := result.GetRequests()[0].GetScheduleTargets()[0].GetTargets()[0]
		target2 := result.GetRequests()[0].GetScheduleTargets()[1].GetTargets()[0]
		if target1.GetHwTarget().GetLegacyHw().GetModel() != "model" {
			swap := target1
			target1 = target2
			target2 = swap
		}
		assert.Loosely(t, target1.GetHwTarget().GetLegacyHw().GetModel(), should.Equal("model"))
		assert.Loosely(t, target2.GetHwTarget().GetLegacyHw().GetModel(), should.Equal("model2"))
	})

	ftt.Run("Multi Translation, grouping, including public manifest", t, func(t *ftt.Test) {
		requests := map[string]*test_platform.Request{
			"r1": getCTPv1Request("board", "model", "public-manifest-release/R123.0.0", "suite", "", "", false, false),
			"r2": getCTPv1Request("board", "model2", "board-release/R123.0.0", "suite", "", "", false, false),
		}
		v2RequestMap, _, _ := builders.NewCTPV2FromV1WithCustomManifestFetcher(context.Background(), requests, MockManifestFetcher, &build.State{}).BuildRequest()
		result := ConstructCtpv2Req(v2RequestMap)

		assert.Loosely(t, result.GetRequests(), should.HaveLength(2))
		assert.Loosely(t, result.GetRequests()[0].GetScheduleTargets(), should.HaveLength(1))
		assert.Loosely(t, result.GetRequests()[1].GetScheduleTargets(), should.HaveLength(1))
		assert.Loosely(t, result.GetRequests()[0].GetScheduleTargets()[0].GetTargets(), should.HaveLength(1))
		assert.Loosely(t, result.GetRequests()[1].GetScheduleTargets()[0].GetTargets(), should.HaveLength(1))
		target1 := result.GetRequests()[0].GetScheduleTargets()[0].GetTargets()[0]
		target2 := result.GetRequests()[1].GetScheduleTargets()[0].GetTargets()[0]
		if target1.GetSwTarget().GetLegacySw().GetGcsPath() != "gs://chromeos-image-archive/public-manifest-release/R123.0.0" {
			swap := target1
			target1 = target2
			target2 = swap
		}
		assert.Loosely(t, target1.GetSwTarget().GetLegacySw().GetGcsPath(), should.Equal("gs://chromeos-image-archive/public-manifest-release/R123.0.0"))
		assert.Loosely(t, target2.GetSwTarget().GetLegacySw().GetGcsPath(), should.Equal("gs://chromeos-image-archive/board-release/R123.0.0"))
	})

	ftt.Run("Multi Translation, grouping, 3d, including public manifest", t, func(t *ftt.Test) {
		requests := map[string]*test_platform.Request{
			"r1": getCTPv1Request("board", "model", "public-manifest-release/R123.0.0", "suite", "", "", false, true),
			"r2": getCTPv1Request("board", "model2", "board-release/R123.0.0", "suite", "", "", false, true),
		}
		v2RequestMap, _, _ := builders.NewCTPV2FromV1WithCustomManifestFetcher(context.Background(), requests, MockManifestFetcher, &build.State{}).BuildRequest()
		result := ConstructCtpv2Req(v2RequestMap)

		assert.Loosely(t, result.GetRequests(), should.HaveLength(1))
		assert.Loosely(t, result.GetRequests()[0].GetScheduleTargets(), should.HaveLength(2))
		assert.Loosely(t, result.GetRequests()[0].GetScheduleTargets()[0].GetTargets(), should.HaveLength(1))
		assert.Loosely(t, result.GetRequests()[0].GetScheduleTargets()[1].GetTargets(), should.HaveLength(1))
		target1 := result.GetRequests()[0].GetScheduleTargets()[0].GetTargets()[0]
		target2 := result.GetRequests()[0].GetScheduleTargets()[1].GetTargets()[0]
		if target1.GetHwTarget().GetLegacyHw().GetModel() != "model" {
			swap := target1
			target1 = target2
			target2 = swap
		}
		assert.Loosely(t, target1.GetHwTarget().GetLegacyHw().GetModel(), should.Equal("model"))
		assert.Loosely(t, target2.GetHwTarget().GetLegacyHw().GetModel(), should.Equal("model2"))
	})
}

func TestGetBuildType(t *testing.T) {
	ftt.Run("GetBuildType", t, func(t *ftt.Test) {
		buildType := builders.GetBuildType(getChromeosSoftwareDeps("board-release/R123.0.0"))
		assert.Loosely(t, buildType, should.Equal("release"))
	})

	ftt.Run("GetBuildType remove postfix", t, func(t *ftt.Test) {
		buildType := builders.GetBuildType(getChromeosSoftwareDeps("board-release-main/R123.0.0"))
		assert.Loosely(t, buildType, should.Equal("release"))
	})

	ftt.Run("GetBuildType empty string", t, func(t *ftt.Test) {
		buildType := builders.GetBuildType(getChromeosSoftwareDeps(""))
		assert.Loosely(t, buildType, should.BeEmpty)
	})
}

func TestGetVariant(t *testing.T) {
	ftt.Run("GetVariant", t, func(t *ftt.Test) {
		variant := builders.GetVariant(getChromeosSoftwareDeps("board-variant-arc-release/R123.0.0"))
		assert.Loosely(t, variant, should.Equal("variant-arc"))
	})

	ftt.Run("GetVariant no variant", t, func(t *ftt.Test) {
		variant := builders.GetVariant(getChromeosSoftwareDeps("board-release/R123.0.0"))
		assert.Loosely(t, variant, should.BeEmpty)
	})

	ftt.Run("GetVariant remove postfix", t, func(t *ftt.Test) {
		variant := builders.GetVariant(getChromeosSoftwareDeps("board-variant-arc-release-main/R123.0.0"))
		assert.Loosely(t, variant, should.Equal("variant-arc"))
	})

	ftt.Run("GetVariant remove prefix", t, func(t *ftt.Test) {
		variant := builders.GetVariant(getChromeosSoftwareDeps("staging-board-variant-arc-release/R123.0.0"))
		assert.Loosely(t, variant, should.Equal("variant-arc"))
	})

	ftt.Run("GetVariant remove prefix and postfix", t, func(t *ftt.Test) {
		variant := builders.GetVariant(getChromeosSoftwareDeps("dev-board-variant-arc-release-main/R123.0.0"))
		assert.Loosely(t, variant, should.Equal("variant-arc"))
	})

	// Currently this incorrectly pulls the variant for the example below; but without better variant info,
	// this appears to be difficult to safely solve. Leaving this test in here as an example of something
	// to look into long term.
	// Convey("GetVariant remove prefix and postfix", t, func() {
	// 	variant := builders.GetVariant(getChromeosSoftwareDeps("staging-rex-release-R124-15823.B/R124-15823.9.0-8752476513443194785"))
	// 	So(variant, ShouldEqual, "")
	// })

	ftt.Run("GetVariant empty string", t, func(t *ftt.Test) {
		variant := builders.GetVariant(getChromeosSoftwareDeps(""))
		assert.Loosely(t, variant, should.BeEmpty)
	})
}

func TestCTP2Grouping(t *testing.T) {
	ftt.Run("Same build, same suite, diff boards", t, func(t *ftt.Test) {
		groupings, _ := builders.GroupEligibleV2Requests(context.Background(), []*builders.V2WithKey{
			getCTPv2WithKeyRequest("board1", "model1", "release", "board1-release/R123.0.0", "", "suite1", ""),
			getCTPv2WithKeyRequest("board2", "model1", "release", "board1-release/R123.0.0", "", "suite1", ""),
		})

		assert.Loosely(t, groupings, should.HaveLength(1))
		assert.Loosely(t, groupings[0].V2.GetScheduleTargets(), should.HaveLength(2))
		assert.Loosely(t, groupings[0].V2.GetScheduleTargets()[0].GetTargets(), should.HaveLength(1))
		assert.Loosely(t, groupings[0].V2.GetScheduleTargets()[1].GetTargets(), should.HaveLength(1))
		assert.Loosely(t, groupings[0].V2.GetScheduleTargets()[0].GetTargets()[0].GetSwTarget().GetLegacySw().GetGcsPath(),
			should.Equal(
				groupings[0].V2.GetScheduleTargets()[1].GetTargets()[0].GetSwTarget().GetLegacySw().GetGcsPath()))
	})

	ftt.Run("Same build, diff suite", t, func(t *ftt.Test) {
		groupings, _ := builders.GroupEligibleV2Requests(context.Background(), []*builders.V2WithKey{
			getCTPv2WithKeyRequest("board1", "model1", "release", "board1-release/R123.0.0", "", "suite1", ""),
			getCTPv2WithKeyRequest("board1", "model1", "release", "board1-release/R123.0.0", "", "suite2", ""),
		})

		assert.Loosely(t, groupings, should.HaveLength(2))
		assert.Loosely(t, groupings[0].V2.GetScheduleTargets(), should.HaveLength(1))
		assert.Loosely(t, groupings[1].V2.GetScheduleTargets(), should.HaveLength(1))
		assert.Loosely(t, groupings[0].V2.GetSuiteRequest().GetTestSuite().GetName(),
			should.NotEqual(
				groupings[1].V2.GetSuiteRequest().GetTestSuite().GetName()))
	})

	ftt.Run("Diff build, same suite", t, func(t *ftt.Test) {
		groupings, _ := builders.GroupEligibleV2Requests(context.Background(), []*builders.V2WithKey{
			getCTPv2WithKeyRequest("board1", "model1", "release", "board1-release/R123.0.0", "", "suite1", ""),
			getCTPv2WithKeyRequest("board1", "model1", "release", "board1-release/R124.0.0", "", "suite1", ""),
		})

		assert.Loosely(t, groupings, should.HaveLength(2))
		assert.Loosely(t, groupings[0].V2.GetScheduleTargets(), should.HaveLength(1))
		assert.Loosely(t, groupings[1].V2.GetScheduleTargets(), should.HaveLength(1))
		assert.Loosely(t, groupings[0].V2.GetScheduleTargets()[0].GetTargets(), should.HaveLength(1))
		assert.Loosely(t, groupings[1].V2.GetScheduleTargets()[0].GetTargets(), should.HaveLength(1))
		assert.Loosely(t, groupings[0].V2.GetScheduleTargets()[0].GetTargets()[0].GetSwTarget().GetLegacySw().GetGcsPath(),
			should.NotEqual(
				groupings[1].V2.GetScheduleTargets()[0].GetTargets()[0].GetSwTarget().GetLegacySw().GetGcsPath()))
	})

	ftt.Run("Diff build, diff suite", t, func(t *ftt.Test) {
		groupings, _ := builders.GroupEligibleV2Requests(context.Background(), []*builders.V2WithKey{
			getCTPv2WithKeyRequest("board1", "model1", "release", "board1-release/R123.0.0", "", "suite1", ""),
			getCTPv2WithKeyRequest("board1", "model1", "release", "board1-release/R124.0.0", "", "suite2", ""),
		})

		assert.Loosely(t, groupings, should.HaveLength(2))
		assert.Loosely(t, groupings[0].V2.GetScheduleTargets(), should.HaveLength(1))
		assert.Loosely(t, groupings[1].V2.GetScheduleTargets(), should.HaveLength(1))
		assert.Loosely(t, groupings[0].V2.GetScheduleTargets()[0].GetTargets(), should.HaveLength(1))
		assert.Loosely(t, groupings[1].V2.GetScheduleTargets()[0].GetTargets(), should.HaveLength(1))
		assert.Loosely(t, groupings[0].V2.GetScheduleTargets()[0].GetTargets()[0].GetSwTarget().GetLegacySw().GetGcsPath(),
			should.NotEqual(
				groupings[1].V2.GetScheduleTargets()[0].GetTargets()[0].GetSwTarget().GetLegacySw().GetGcsPath()))
		assert.Loosely(t, groupings[0].V2.GetSuiteRequest().GetTestSuite().GetName(),
			should.NotEqual(
				groupings[1].V2.GetSuiteRequest().GetTestSuite().GetName()))
	})
}

func getCTPv1Request(board, model, build, suite, testArgs string, analyticsName string, runWithQs bool, is3d bool) *test_platform.Request {
	return &test_platform.Request{
		TestPlan: &test_platform.Request_TestPlan{
			Suite: []*test_platform.Request_Suite{
				{
					Name:     suite,
					TestArgs: testArgs,
				},
			},
		},
		Params: &test_platform.Request_Params{
			Decorations: &test_platform.Request_Params_Decorations{
				Tags: []string{
					"label-pool:schedukeTest",

					fmt.Sprintf("analytics_name:%s", analyticsName),
				},
			},
			Scheduling: &test_platform.Request_Params_Scheduling{
				QsAccount: "qs_account",
			},
			HardwareAttributes: &test_platform.Request_Params_HardwareAttributes{
				Model: model,
			},
			SoftwareAttributes: &test_platform.Request_Params_SoftwareAttributes{
				BuildTarget: &chromiumos.BuildTarget{
					Name: board,
				},
			},
			SoftwareDependencies: []*test_platform.Request_Params_SoftwareDependency{
				{
					Dep: &test_platform.Request_Params_SoftwareDependency_ChromeosBuild{
						ChromeosBuild: build,
					},
				},
			},
			RunCtpv2WithQs: runWithQs,
			DddSuite:       is3d,
		},
	}
}

func getCTPv2WithKeyRequest(board, model, build, gcsPath, variant, suite, testArgs string) *builders.V2WithKey {
	ctpReq := &testapi.CTPRequest{
		Pool: "schedukeTest",
		SchedulerInfo: &testapi.SchedulerInfo{
			Scheduler: testapi.SchedulerInfo_PRINT_REQUEST_ONLY,
			QsAccount: "qs_account",
		},
		SuiteRequest: &testapi.SuiteRequest{
			SuiteRequest: &testapi.SuiteRequest_TestSuite{
				TestSuite: &testapi.TestSuite{
					Name: suite,
					Spec: &testapi.TestSuite_TestCaseTagCriteria_{
						TestCaseTagCriteria: &testapi.TestSuite_TestCaseTagCriteria{
							Tags: []string{"suite:" + suite},
						},
					},
				},
			},
			TestArgs: testArgs,
		},
		ScheduleTargets: []*testapi.ScheduleTargets{
			{
				Targets: []*testapi.Targets{
					{
						HwTarget: &testapi.HWTarget{
							Target: &testapi.HWTarget_LegacyHw{
								LegacyHw: &testapi.LegacyHW{
									Board: board,
									Model: model,
								},
							},
						},
						SwTarget: &testapi.SWTarget{
							SwTarget: &testapi.SWTarget_LegacySw{
								LegacySw: &testapi.LegacySW{
									Build:   build,
									GcsPath: "gs://chromeos-image-archive/" + gcsPath,
									Variant: variant,
									KeyValues: []*testapi.KeyValue{
										{
											Key:   builders.ChromeosBuild,
											Value: gcsPath,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return builders.NewV2WithKey("", ctpReq)
}

func getChromeosSoftwareDeps(chromeosBuild string) []*test_platform.Request_Params_SoftwareDependency {
	return []*test_platform.Request_Params_SoftwareDependency{
		{
			Dep: &test_platform.Request_Params_SoftwareDependency_ChromeosBuild{
				ChromeosBuild: chromeosBuild,
			},
		},
	}
}

// isDDDSuite will return if the suite is to run in ddd.
// For now, use the ddd prefix, but long term will move to a proper flag.
func TestIsDDDSuite(t *testing.T) {
	requests := map[string]*test_platform.Request{
		"r1": getCTPv1Request("board", "model", "board-release/R123.0.0", "suite", "", "ddd_meme", false, false),
		"r2": getCTPv1Request("board", "model", "board-release/R123.0.0", "suite", "", "not_ddd_suite", false, false),
	}
	if builders.IsDDDSuite(requests["r1"]) != true {
		t.Fatalf("Incorrectly determined if a request was for 3d")
	}
	if builders.IsDDDSuite(requests["r2"]) != false {
		t.Fatalf("Incorrectly determined if a request was for 3d")
	}
}
