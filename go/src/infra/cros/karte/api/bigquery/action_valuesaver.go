// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kbqpb

import (
	"cloud.google.com/go/bigquery"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/karte/internal/scalars"
)

// Save takes in an action and produces a record to be inserted into bigquery and
// an insertID used for best-effort deduplication (which Karte) doesn't use.
// Why do we need this? Without the value-saver interface, we try to insert rows with fields that use the wrong casing convention `CreateTime` vs `create_time`. A sample error is shown below.
//
// "persist to bigquery failed:
// rpc error:
// code = Aborted desc = error persisting single record:
// ...
// Message: \"no such field: CreateTime.\";
// ...
func (a *Action) Save() (row map[string]bigquery.Value, insertID string, err error) {
	if a == nil {
		return nil, "", errors.Reason("action save: refusing to insert empty action").Err()
	}
	if a.GetName() == "" {
		return nil, "", errors.Reason("action save: refusing to insert action with empty name").Err()
	}

	// NOTE: The NoDedupeID feature is experimental.
	insertID = bigquery.NoDedupeID

	row = make(map[string]bigquery.Value)
	row["name"] = a.GetName()
	row["kind"] = a.GetKind()
	row["swarming_task_id"] = a.GetSwarmingTaskId()
	row["asset_tag"] = a.GetAssetTag()
	row["start_time"] = scalars.ConvertTimestampPtrToTime(a.GetStartTime())
	row["stop_time"] = scalars.ConvertTimestampPtrToTime(a.GetStopTime())
	row["create_time"] = scalars.ConvertTimestampPtrToTime(a.GetCreateTime())
	row["status"] = a.GetStatus()
	row["fail_reason"] = a.GetFailReason()
	row["seal_time"] = scalars.ConvertTimestampPtrToTime(a.GetSealTime())
	row["update_time"] = scalars.ConvertTimestampPtrToTime(a.GetUpdateTime())
	row["client_name"] = a.GetClientName()
	row["client_version"] = a.GetClientVersion()
	row["buildbucket_id"] = a.GetBuildbucketId()
	row["hostname"] = a.GetHostname()
	row["model"] = a.GetModel()
	row["board"] = a.GetBoard()
	row["restarts"] = a.GetRestarts()
	row["modification_count"] = a.GetModificationCount()
	row["recovered_by"] = a.GetRecoveredBy()
	row["plan_name"] = a.GetPlanName()
	row["allow_fail"] = a.GetAllowFail()
	row["action_type"] = a.GetActionType()

	return
}
