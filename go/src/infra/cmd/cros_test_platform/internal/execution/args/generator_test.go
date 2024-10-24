// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package args contains the logic for assembling all data required for
// creating an individual task request.
package args

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/config"
	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestHasVariant(t *testing.T) {
	ftt.Run("Given a request that build has variant", t, func(t *ftt.Test) {
		ctx := context.Background()
		inv := basicInvocation()
		setTestName(inv, "foo-name")
		var params test_platform.Request_Params
		var dummyWorkerConfig = &config.Config_SkylabWorker{}
		setBuild(&params, "foo-arc-r-postsubmit/R106-12222.0.0")
		setRequestKeyval(&params, "suite", "foo-suite")
		setRequestMaximumDuration(&params, 1000)
		setPrimayDeviceBoard(&params, "foo")
		setPrimayDeviceModel(&params, "model")
		setRunViaCft(&params, true)
		t.Run("when generating a test runner request's args", func(t *ftt.Test) {
			g := Generator{
				Invocation:       inv,
				Params:           &params,
				WorkerConfig:     dummyWorkerConfig,
				ParentRequestUID: "TestPlanRuns/12345678/foo",
			}
			got, err := g.GenerateArgs(ctx)
			assert.Loosely(t, err, should.BeNil)
			t.Run("the container metadata key is correct has variant", func(t *ftt.Test) {
				assert.Loosely(t, got.CFTTestRunnerRequest.PrimaryDut.ContainerMetadataKey, should.Equal("foo-arc-r"))
			})

		})
	})
}

func TestDisplayNameTagsForUnamedRequest(t *testing.T) {
	ftt.Run("Given a request does not specify a display name", t, func(t *ftt.Test) {
		ctx := context.Background()
		inv := basicInvocation()
		setTestName(inv, "foo-name")
		var params test_platform.Request_Params
		var dummyWorkerConfig = &config.Config_SkylabWorker{}
		setBuild(&params, "foo-build")
		setRequestKeyval(&params, "suite", "foo-suite")
		setRequestMaximumDuration(&params, 1000)
		t.Run("when generating a test runner request's args", func(t *ftt.Test) {
			g := Generator{
				Invocation:       inv,
				Params:           &params,
				WorkerConfig:     dummyWorkerConfig,
				ParentRequestUID: "TestPlanRuns/12345678/foo",
			}
			got, err := g.GenerateArgs(ctx)
			assert.Loosely(t, err, should.BeNil)
			t.Run("the display name tag is generated correctly.", func(t *ftt.Test) {
				assert.Loosely(t, got.SwarmingTags, should.Contain("display_name:foo-build/foo-suite/foo-name"))
				assert.Loosely(t, got.ParentRequestUID, should.Equal("TestPlanRuns/12345678/foo"))
			})
		})
	})
}

func TestInventoryLabels(t *testing.T) {
	ftt.Run("Given a request with board and model info", t, func(t *ftt.Test) {
		ctx := context.Background()
		inv := basicInvocation()
		setTestName(inv, "foo-name")
		var params test_platform.Request_Params
		var dummyWorkerConfig = &config.Config_SkylabWorker{}
		setRequestMaximumDuration(&params, 1000)
		setPrimayDeviceBoard(&params, "coral")
		setPrimayDeviceModel(&params, "babytiger")
		t.Run("when generating a test runner request's args", func(t *ftt.Test) {
			g := Generator{
				Invocation:   inv,
				Params:       &params,
				WorkerConfig: dummyWorkerConfig,
			}
			got, err := g.GenerateArgs(ctx)
			assert.Loosely(t, err, should.BeNil)
			t.Run("the SchedulableLabels is generated correctly", func(t *ftt.Test) {
				assert.Loosely(t, *got.SchedulableLabels.Board, should.Equal("coral"))
				assert.Loosely(t, *got.SchedulableLabels.Model, should.Equal("babytiger"))
				assert.Loosely(t, len(got.SecondaryDevicesLabels), should.BeZero)
			})
		})
	})
}

func TestSecondaryDevicesLabels(t *testing.T) {
	ftt.Run("Given a request with secondary devices", t, func(t *ftt.Test) {
		ctx := context.Background()
		inv := basicInvocation()
		setTestName(inv, "foo-name")
		var params test_platform.Request_Params
		var dummyWorkerConfig = &config.Config_SkylabWorker{}
		setRequestMaximumDuration(&params, 1000)
		setSecondaryDevice(&params, "nami", "", "")
		setSecondaryDevice(&params, "coral", "babytiger", "")
		t.Run("when generating a test runner request's args", func(t *ftt.Test) {
			g := Generator{
				Invocation:   inv,
				Params:       &params,
				WorkerConfig: dummyWorkerConfig,
			}
			got, err := g.GenerateArgs(ctx)
			assert.Loosely(t, err, should.BeNil)
			t.Run("the SecondaryDevicesLabels is generated correctly", func(t *ftt.Test) {
				assert.Loosely(t, len(got.SecondaryDevicesLabels), should.Equal(2))
				assert.Loosely(t, *got.SecondaryDevicesLabels[0].Board, should.Equal("nami"))
				assert.Loosely(t, *got.SecondaryDevicesLabels[0].Model, should.BeEmpty)
				assert.Loosely(t, *got.SecondaryDevicesLabels[1].Board, should.Equal("coral"))
				assert.Loosely(t, *got.SecondaryDevicesLabels[1].Model, should.Equal("babytiger"))
			})
		})
	})
}

func TestExperiments(t *testing.T) {
	ftt.Run("Given a request with experiments", t, func(t *ftt.Test) {
		ctx := context.Background()
		inv := basicInvocation()
		setTestName(inv, "foo-name")
		var params test_platform.Request_Params
		var dummyWorkerConfig = &config.Config_SkylabWorker{}
		setRequestMaximumDuration(&params, 1000)
		t.Run("when generating a test runner request's args", func(t *ftt.Test) {
			g := Generator{
				Invocation:   inv,
				Params:       &params,
				WorkerConfig: dummyWorkerConfig,
				Experiments:  []string{"exp1", "exp2"},
			}
			got, err := g.GenerateArgs(ctx)
			assert.Loosely(t, err, should.BeNil)
			t.Run("the experiments field is propogated correctly", func(t *ftt.Test) {
				assert.Loosely(t, len(got.Experiments), should.Equal(2))
				assert.Loosely(t, got.Experiments, should.Resemble([]string{"exp1", "exp2"}))
			})
		})
	})
}

func TestGerritChanges(t *testing.T) {
	ftt.Run("Given Gerrit Changes", t, func(t *ftt.Test) {
		ctx := context.Background()
		inv := basicInvocation()
		setTestName(inv, "foo-name")
		var params test_platform.Request_Params
		var dummyWorkerConfig = &config.Config_SkylabWorker{}
		setRequestMaximumDuration(&params, 1000)
		t.Run("when generating a test runner request's args", func(t *ftt.Test) {
			gc := &bbpb.GerritChange{
				Host:     "a",
				Project:  "b",
				Change:   123,
				Patchset: 1,
			}
			g := Generator{
				Invocation:    inv,
				Params:        &params,
				WorkerConfig:  dummyWorkerConfig,
				GerritChanges: []*bbpb.GerritChange{gc},
			}
			got, err := g.GenerateArgs(ctx)
			assert.Loosely(t, err, should.BeNil)
			t.Run("the GerritChanges are added correctly", func(t *ftt.Test) {
				assert.Loosely(t, len(got.GerritChanges), should.Equal(1))
				assert.Loosely(t, got.GerritChanges, should.Resemble([]*bbpb.GerritChange{gc}))
			})
		})
	})
}

// TestSwarmingPool ensures we correctly pass on the `SwarmingPool` arg
func TestSwarmingPool(t *testing.T) {
	ftt.Run("Given SwarmingPool", t, func(t *ftt.Test) {
		ctx := context.Background()
		inv := basicInvocation()
		setTestName(inv, "foo-name")
		var params test_platform.Request_Params
		var dummyWorkerConfig = &config.Config_SkylabWorker{}
		setRequestMaximumDuration(&params, 1000)
		t.Run("when generating a test runner request's args", func(t *ftt.Test) {
			g := Generator{
				Invocation:   inv,
				Params:       &params,
				WorkerConfig: dummyWorkerConfig,
				SwarmingPool: "OtherPool",
			}
			got, err := g.GenerateArgs(ctx)
			assert.Loosely(t, err, should.BeNil)
			t.Run("the SwarmingPool should be added correctly", func(t *ftt.Test) {
				assert.Loosely(t, got.SwarmingPool, should.Equal("OtherPool"))
			})
		})
	})
}

// TestAndroidProvisionState ensures we correctly pass android provisioning metadata through provision state
func TestAndroidProvisionState(t *testing.T) {
	ftt.Run("Given a request with Android provision metadata in software deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		inv := basicInvocation()
		setTestName(inv, "foo-name")
		var params test_platform.Request_Params
		var dummyWorkerConfig = &config.Config_SkylabWorker{}
		setBuild(&params, "foo-build")
		setRequestKeyval(&params, "suite", "foo-suite")
		setRequestMaximumDuration(&params, 1000)
		setRunViaCft(&params, true)
		setSecondaryDevice(&params, "coral", "babytiger", "")
		t.Run("when generating a cft test runner request's args with nil android provision metadata", func(t *ftt.Test) {
			g := Generator{
				Invocation:       inv,
				Params:           &params,
				WorkerConfig:     dummyWorkerConfig,
				ParentRequestUID: "TestPlanRuns/12345678/foo",
			}
			_, err := g.GenerateArgs(ctx)
			assert.Loosely(t, err, should.NotBeNil)
		})
		params.SecondaryDevices = []*test_platform.Request_Params_SecondaryDevice{}
		setAndroidSecondaryDeviceWithAndroidProvisionMetadata(&params, "androidBoard", "Pixel6", "11", "latest_stable")
		t.Run("when generating a cft test runner request's args with not nil android provision metadata", func(t *ftt.Test) {
			g := Generator{
				Invocation:       inv,
				Params:           &params,
				WorkerConfig:     dummyWorkerConfig,
				ParentRequestUID: "TestPlanRuns/12345678/foo",
			}
			got, err := g.GenerateArgs(ctx)
			assert.Loosely(t, err, should.BeNil)
			t.Run("provision state should have androidProvisionRequestMetadata as provision state when android metadata is passed", func(t *ftt.Test) {

				companionDuts := got.CFTTestRunnerRequest.GetCompanionDuts()
				for _, companionDut := range companionDuts {
					var androidProvisionRequestMetadata testapi.AndroidProvisionRequestMetadata
					err = companionDut.ProvisionState.ProvisionMetadata.UnmarshalTo(&androidProvisionRequestMetadata)
					assert.Loosely(t, err, should.BeNil)
					cipdPackage := &testapi.CIPDPackage{
						AndroidPackage: 1,
						VersionOneof: &testapi.CIPDPackage_Ref{
							Ref: "latest_stable",
						},
					}
					assert.Loosely(t, androidProvisionRequestMetadata.GetAndroidOsImage().GetOsVersion(), should.Equal("11"))
					assert.Loosely(t, androidProvisionRequestMetadata.GetCipdPackages(), should.ContainMatch(cipdPackage))
				}
			})
		})
	})
}

func TestResultConfig(t *testing.T) {
	ftt.Run("Given ResultConfigs Changes", t, func(t *ftt.Test) {
		ctx := context.Background()
		inv := basicInvocation()
		setTestName(inv, "foo-name")
		var params test_platform.Request_Params
		setTestUploadVisibility(&params, test_platform.Request_Params_ResultsUploadConfig_TEST_RESULTS_VISIBILITY_CUSTOM_REALM)
		var dummyWorkerConfig = &config.Config_SkylabWorker{}
		setRequestMaximumDuration(&params, 1000)
		t.Run("when generating a test runner request's args", func(t *ftt.Test) {
			g := Generator{
				Invocation:   inv,
				Params:       &params,
				WorkerConfig: dummyWorkerConfig,
			}
			got, err := g.GenerateArgs(ctx)
			assert.Loosely(t, err, should.BeNil)
			t.Run("the ResultsConfig is added correctly", func(t *ftt.Test) {
				assert.Loosely(t, got.ResultsConfig.Mode, should.Equal(test_platform.Request_Params_ResultsUploadConfig_TEST_RESULTS_VISIBILITY_CUSTOM_REALM))
			})
		})
	})
}

func TestExperiment2s(t *testing.T) {
	ftt.Run("Given a request with testargs", t, func(t *ftt.Test) {
		inv := basicInvocation()
		inv.TestArgs = "foo=bar zoo=topia=="
		setTestName(inv, "foo-name")
		var params test_platform.Request_Params
		var dummyWorkerConfig = &config.Config_SkylabWorker{}

		things := []*testapi.Arg{}
		k1 := &testapi.Arg{
			Flag:  "foo",
			Value: "bar",
		}
		k2 := &testapi.Arg{
			Flag:  "zoo",
			Value: "topia==",
		}

		things = append(things, k1)
		things = append(things, k2)

		setRequestMaximumDuration(&params, 1000)
		t.Run("when generating a test runner request's args", func(t *ftt.Test) {
			g := Generator{
				Invocation:   inv,
				Params:       &params,
				WorkerConfig: dummyWorkerConfig,
			}
			got := g.testargsforCFT()
			t.Run("the testargs field is propogated correctly", func(t *ftt.Test) {
				assert.Loosely(t, got.Args, should.Resemble(things))
			})
		})
	})
}

var testDimsWithDUTStateData = []struct {
	inputDims []string
	wantDims  []string
}{
	{
		inputDims: []string{"foo"},
		wantDims:  []string{"foo", "dut_state:ready"},
	},
	{
		inputDims: []string{"bar", "dut_state:reserved"},
		wantDims:  []string{"bar", "dut_state:reserved"},
	},
	{
		inputDims: []string{"baz", "dut_state:ready"},
		wantDims:  []string{"baz", "dut_state:ready"},
	},
}

func TestDimsWithDUTState(t *testing.T) {
	t.Parallel()
	for _, tt := range testDimsWithDUTStateData {
		tt := tt
		t.Run(fmt.Sprintf("%v", tt.inputDims), func(t *testing.T) {
			t.Parallel()
			gotDims := dimsWithDUTState(tt.inputDims)
			if !reflect.DeepEqual(tt.wantDims, gotDims) {
				t.Errorf("unexpected error: wanted %v, got %v", tt.wantDims, gotDims)
			}
		})
	}
}
