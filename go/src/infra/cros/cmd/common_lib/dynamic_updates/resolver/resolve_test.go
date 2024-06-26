// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package resolver_test

import (
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	storage_path "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"

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
	Convey("no placeholders", t, func() {
		simpleStr := "Hello, world!"
		resolvedStr := common.ResolvePlaceholders(simpleStr, lookupTable)

		So(resolvedStr, ShouldEqual, simpleStr)
	})

	Convey("empty", t, func() {
		empty := ""
		resolvedStr := common.ResolvePlaceholders(empty, lookupTable)

		So(resolvedStr, ShouldEqual, "")
	})

	Convey("one placeholder", t, func() {
		simplePlaceholder := "This is board: ${board}"
		resolvedStr := common.ResolvePlaceholders(simplePlaceholder, lookupTable)

		So(resolvedStr, ShouldEqual, fmt.Sprintf("This is board: %s", lookupTable["board"]))
	})

	Convey("two placeholders", t, func() {
		twoPlaceholders := "This is board/model: ${board}/${model}"
		resolvedStr := common.ResolvePlaceholders(twoPlaceholders, lookupTable)

		So(resolvedStr, ShouldEqual, fmt.Sprintf("This is board/model: %s/%s", lookupTable["board"], lookupTable["model"]))
	})

	Convey("embedded placeholder", t, func() {
		embeddedPlaceholder := "${${board}_${model}_dut_id}"
		resolvedStr := common.ResolvePlaceholders(embeddedPlaceholder, lookupTable)
		// Extra resolution to support one layer of embedded placeholders.
		resolvedStr = common.ResolvePlaceholders(resolvedStr, lookupTable)

		So(resolvedStr, ShouldEqual, lookupTable["dedede_helion_dut_id"])
	})
}

func TestResolveDynamicUpdate(t *testing.T) {
	Convey("placeholders", t, func() {
		dynamicUpdate := &api.UserDefinedDynamicUpdate{
			UpdateAction: getAppendUpdateAction(getProvisionTask("gs://${gcsBasePath}/${board}/${buildNumber}")),
		}

		resolvedUpdate, err := Resolve(dynamicUpdate, lookupTable)
		So(err, ShouldBeNil)
		So(resolvedUpdate.UpdateAction.GetInsert().Task.GetProvision().InstallRequest.ImagePath.Path,
			ShouldEqual,
			fmt.Sprintf("gs://%s/%s/%s", lookupTable["gcsBasePath"], lookupTable["board"], lookupTable["buildNumber"]))
	})

	Convey("no placeholders", t, func() {
		dynamicUpdate := &api.UserDefinedDynamicUpdate{
			UpdateAction: getAppendUpdateAction(getProvisionTask("no_placeholders_here")),
		}

		resolvedUpdate, err := Resolve(dynamicUpdate, lookupTable)
		So(err, ShouldBeNil)
		So(resolvedUpdate.UpdateAction.GetInsert().Task.GetProvision().InstallRequest.ImagePath.Path,
			ShouldEqual,
			"no_placeholders_here")
	})

	Convey("embedded placeholders", t, func() {
		dynamicUpdate := &api.UserDefinedDynamicUpdate{
			UpdateAction: getAppendUpdateAction(getProvisionTask("gs://${gcsBasePath}/${${board}_${model}_dut_id}/${buildNumber}")),
		}

		resolvedUpdate, err := Resolve(dynamicUpdate, lookupTable)
		So(err, ShouldBeNil)
		So(resolvedUpdate.UpdateAction.GetInsert().Task.GetProvision().InstallRequest.ImagePath.Path,
			ShouldEqual,
			fmt.Sprintf("gs://%s/%s/%s", lookupTable["gcsBasePath"], lookupTable["dedede_helion_dut_id"], lookupTable["buildNumber"]))
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
