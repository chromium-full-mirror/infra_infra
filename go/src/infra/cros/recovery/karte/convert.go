// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package karte

import (
	"math"
	"strconv"
	"time"

	timestamppb "google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/luci/common/errors"

	kartepb "go.chromium.org/infra/cros/karte/api"
	"go.chromium.org/infra/cros/recovery/logger/metrics"
)

// ConvertActionStatusToKarteActionStatus takes a metrics action status and converts it to a Karte action status.
func convertActionStatusToKarteActionStatus(status metrics.ActionStatus) kartepb.Action_Status {
	// TODO(gregorynisbet): Add support for skipped actions to Karte.
	switch status {
	case metrics.ActionStatusSuccess:
		return kartepb.Action_SUCCESS
	case metrics.ActionStatusFail:
		return kartepb.Action_FAIL
	case metrics.ActionStatusSkip:
		return kartepb.Action_SKIP
	default:
		return kartepb.Action_STATUS_UNSPECIFIED
	}
}

// ConvertKarteActionStatusToActionStatus takes a Karte action status and converts it to a metrics action status.
func convertKarteActionStatusToActionStatus(status kartepb.Action_Status) metrics.ActionStatus {
	switch status {
	case kartepb.Action_SUCCESS:
		return metrics.ActionStatusSuccess
	case kartepb.Action_FAIL:
		return metrics.ActionStatusFail
	case kartepb.Action_SKIP:
		return metrics.ActionStatusSkip
	default:
		return metrics.ActionStatusUnspecified
	}
}

// convertAllowFailToKarteAllowFail converts an allow fail value to a Karte allow fail value.
func convertAllowFailToKarteAllowFail(allowFail metrics.AllowFail) kartepb.Action_AllowFail {
	switch allowFail {
	case metrics.YesAllowFail:
		return kartepb.Action_ALLOW_FAIL
	case metrics.NoAllowFail:
		return kartepb.Action_NO_ALLOW_FAIL
	default:
		return kartepb.Action_ALLOW_FAIL_UNSPECIFIED
	}
}

// convertKarteAllowFailToAllowFail converts a Karte allow fail value to a Boolean.
func convertKarteAllowFailToAllowFail(allowFail kartepb.Action_AllowFail) metrics.AllowFail {
	switch allowFail {
	case kartepb.Action_ALLOW_FAIL:
		return metrics.YesAllowFail
	case kartepb.Action_NO_ALLOW_FAIL:
		return metrics.NoAllowFail
	default:
		return metrics.AllowFailUnspecified
	}
}

func convertKarteActionTypeToActionType(actionType kartepb.Action_ActionType) metrics.ActionType {
	switch actionType {
	case kartepb.Action_ACTION_TYPE_UNSPECIFIED:
		return metrics.ActionTypeUnspecified
	case kartepb.Action_ACTION_TYPE_VERIFIER:
		return metrics.ActionTypeVerifier
	case kartepb.Action_ACTION_TYPE_CONDITION:
		return metrics.ActionTypeCondition
	case kartepb.Action_ACTION_TYPE_RECOVERY:
		return metrics.ActionTypeRecovery
	}
	return metrics.ActionTypeUnspecified
}

func convertActionTypeToKarteActionType(actionType metrics.ActionType) kartepb.Action_ActionType {
	switch actionType {
	case metrics.ActionTypeUnspecified:
		return kartepb.Action_ACTION_TYPE_UNSPECIFIED
	case metrics.ActionTypeVerifier:
		return kartepb.Action_ACTION_TYPE_VERIFIER
	case metrics.ActionTypeCondition:
		return kartepb.Action_ACTION_TYPE_CONDITION
	case metrics.ActionTypeRecovery:
		return kartepb.Action_ACTION_TYPE_RECOVERY
	}
	return kartepb.Action_ACTION_TYPE_UNSPECIFIED
}

// ConvertTimeToProtobufTimestamp takes a time and converts it to a pointer to a protobuf timestamp.
// This method sends the zero time to a nil pointer.
func convertTimeToProtobufTimestamp(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

// ConvertProtobufTimestampToTime takes a protobuf timestamp and converts it
// to a Go time.Time. We can't just use thing.AsTime() because the protobuf timestamp and time.Time do not agree on what the "zero time" is.
//
// This is the error message that we get if we instead use thing.AsTime().
//
//	-   StartTime:      s"0001-01-01 00:00:00 +0000 UTC",
//	+   StartTime:      s"1970-01-01 00:00:00 +0000 UTC",
//	-   StopTime:       s"0001-01-01 00:00:00 +0000 UTC",
//	+   StopTime:       s"1970-01-01 00:00:00 +0000 UTC",
func convertProtobufTimestampToTime(t *timestamppb.Timestamp) time.Time {
	var zero time.Time
	if t == nil {
		return zero
	}
	return t.AsTime()
}

// ConvertActionToKarteAction takes an action and converts it to a Karte action.
func convertActionToKarteAction(action *metrics.Action) *kartepb.Action {
	if action == nil {
		return nil
	}
	return &kartepb.Action{
		Name:           action.Name,
		Kind:           action.ActionKind,
		SwarmingTaskId: action.SwarmingTaskID,
		BuildbucketId:  action.BuildbucketID,
		Model:          action.Model,
		Board:          action.Board,
		AssetTag:       action.AssetTag,
		StartTime:      convertTimeToProtobufTimestamp(action.StartTime),
		StopTime:       convertTimeToProtobufTimestamp(action.StopTime),
		Status:         convertActionStatusToKarteActionStatus(action.Status),
		FailReason:     action.FailReason,
		Hostname:       action.Hostname,
		RecoveredBy:    action.RecoveredBy,
		Restarts:       action.Restarts,
		AllowFail:      convertAllowFailToKarteAllowFail(action.AllowFail),
		PlanName:       action.PlanName,
		ActionType:     convertActionTypeToKarteActionType(action.Type),
	}
}

// ConvertKarteActionToAction takes a Karte action and converts it to an action.
func convertKarteActionToAction(action *kartepb.Action) *metrics.Action {
	if action == nil {
		return nil
	}
	return &metrics.Action{
		Name:           action.GetName(),
		ActionKind:     action.GetKind(),
		SwarmingTaskID: action.GetSwarmingTaskId(),
		BuildbucketID:  action.GetBuildbucketId(),
		Model:          action.GetModel(),
		Board:          action.GetBoard(),
		AssetTag:       action.GetAssetTag(),
		StartTime:      convertProtobufTimestampToTime(action.GetStartTime()),
		StopTime:       convertProtobufTimestampToTime(action.GetStopTime()),
		Status:         convertKarteActionStatusToActionStatus(action.GetStatus()),
		FailReason:     action.GetFailReason(),
		Hostname:       action.GetHostname(),
		RecoveredBy:    action.GetRecoveredBy(),
		Restarts:       action.GetRestarts(),
		AllowFail:      convertKarteAllowFailToAllowFail(action.GetAllowFail()),
		PlanName:       action.GetPlanName(),
		Type:           convertKarteActionTypeToActionType(action.GetActionType()),
	}
}

// makeKarteObservation takes an action name and an observation and creates a Karte observation.
func makeKarteObservation(actionName string, observation *metrics.Observation) (*kartepb.Observation, error) {
	if actionName == "" {
		return nil, errors.Reason(`action name is ""`).Err()
	}
	if observation == nil {
		return nil, errors.Reason("observation is nil").Err()
	}
	out := &kartepb.Observation{
		ActionName: actionName,
		MetricKind: observation.MetricKind,
	}
	switch observation.ValueType {
	case metrics.ValueTypeNumber:
		f, err := strconv.ParseFloat(observation.Value, 64)
		if err != nil {
			f = math.NaN()
		}
		out.Value = &kartepb.Observation_ValueNumber{
			ValueNumber: f,
		}
	default:
		out.Value = &kartepb.Observation_ValueString{
			ValueString: observation.Value,
		}
	}
	return out, nil
}
