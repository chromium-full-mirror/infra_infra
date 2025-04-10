// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package generators_test

import (
	"reflect"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"google.golang.org/protobuf/types/known/structpb"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	libapi "go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/builders"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/generators"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/helpers"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/interfaces"
)

func TestGeneric(t *testing.T) {
	dynamicUpdates := []*api.UserDefinedDynamicUpdate{}
	Convey("Generic Generation", t, func() {
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

		So(err, ShouldBeNil)
		So(dynamicUpdates, ShouldHaveLength, 2)
		So(reflect.TypeOf(dynamicUpdates[0].FocalTaskFinder.Finder), ShouldEqual, reflect.TypeOf((*api.FocalTaskFinder_First_)(nil)))
		So(dynamicUpdates[0].FocalTaskFinder.GetFirst().TaskType, ShouldEqual, api.FocalTaskFinder_PROVISION)
		So(reflect.TypeOf(dynamicUpdates[0].UpdateAction.Action), ShouldEqual, reflect.TypeOf((*api.UpdateAction_Insert_)(nil)))
		So(dynamicUpdates[0].UpdateAction.GetInsert().InsertType, ShouldEqual, api.UpdateAction_Insert_PREPEND)
		So(dynamicUpdates[0].UpdateAction.GetInsert().Task, ShouldNotBeNil)
		So(dynamicUpdates[0].UpdateAction.GetInsert().Task.GetGeneric().StartRequest, ShouldNotBeNil)
		So(dynamicUpdates[0].UpdateAction.GetInsert().Task.GetGeneric().RunRequest, ShouldNotBeNil)
		So(dynamicUpdates[0].UpdateAction.GetInsert().Task.GetGeneric().StopRequest, ShouldBeNil)

		So(reflect.TypeOf(dynamicUpdates[1].FocalTaskFinder.Finder), ShouldEqual, reflect.TypeOf((*api.FocalTaskFinder_First_)(nil)))
		So(dynamicUpdates[1].FocalTaskFinder.GetFirst().TaskType, ShouldEqual, api.FocalTaskFinder_PROVISION)
		So(reflect.TypeOf(dynamicUpdates[1].UpdateAction.Action), ShouldEqual, reflect.TypeOf((*api.UpdateAction_Insert_)(nil)))
		So(dynamicUpdates[1].UpdateAction.GetInsert().InsertType, ShouldEqual, api.UpdateAction_Insert_APPEND)
		So(dynamicUpdates[1].UpdateAction.GetInsert().Task, ShouldNotBeNil)
		So(dynamicUpdates[1].UpdateAction.GetInsert().Task.GetGeneric().StartRequest, ShouldBeNil)
		So(dynamicUpdates[1].UpdateAction.GetInsert().Task.GetGeneric().RunRequest, ShouldBeNil)
		So(dynamicUpdates[1].UpdateAction.GetInsert().Task.GetGeneric().StopRequest, ShouldNotBeNil)
	})
}

func TestProvision(t *testing.T) {
	dynamicUpdates := []*api.UserDefinedDynamicUpdate{}
	Convey("Provision Generation", t, func() {
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

		So(err, ShouldBeNil)
		So(dynamicUpdates, ShouldHaveLength, 1)
		So(reflect.TypeOf(dynamicUpdates[0].FocalTaskFinder.Finder), ShouldEqual, reflect.TypeOf((*api.FocalTaskFinder_First_)(nil)))
		So(dynamicUpdates[0].FocalTaskFinder.GetFirst().TaskType, ShouldEqual, api.FocalTaskFinder_PROVISION)
		So(reflect.TypeOf(dynamicUpdates[0].UpdateAction.Action), ShouldEqual, reflect.TypeOf((*api.UpdateAction_Insert_)(nil)))
		So(dynamicUpdates[0].UpdateAction.GetInsert().InsertType, ShouldEqual, api.UpdateAction_Insert_REPLACE)
		So(dynamicUpdates[0].UpdateAction.GetInsert().Task, ShouldNotBeNil)
		So(dynamicUpdates[0].UpdateAction.GetInsert().Task.GetProvision().StartupRequest, ShouldNotBeNil)
		So(dynamicUpdates[0].UpdateAction.GetInsert().Task.GetProvision().InstallRequest, ShouldNotBeNil)
	})
}

func TestModify(t *testing.T) {
	dynamicUpdates := []*api.UserDefinedDynamicUpdate{}
	Convey("Modify Generation", t, func() {
		generator := generators.NewModifyGenerator(
			common.FindFirst(api.FocalTaskFinder_TEST),
		)
		So(generator.AddModification(
			structpb.NewStringValue("cros-test-cq-light"),
			map[string]string{
				"orderedContainerRequests.0.containerImageKey": "value",
			},
		), ShouldBeNil)
		So(generator.AddModification(
			&labapi.IpEndpoint{
				Address: "devboard-address",
				Port:    12345,
			},
			map[string]string{
				"test.testRequest.primary.devboardServer": "",
			},
		), ShouldBeNil)

		err := libapi.AppendUserDefinedDynamicUpdates(&dynamicUpdates, generator.Generate)

		So(err, ShouldBeNil)
		So(dynamicUpdates, ShouldHaveLength, 1)
		So(reflect.TypeOf(dynamicUpdates[0].UpdateAction.Action), ShouldEqual, reflect.TypeOf((*api.UpdateAction_Modify_)(nil)))
		So(dynamicUpdates[0].UpdateAction.GetModify().Modifications, ShouldHaveLength, 2)
		stringValue := &structpb.Value{}
		err = dynamicUpdates[0].UpdateAction.GetModify().GetModifications()[0].GetPayload().UnmarshalTo(stringValue)
		So(err, ShouldBeNil)
		So(stringValue.GetStringValue(), ShouldEqual, "cros-test-cq-light")
		So(dynamicUpdates[0].UpdateAction.GetModify().GetModifications()[0].GetInstructions()["orderedContainerRequests.0.containerImageKey"], ShouldEqual, "value")
		ipEndpoint := &labapi.IpEndpoint{}
		err = dynamicUpdates[0].UpdateAction.GetModify().GetModifications()[1].GetPayload().UnmarshalTo(ipEndpoint)
		So(err, ShouldBeNil)
		So(ipEndpoint.GetAddress(), ShouldEqual, "devboard-address")
		So(ipEndpoint.GetPort(), ShouldEqual, 12345)
		So(dynamicUpdates[0].UpdateAction.GetModify().GetModifications()[1].GetInstructions()["test.testRequest.primary.devboardServer"], ShouldEqual, "")
	})
}
