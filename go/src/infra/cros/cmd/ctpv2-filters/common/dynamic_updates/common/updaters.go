// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/interfaces"
)

// PrependTaskWrapper creates an insert action that prepends the payload task.
func PrependTaskWrapper(focalTaskFinder *api.FocalTaskFinder) interfaces.InsertInstructionGetter {
	return InsertActionWrapper(api.UpdateAction_Insert_PREPEND, focalTaskFinder)
}

// AppendTaskWrapper creates an insert action that appends the payload task.
func AppendTaskWrapper(focalTaskFinder *api.FocalTaskFinder) interfaces.InsertInstructionGetter {
	return InsertActionWrapper(api.UpdateAction_Insert_APPEND, focalTaskFinder)
}

// ReplaceTaskWrapper creates an insert action that replaces with the payload task.
func ReplaceTaskWrapper(focalTaskFinder *api.FocalTaskFinder) interfaces.InsertInstructionGetter {
	return InsertActionWrapper(api.UpdateAction_Insert_REPLACE, focalTaskFinder)
}

// InsertActionWrapper provides a wrapper for creating an insert action for a task
// based on the desired insert type and focal task finder.
func InsertActionWrapper(insertType api.UpdateAction_Insert_InsertType, focalTaskFinder *api.FocalTaskFinder) interfaces.InsertInstructionGetter {
	return func(task *api.CrosTestRunnerDynamicRequest_Task) *api.UserDefinedDynamicUpdate {
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
}

// ModifyActionWrapper provides a wrapper for creating a modify action for a list of modifications.
func ModifyActionWrapper(focalTaskFinder *api.FocalTaskFinder) interfaces.ModifyActionGetter {
	return func(modifications []*api.UpdateAction_Modify_Modification) *api.UserDefinedDynamicUpdate {
		return &api.UserDefinedDynamicUpdate{
			FocalTaskFinder: focalTaskFinder,
			UpdateAction: &api.UpdateAction{
				Action: &api.UpdateAction_Modify_{
					Modify: &api.UpdateAction_Modify{
						Modifications: modifications,
					},
				},
			},
		}
	}
}

// RemoveAction creates a dynamic update for removing the task
// found by the focal task finder.
func RemoveAction(focalTaskFinder *api.FocalTaskFinder) *api.UserDefinedDynamicUpdate {
	return &api.UserDefinedDynamicUpdate{
		FocalTaskFinder: focalTaskFinder,
		UpdateAction: &api.UpdateAction{
			Action: &api.UpdateAction_Remove_{
				Remove: &api.UpdateAction_Remove{},
			},
		},
	}
}
