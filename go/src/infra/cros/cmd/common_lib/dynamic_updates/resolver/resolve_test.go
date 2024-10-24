// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package resolver_test

import (
	"fmt"
	"testing"

	storage_path "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/cros/cmd/common_lib/common"
	. "infra/cros/cmd/common_lib/dynamic_updates/resolver"
)

var lookupTable DynamicPlaceholderLookup = DynamicPlaceholderLookup{
	"board":                "dedede",
	"model":                "helion",
	"dedede_helion_dut_id": "chromeos8-row5",
	"buildNumber":          "R123-0.0",
	"gcsBasePath":          "chromeos-image-archive",
}

func TestPlaceholderResolution(t *testing.T) {
	ftt.Run("no placeholders", t, func(t *ftt.Test) {
		simpleStr := "Hello, world!"
		resolvedStr := common.ResolvePlaceholders(simpleStr, lookupTable)

		assert.Loosely(t, resolvedStr, should.Equal(simpleStr))
	})

	ftt.Run("empty", t, func(t *ftt.Test) {
		empty := ""
		resolvedStr := common.ResolvePlaceholders(empty, lookupTable)

		assert.Loosely(t, resolvedStr, should.BeEmpty)
	})

	ftt.Run("one placeholder", t, func(t *ftt.Test) {
		simplePlaceholder := "This is board: ${board}"
		resolvedStr := common.ResolvePlaceholders(simplePlaceholder, lookupTable)

		assert.Loosely(t, resolvedStr, should.Equal(fmt.Sprintf("This is board: %s", lookupTable["board"])))
	})

	ftt.Run("two placeholders", t, func(t *ftt.Test) {
		twoPlaceholders := "This is board/model: ${board}/${model}"
		resolvedStr := common.ResolvePlaceholders(twoPlaceholders, lookupTable)

		assert.Loosely(t, resolvedStr, should.Equal(fmt.Sprintf("This is board/model: %s/%s", lookupTable["board"], lookupTable["model"])))
	})

	ftt.Run("embedded placeholder", t, func(t *ftt.Test) {
		embeddedPlaceholder := "${${board}_${model}_dut_id}"
		resolvedStr := common.ResolvePlaceholders(embeddedPlaceholder, lookupTable)
		// Extra resolution to support one layer of embedded placeholders.
		resolvedStr = common.ResolvePlaceholders(resolvedStr, lookupTable)

		assert.Loosely(t, resolvedStr, should.Equal(lookupTable["dedede_helion_dut_id"]))
	})
}

func TestResolveDynamicUpdate(t *testing.T) {
	ftt.Run("placeholders", t, func(t *ftt.Test) {
		dynamicUpdate := &api.UserDefinedDynamicUpdate{
			UpdateAction: getAppendUpdateAction(getProvisionTask("gs://${gcsBasePath}/${board}/${buildNumber}")),
		}

		resolvedUpdate, err := Resolve(dynamicUpdate, lookupTable)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resolvedUpdate.UpdateAction.GetInsert().Task.GetProvision().InstallRequest.ImagePath.Path,
			should.Equal(
				fmt.Sprintf("gs://%s/%s/%s", lookupTable["gcsBasePath"], lookupTable["board"], lookupTable["buildNumber"])))
	})

	ftt.Run("no placeholders", t, func(t *ftt.Test) {
		dynamicUpdate := &api.UserDefinedDynamicUpdate{
			UpdateAction: getAppendUpdateAction(getProvisionTask("no_placeholders_here")),
		}

		resolvedUpdate, err := Resolve(dynamicUpdate, lookupTable)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resolvedUpdate.UpdateAction.GetInsert().Task.GetProvision().InstallRequest.ImagePath.Path,
			should.Equal(
				"no_placeholders_here"))
	})

	ftt.Run("embedded placeholders", t, func(t *ftt.Test) {
		dynamicUpdate := &api.UserDefinedDynamicUpdate{
			UpdateAction: getAppendUpdateAction(getProvisionTask("gs://${gcsBasePath}/${${board}_${model}_dut_id}/${buildNumber}")),
		}

		resolvedUpdate, err := Resolve(dynamicUpdate, lookupTable)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resolvedUpdate.UpdateAction.GetInsert().Task.GetProvision().InstallRequest.ImagePath.Path,
			should.Equal(
				fmt.Sprintf("gs://%s/%s/%s", lookupTable["gcsBasePath"], lookupTable["dedede_helion_dut_id"], lookupTable["buildNumber"])))
	})
}

func getAppendUpdateAction(task *api.CrosTestRunnerDynamicRequest_Task) *api.UpdateAction {
	return &api.UpdateAction{
		Action: &api.UpdateAction_Insert_{
			Insert: &api.UpdateAction_Insert{
				Task: task,
			},
		},
	}
}

func getProvisionTask(installPath string) *api.CrosTestRunnerDynamicRequest_Task {
	return &api.CrosTestRunnerDynamicRequest_Task{
		Task: &api.CrosTestRunnerDynamicRequest_Task_Provision{
			Provision: &api.ProvisionTask{
				InstallRequest: &api.InstallRequest{
					ImagePath: &storage_path.StoragePath{
						HostType: storage_path.StoragePath_GS,
						Path:     installPath,
					},
				},
			},
		},
	}
}
