// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dynamic_updates_test

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	_go "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/cros/cmd/common_lib/common"
	dynamic "infra/cros/cmd/common_lib/dynamic_updates"
)

var (
	provisionType = reflect.TypeOf((*api.CrosTestRunnerDynamicRequest_Task_Provision)(nil))
	preTestType   = reflect.TypeOf((*api.CrosTestRunnerDynamicRequest_Task_PreTest)(nil))
	testType      = reflect.TypeOf((*api.CrosTestRunnerDynamicRequest_Task_Test)(nil))
	postTestType  = reflect.TypeOf((*api.CrosTestRunnerDynamicRequest_Task_PostTest)(nil))
	publishType   = reflect.TypeOf((*api.CrosTestRunnerDynamicRequest_Task_Publish)(nil))
	genericType   = reflect.TypeOf((*api.CrosTestRunnerDynamicRequest_Task_Generic)(nil))

	lookupTable = map[string]string{
		"board":       "fakeBoard",
		"installPath": "fakeImageInstallPath",
	}
)

func TestDynamicUpdater(t *testing.T) {
	req := baseRequest()
	ftt.Run("Find Provision, Prepend, Append, Replace", t, func(t *ftt.Test) {
		UDFs := []*api.UserDefinedDynamicUpdate{}
		UDFs = append(UDFs, insertActionWrapper(
			api.UpdateAction_Insert_PREPEND,
			getFirstTask(api.FocalTaskFinder_PROVISION),
			&api.CrosTestRunnerDynamicRequest_Task{
				Task: &api.CrosTestRunnerDynamicRequest_Task_Generic{},
			}))

		UDFs = append(UDFs, insertActionWrapper(
			api.UpdateAction_Insert_APPEND,
			getFirstTask(api.FocalTaskFinder_PROVISION),
			&api.CrosTestRunnerDynamicRequest_Task{
				Task: &api.CrosTestRunnerDynamicRequest_Task_Generic{},
			}))

		UDFs = append(UDFs, insertActionWrapper(
			api.UpdateAction_Insert_REPLACE,
			getFirstTask(api.FocalTaskFinder_PROVISION),
			&api.CrosTestRunnerDynamicRequest_Task{
				Task: &api.CrosTestRunnerDynamicRequest_Task_Generic{},
			}))

		err := dynamic.AddUserDefinedDynamicUpdates(req, UDFs, lookupTable)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, req.OrderedTasks, should.HaveLength(6))
		assert.Loosely(t, reflect.TypeOf(req.OrderedTasks[0].GetTask()), should.Equal(genericType))
		assert.Loosely(t, reflect.TypeOf(req.OrderedTasks[1].GetTask()), should.Equal(genericType))
		assert.Loosely(t, reflect.TypeOf(req.OrderedTasks[2].GetTask()), should.Equal(genericType))
		assert.Loosely(t, reflect.TypeOf(req.OrderedTasks[3].GetTask()), should.Equal(testType))
		assert.Loosely(t, reflect.TypeOf(req.OrderedTasks[4].GetTask()), should.Equal(publishType))
		assert.Loosely(t, reflect.TypeOf(req.OrderedTasks[5].GetTask()), should.Equal(publishType))
	})

	req = baseRequest()
	ftt.Run("Find Test, Inject", t, func(t *ftt.Test) {
		req.OrderedTasks[1].OrderedContainerRequests = []*api.ContainerRequest{
			{
				ContainerImageKey: "cros-test",
			},
		}
		req.OrderedTasks[1].GetTest().TestRequest = &api.CrosTestRequest{
			Primary: &api.CrosTestRequest_Device{},
		}
		req.OrderedTasks[1].GetTest().DynamicDeps = []*api.DynamicDep{
			{
				Key:   "dep-key",
				Value: "dep-value",
			},
		}
		UDFs := []*api.UserDefinedDynamicUpdate{}
		UDFs = append(UDFs, &api.UserDefinedDynamicUpdate{
			FocalTaskFinder: getFirstTask(api.FocalTaskFinder_TEST),
			UpdateAction: &api.UpdateAction{
				Action: &api.UpdateAction_Modify_{
					Modify: &api.UpdateAction_Modify{
						Modifications: []*api.UpdateAction_Modify_Modification{
							{
								Payload: convertToAny(structpb.NewStringValue("cros-test-cq-light")),
								Instructions: map[string]string{
									"orderedContainerRequests.0.containerImageKey": "value",
								},
							},
							{
								Payload: convertToAny(&api.ContainerRequest{
									DynamicIdentifier: "appended-container",
								}),
								Instructions: map[string]string{
									"orderedContainerRequests": "",
								},
							},
							{
								Payload: convertToAny(&labapi.IpEndpoint{
									Address: "devboard-address",
									Port:    12345,
								}),
								Instructions: map[string]string{
									"test.testRequest.primary.devboardServer": "",
								},
							},
							{
								Payload: convertToAny(&api.DynamicDep{
									Key:   "new-dep-key",
									Value: "new-dep-value",
								}),
								Instructions: map[string]string{
									"test.dynamicDeps": "",
								},
							},
						},
					},
				},
			},
		})

		err := dynamic.AddUserDefinedDynamicUpdates(req, UDFs, lookupTable)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, req.OrderedTasks, should.HaveLength(4))
		assert.Loosely(t, req.OrderedTasks[1].GetOrderedContainerRequests(), should.HaveLength(2))
		assert.Loosely(t, req.OrderedTasks[1].GetOrderedContainerRequests()[0].ContainerImageKey, should.Equal("cros-test-cq-light"))
		assert.Loosely(t, req.OrderedTasks[1].GetOrderedContainerRequests()[1].DynamicIdentifier, should.Equal("appended-container"))
		assert.Loosely(t, req.OrderedTasks[1].GetTest().GetTestRequest().GetPrimary().GetDevboardServer().GetAddress(), should.Equal("devboard-address"))
		assert.Loosely(t, req.OrderedTasks[1].GetTest().GetTestRequest().GetPrimary().GetDevboardServer().GetPort(), should.Equal(12345))
		assert.Loosely(t, req.OrderedTasks[1].GetTest().GetDynamicDeps(), should.HaveLength(2))
		assert.Loosely(t, req.OrderedTasks[1].GetTest().GetDynamicDeps()[1].GetKey(), should.Equal("new-dep-key"))
		assert.Loosely(t, req.OrderedTasks[1].GetTest().GetDynamicDeps()[1].GetValue(), should.Equal("new-dep-value"))
	})

	req = baseRequest()
	ftt.Run("Purge and test Append/Prepend on empty", t, func(t *ftt.Test) {
		UDFs := []*api.UserDefinedDynamicUpdate{
			getRemoveRequest(getBeginningTask()),
			getRemoveRequest(getBeginningTask()),
			getRemoveRequest(getBeginningTask()),
			getRemoveRequest(getBeginningTask()),
		}

		err := dynamic.AddUserDefinedDynamicUpdates(req, UDFs, lookupTable)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, req.OrderedTasks, should.HaveLength(0))

		UDFs = []*api.UserDefinedDynamicUpdate{
			insertActionWrapper(
				api.UpdateAction_Insert_PREPEND,
				getBeginningTask(),
				&api.CrosTestRunnerDynamicRequest_Task{
					Task: &api.CrosTestRunnerDynamicRequest_Task_Generic{},
				}),
			getRemoveRequest(getBeginningTask()),
			insertActionWrapper(
				api.UpdateAction_Insert_APPEND,
				getBeginningTask(),
				&api.CrosTestRunnerDynamicRequest_Task{
					Task: &api.CrosTestRunnerDynamicRequest_Task_Generic{},
				}),
		}

		err = dynamic.AddUserDefinedDynamicUpdates(req, UDFs, lookupTable)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, req.OrderedTasks, should.HaveLength(1))
	})

	req = baseRequest()
	ftt.Run("Remove specific id", t, func(t *ftt.Test) {
		UDFs := []*api.UserDefinedDynamicUpdate{
			getRemoveRequest(getTaskWithDynamicId("test-id")),
		}

		err := dynamic.AddUserDefinedDynamicUpdates(req, UDFs, lookupTable)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, req.OrderedTasks, should.HaveLength(3))
		assert.Loosely(t, reflect.TypeOf(req.OrderedTasks[0].GetTask()), should.Equal(provisionType))
		assert.Loosely(t, reflect.TypeOf(req.OrderedTasks[1].GetTask()), should.Equal(publishType))
		assert.Loosely(t, reflect.TypeOf(req.OrderedTasks[2].GetTask()), should.Equal(publishType))
	})

	req = baseRequest()
	ftt.Run("Cant find id", t, func(t *ftt.Test) {
		UDFs := []*api.UserDefinedDynamicUpdate{
			getRemoveRequest(getTaskWithDynamicId("test-id-does-not-exist")),
		}

		err := dynamic.AddUserDefinedDynamicUpdates(req, UDFs, lookupTable)
		assert.Loosely(t, err, should.NotBeNil)
	})

	req = baseRequest()
	ftt.Run("Can resolve placeholders", t, func(t *ftt.Test) {
		UDFs := []*api.UserDefinedDynamicUpdate{
			insertActionWrapper(
				api.UpdateAction_Insert_PREPEND,
				getBeginningTask(),
				&api.CrosTestRunnerDynamicRequest_Task{
					Task: &api.CrosTestRunnerDynamicRequest_Task_Provision{
						Provision: &api.ProvisionTask{
							InstallRequest: &api.InstallRequest{
								ImagePath: &_go.StoragePath{
									HostType: _go.StoragePath_GS,
									Path:     "${installPath}",
								},
							},
							Target: common.NewCompanionDeviceIdentifier("${board}").Id,
						},
					},
				}),
		}

		err := dynamic.AddUserDefinedDynamicUpdates(req, UDFs, lookupTable)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, req.OrderedTasks, should.HaveLength(5))
		assert.Loosely(t, reflect.TypeOf(req.OrderedTasks[0].GetTask()), should.Equal(provisionType))
		assert.Loosely(t, req.OrderedTasks[0].GetProvision().Target, should.Equal(common.NewCompanionDeviceIdentifier(lookupTable["board"]).Id))
		assert.Loosely(t, req.OrderedTasks[0].GetProvision().InstallRequest.ImagePath.Path, should.Equal(lookupTable["installPath"]))
	})
}

func baseRequest() *api.CrosTestRunnerDynamicRequest {
	return &api.CrosTestRunnerDynamicRequest{
		OrderedTasks: []*api.CrosTestRunnerDynamicRequest_Task{
			{
				Task: &api.CrosTestRunnerDynamicRequest_Task_Provision{
					Provision: &api.ProvisionTask{
						DynamicIdentifier: "provision-id",
					},
				},
			},
			{
				Task: &api.CrosTestRunnerDynamicRequest_Task_Test{
					Test: &api.TestTask{
						DynamicIdentifier: "test-id",
					},
				},
			},
			{
				Task: &api.CrosTestRunnerDynamicRequest_Task_Publish{},
			},
			{
				Task: &api.CrosTestRunnerDynamicRequest_Task_Publish{},
			},
		},
	}
}

func getRemoveRequest(focalTaskFinder *api.FocalTaskFinder) *api.UserDefinedDynamicUpdate {
	return &api.UserDefinedDynamicUpdate{
		FocalTaskFinder: focalTaskFinder,
		UpdateAction: &api.UpdateAction{
			Action: &api.UpdateAction_Remove_{
				Remove: &api.UpdateAction_Remove{},
			},
		},
	}
}

func getBeginningTask() *api.FocalTaskFinder {
	return &api.FocalTaskFinder{
		Finder: &api.FocalTaskFinder_Beginning_{
			Beginning: &api.FocalTaskFinder_Beginning{},
		},
	}
}

func getFirstTask(taskType api.FocalTaskFinder_TaskType) *api.FocalTaskFinder {
	return &api.FocalTaskFinder{
		Finder: &api.FocalTaskFinder_First_{
			First: &api.FocalTaskFinder_First{
				TaskType: taskType,
			},
		},
	}
}

func getLastTask(taskType api.FocalTaskFinder_TaskType) *api.FocalTaskFinder {
	return &api.FocalTaskFinder{
		Finder: &api.FocalTaskFinder_Last_{
			Last: &api.FocalTaskFinder_Last{
				TaskType: taskType,
			},
		},
	}
}

func getTaskWithDynamicId(dynamicId string) *api.FocalTaskFinder {
	return &api.FocalTaskFinder{
		Finder: &api.FocalTaskFinder_ByDynamicIdentifier_{
			ByDynamicIdentifier: &api.FocalTaskFinder_ByDynamicIdentifier{
				DynamicIdentifier: dynamicId,
			},
		},
	}
}

func insertActionWrapper(insertType api.UpdateAction_Insert_InsertType, focalTaskFinder *api.FocalTaskFinder, task *api.CrosTestRunnerDynamicRequest_Task) *api.UserDefinedDynamicUpdate {
	return &api.UserDefinedDynamicUpdate{
		FocalTaskFinder: focalTaskFinder,
		UpdateAction: &api.UpdateAction{
			Action: &api.UpdateAction_Insert_{
				Insert: &api.UpdateAction_Insert{
					InsertType: insertType,
					Task:       task,
				},
			},
		},
	}
}

func convertToAny(src protoreflect.ProtoMessage) *anypb.Any {
	// Ignoring errors for sake of conciseness and readability.
	convertedAny, _ := anypb.New(src)
	return convertedAny
}
