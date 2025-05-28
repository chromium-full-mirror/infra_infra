// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package generators_test

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	libapi "go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/builders"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/generators"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/helpers"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/interfaces"
)

func TestGeneric(t *testing.T) {
	dynamicUpdates := []*api.UserDefinedDynamicUpdate{}
	ftt.Run("Generic Generation", t, func(t *ftt.Test) {
		generator := generators.NewGenericTaskGenerator(
			common.NewTaskIdentifier("generic-task").Id,
			"user-container-id",
			[]*builders.ContainerBuilder{
				builders.NewContainerBuilder(
					"user-container-id",
					"",
					"us-docker.pkg.dev/cros-registry/test-service/container-name@sha:123456",
					"/tmp/user-container",
					"user-container server -port 0",
				),
			},
			map[string]interfaces.InsertInstructionGetter{
				"beforeProvision": common.InsertActionWrapper(
					api.UpdateAction_Insert_PREPEND,
					common.FindFirst(api.FocalTaskFinder_PROVISION),
				),
				"afterProvision": common.InsertActionWrapper(
					api.UpdateAction_Insert_APPEND,
					common.FindFirst(api.FocalTaskFinder_PROVISION),
				),
			},
			&interfaces.GenericTaskMessageRequest{
				InsertionId: "beforeProvision",
			},
			&interfaces.GenericTaskMessageRequest{
				InsertionId: "beforeProvision",
			},
			&interfaces.GenericTaskMessageRequest{
				InsertionId: "afterProvision",
			},
		)

		err := libapi.AppendUserDefinedDynamicUpdates(&dynamicUpdates, generator.Generate)

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, dynamicUpdates, should.HaveLength(2))
		assert.Loosely(t, reflect.TypeOf(dynamicUpdates[0].FocalTaskFinder.Finder), should.Equal(reflect.TypeOf((*api.FocalTaskFinder_First_)(nil))))
		assert.Loosely(t, dynamicUpdates[0].FocalTaskFinder.GetFirst().TaskType, should.Equal(api.FocalTaskFinder_PROVISION))
		assert.Loosely(t, reflect.TypeOf(dynamicUpdates[0].UpdateAction.Action), should.Equal(reflect.TypeOf((*api.UpdateAction_Insert_)(nil))))
		assert.Loosely(t, dynamicUpdates[0].UpdateAction.GetInsert().InsertType, should.Equal(api.UpdateAction_Insert_PREPEND))
		assert.Loosely(t, dynamicUpdates[0].UpdateAction.GetInsert().Task, should.NotBeNil)
		assert.Loosely(t, dynamicUpdates[0].UpdateAction.GetInsert().Task.GetGeneric().StartRequest, should.NotBeNil)
		assert.Loosely(t, dynamicUpdates[0].UpdateAction.GetInsert().Task.GetGeneric().RunRequest, should.NotBeNil)
		assert.Loosely(t, dynamicUpdates[0].UpdateAction.GetInsert().Task.GetGeneric().StopRequest, should.BeNil)

		assert.Loosely(t, reflect.TypeOf(dynamicUpdates[1].FocalTaskFinder.Finder), should.Equal(reflect.TypeOf((*api.FocalTaskFinder_First_)(nil))))
		assert.Loosely(t, dynamicUpdates[1].FocalTaskFinder.GetFirst().TaskType, should.Equal(api.FocalTaskFinder_PROVISION))
		assert.Loosely(t, reflect.TypeOf(dynamicUpdates[1].UpdateAction.Action), should.Equal(reflect.TypeOf((*api.UpdateAction_Insert_)(nil))))
		assert.Loosely(t, dynamicUpdates[1].UpdateAction.GetInsert().InsertType, should.Equal(api.UpdateAction_Insert_APPEND))
		assert.Loosely(t, dynamicUpdates[1].UpdateAction.GetInsert().Task, should.NotBeNil)
		assert.Loosely(t, dynamicUpdates[1].UpdateAction.GetInsert().Task.GetGeneric().StartRequest, should.BeNil)
		assert.Loosely(t, dynamicUpdates[1].UpdateAction.GetInsert().Task.GetGeneric().RunRequest, should.BeNil)
		assert.Loosely(t, dynamicUpdates[1].UpdateAction.GetInsert().Task.GetGeneric().StopRequest, should.NotBeNil)
	})
}

func TestProvision(t *testing.T) {
	dynamicUpdates := []*api.UserDefinedDynamicUpdate{}
	ftt.Run("Provision Generation", t, func(t *ftt.Test) {
		generator := generators.NewProvisionTaskGenerator(
			common.NewTaskIdentifier(common.CrosProvision).AddDeviceId(common.NewPrimaryDeviceIdentifier()).Id,
			"provision-container-id",
			common.NewPrimaryDeviceIdentifier().Id,
			[]*builders.ContainerBuilder{
				builders.NewContainerBuilder(
					"provision-container-id",
					"cros-provision",
					"",
					"/tmp/provision",
					"cros-provision server -port 0",
				),
			},
			common.InsertActionWrapper(
				api.UpdateAction_Insert_REPLACE,
				common.FindFirst(api.FocalTaskFinder_PROVISION),
			),
			helpers.DefaultProvisionStartUpRequest(common.NewPrimaryDeviceIdentifier()),
			helpers.DefaultCrosProvisionInstallRequest(common.SetPlaceholder("installPath")),
		)
		err := libapi.AppendUserDefinedDynamicUpdates(&dynamicUpdates, generator.Generate)

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, dynamicUpdates, should.HaveLength(1))
		assert.Loosely(t, reflect.TypeOf(dynamicUpdates[0].FocalTaskFinder.Finder), should.Equal(reflect.TypeOf((*api.FocalTaskFinder_First_)(nil))))
		assert.Loosely(t, dynamicUpdates[0].FocalTaskFinder.GetFirst().TaskType, should.Equal(api.FocalTaskFinder_PROVISION))
		assert.Loosely(t, reflect.TypeOf(dynamicUpdates[0].UpdateAction.Action), should.Equal(reflect.TypeOf((*api.UpdateAction_Insert_)(nil))))
		assert.Loosely(t, dynamicUpdates[0].UpdateAction.GetInsert().InsertType, should.Equal(api.UpdateAction_Insert_REPLACE))
		assert.Loosely(t, dynamicUpdates[0].UpdateAction.GetInsert().Task, should.NotBeNil)
		assert.Loosely(t, dynamicUpdates[0].UpdateAction.GetInsert().Task.GetProvision().StartupRequest, should.NotBeNil)
		assert.Loosely(t, dynamicUpdates[0].UpdateAction.GetInsert().Task.GetProvision().InstallRequest, should.NotBeNil)
	})
}

func TestModify(t *testing.T) {
	dynamicUpdates := []*api.UserDefinedDynamicUpdate{}
	ftt.Run("Modify Generation", t, func(t *ftt.Test) {
		generator := generators.NewModifyGenerator(
			common.FindFirst(api.FocalTaskFinder_TEST),
		)
		assert.Loosely(t, generator.AddModification(
			structpb.NewStringValue("cros-test-cq-light"),
			map[string]string{
				"orderedContainerRequests.0.containerImageKey": "value",
			},
		), should.BeNil)
		assert.Loosely(t, generator.AddModification(
			&labapi.IpEndpoint{
				Address: "devboard-address",
				Port:    12345,
			},
			map[string]string{
				"test.testRequest.primary.devboardServer": "",
			},
		), should.BeNil)

		err := libapi.AppendUserDefinedDynamicUpdates(&dynamicUpdates, generator.Generate)

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, dynamicUpdates, should.HaveLength(1))
		assert.Loosely(t, reflect.TypeOf(dynamicUpdates[0].UpdateAction.Action), should.Equal(reflect.TypeOf((*api.UpdateAction_Modify_)(nil))))
		assert.Loosely(t, dynamicUpdates[0].UpdateAction.GetModify().Modifications, should.HaveLength(2))
		stringValue := &structpb.Value{}
		err = dynamicUpdates[0].UpdateAction.GetModify().GetModifications()[0].GetPayload().UnmarshalTo(stringValue)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, stringValue.GetStringValue(), should.Equal("cros-test-cq-light"))
		assert.Loosely(t, dynamicUpdates[0].UpdateAction.GetModify().GetModifications()[0].GetInstructions()["orderedContainerRequests.0.containerImageKey"], should.Equal("value"))
		ipEndpoint := &labapi.IpEndpoint{}
		err = dynamicUpdates[0].UpdateAction.GetModify().GetModifications()[1].GetPayload().UnmarshalTo(ipEndpoint)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, ipEndpoint.GetAddress(), should.Equal("devboard-address"))
		assert.Loosely(t, ipEndpoint.GetPort(), should.Equal(12345))
		assert.Loosely(t, dynamicUpdates[0].UpdateAction.GetModify().GetModifications()[1].GetInstructions()["test.testRequest.primary.devboardServer"], should.BeEmpty)
	})
}
