// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"

	api "infra/unifiedfleet/api/v1/rpc"
	"infra/unifiedfleet/app/model/configuration"
)

func TestGetPublicChromiumTestStatus(t *testing.T) {
	t.Parallel()
	ctx := auth.WithState(testingContext(), &authtest.FakeState{
		Identity:       "user:abc@def.com",
		IdentityGroups: []string{"public-chromium-in-chromeos-builders"},
	})
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	configuration.AddPublicBoardModelData(ctx, "eve", []string{"eve"}, false)
	ftt.Run("Check Fleet Policy For Tests", t, func(t *ftt.Test) {
		t.Run("happy path", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName:  "tast.lacros",
				Board:     "eve",
				Model:     "eve",
				Image:     "eve-public/R105-14988.0.0",
				QsAccount: "chromium",
			}

			res, err := tf.Fleet.CheckFleetTestsPolicy(ctx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.TestStatus.Code, should.Equal(api.TestStatus_OK))
		})
		t.Run("Private board", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "tast.lacros",
				Board:    "private",
				Model:    "eve",
				Image:    "R100-14495.0.0-rc1",
			}

			res, err := tf.Fleet.CheckFleetTestsPolicy(ctx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.TestStatus.Code, should.Equal(api.TestStatus_NOT_A_PUBLIC_BOARD))
		})
		t.Run("Private model", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "tast.lacros",
				Board:    "eve",
				Model:    "private",
				Image:    "R100-14495.0.0-rc1",
			}

			res, err := tf.Fleet.CheckFleetTestsPolicy(ctx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.TestStatus.Code, should.Equal(api.TestStatus_NOT_A_PUBLIC_MODEL))
		})
		t.Run("Non allowlisted image", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "tast.lacros",
				Board:    "eve",
				Model:    "eve",
				Image:    "invalid",
			}

			res, err := tf.Fleet.CheckFleetTestsPolicy(ctx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.TestStatus.Code, should.Equal(api.TestStatus_NOT_A_PUBLIC_IMAGE))
		})
		t.Run("Private test name and public auth group member", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "private",
				Board:    "eve",
				Model:    "eve",
				Image:    "R100-14495.0.0-rc1",
			}

			res, err := tf.Fleet.CheckFleetTestsPolicy(ctx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.TestStatus.Code, should.Equal(api.TestStatus_NOT_A_PUBLIC_TEST))
		})
		t.Run("Public test name and not a public auth group member", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "tast.lacros",
				Board:    "eve",
				Model:    "eve",
				Image:    "R100-14495.0.0-rc1",
			}
			ctx := auth.WithState(testingContext(), &authtest.FakeState{
				Identity: "user:abc@def.com",
			})

			res, err := tf.Fleet.CheckFleetTestsPolicy(ctx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.TestStatus.Code, should.Equal(api.TestStatus_OK))
		})
		t.Run("Private test name and not a public auth group member", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "private",
				Board:    "eve",
				Model:    "eve",
				Image:    "R100-14495.0.0-rc1",
			}
			ctx := auth.WithState(testingContext(), &authtest.FakeState{
				Identity: "user:abc@def.com",
			})

			res, err := tf.Fleet.CheckFleetTestsPolicy(ctx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.TestStatus.Code, should.Equal(api.TestStatus_OK))
		})
		t.Run("Missing Test names", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "",
				Board:    "eve",
				Model:    "eve",
				Image:    "R100-14495.0.0-rc1",
			}

			_, err := tf.Fleet.CheckFleetTestsPolicy(ctx, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Test name cannot be empty"))
		})
		t.Run("Missing Board", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "tast.lacros",
				Model:    "eve",
				Image:    "R100-14495.0.0-rc1",
			}

			_, err := tf.Fleet.CheckFleetTestsPolicy(ctx, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Board cannot be empty"))
		})
		t.Run("Missing Models", func(t *ftt.Test) {
			configuration.AddPublicBoardModelData(ctx, "fakeBoard", []string{"fakeModel"}, true)
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "tast.lacros",
				Board:    "fakeBoard",
				Image:    "R100-14495.0.0-rc1",
			}

			_, err := tf.Fleet.CheckFleetTestsPolicy(ctx, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Model cannot be empty as the specified board has unlaunched models"))
		})
		t.Run("Missing Models - succeeds for boards with only public models", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "tast.lacros",
				Board:    "eve",
				Image:    "R100-14495.0.0-rc1",
			}

			_, err := tf.Fleet.CheckFleetTestsPolicy(ctx, req)
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("Missing Image", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "tast.lacros",
				Board:    "eve",
				Model:    "eve",
			}

			_, err := tf.Fleet.CheckFleetTestsPolicy(ctx, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Image cannot be empty"))
		})
		t.Run("Invalid QsAccount", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName:  "tast.lacros",
				Board:     "eve",
				Model:     "eve",
				Image:     "eve-public/R105-14988.0.0",
				QsAccount: "invalid",
			}

			res, err := tf.Fleet.CheckFleetTestsPolicy(ctx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.TestStatus.Code, should.Equal(api.TestStatus_INVALID_QS_ACCOUNT))
		})
	})
}
