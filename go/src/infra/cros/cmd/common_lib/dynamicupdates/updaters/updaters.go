// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package updaters defines updates for dynamic requests.
package updaters

import (
	"fmt"
	"reflect"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"infra/cros/cmd/common_lib/common"
)

// ProcessUpdateAction maps each update action to a handler.
func ProcessUpdateAction(req *api.CrosTestRunnerDynamicRequest, updateAction *api.UpdateAction, focalIndex int) error {
	switch action := updateAction.Action.(type) {
	case *api.UpdateAction_Insert_:
		return handleInsertAction(req, action.Insert, focalIndex)
	case *api.UpdateAction_Remove_:
		return handleRemoveAction(req, focalIndex)
	case *api.UpdateAction_Modify_:
		return handleModifyAction(req, action.Modify, focalIndex)
	default:
		return fmt.Errorf("unhandled action type: %s", reflect.TypeOf(action))
	}
}

// handleInsertAction places the task within the trv2 dynamic request.
func handleInsertAction(req *api.CrosTestRunnerDynamicRequest, insertAction *api.UpdateAction_Insert, focalIndex int) error {
	switch insertAction.InsertType {
	case api.UpdateAction_Insert_APPEND:
		InsertTaskRight(&req.OrderedTasks, insertAction.Task, focalIndex)
	case api.UpdateAction_Insert_PREPEND:
		InsertTaskLeft(&req.OrderedTasks, insertAction.Task, focalIndex)
	case api.UpdateAction_Insert_REPLACE:
		if focalIndex >= len(req.GetOrderedTasks()) {
			return fmt.Errorf("index %d out of range, length was %d", focalIndex, len(req.GetOrderedTasks()))
		}
		req.OrderedTasks[focalIndex] = insertAction.Task
	default:
		return fmt.Errorf("unhandled insert type %s", insertAction.InsertType)
	}
	return nil
}

// handleRemoveAction deletes the task from the trv2 dynamic request,
// and shrinks the list accordingly.
func handleRemoveAction(req *api.CrosTestRunnerDynamicRequest, focalIndex int) error {
	if focalIndex >= len(req.GetOrderedTasks()) {
		return fmt.Errorf("index %d out of range, length was %d", focalIndex, len(req.GetOrderedTasks()))
	}
	req.OrderedTasks = append(req.OrderedTasks[:focalIndex], req.OrderedTasks[focalIndex+1:]...)
	return nil
}

// handleModifyAction applies the provided modifications to the task.
func handleModifyAction(req *api.CrosTestRunnerDynamicRequest, modifyAction *api.UpdateAction_Modify, focalIndex int) error {
	var err error
	if focalIndex >= len(req.GetOrderedTasks()) {
		return fmt.Errorf("index %d out of range, length was %d", focalIndex, len(req.GetOrderedTasks()))
	}

	storage := common.NewInjectableStorage()
	focalTask := req.GetOrderedTasks()[focalIndex]
	for _, modification := range modifyAction.Modifications {
		err = storage.Set("payload", modification.Payload)
		if err != nil {
			return errors.Annotate(err, "failed to add payload to injection storage").Err()
		}
		for injectionPath, payloadKey := range modification.Instructions {
			payloadKey = strings.Join([]string{"payload", payloadKey}, ".")
			err = common.InjectDependencies(focalTask, storage, []*api.DynamicDep{
				{
					Key:   injectionPath,
					Value: payloadKey,
				},
			})
			if err != nil {
				return errors.Annotate(err, "failed to inject %s into %s", payloadKey, injectionPath).Err()
			}
		}
	}
	return nil
}
