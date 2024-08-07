// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package scopes

import (
	"context"
)

const (
	ParamKeyStableVersionServicePath = "stable_version_service_path"
	ParamKeyInventoryServicePath     = "inventory_service_path"
	ParamKeySwarmingTaskID           = "swarming_task_id"
	ParamKeySwarmingTaskTags         = "swarming_task_tags"
	ParamKeyBuildbucketID            = "buildbucket_id"
	ParamKeyCTRClient                = "ctr_client"
)

// ParamsMap is a special type to describe mapping of params of context
type ParamsMap = map[string]any

// WithParams sets params value map to the context.
func WithParams(ctx context.Context, params ParamsMap) context.Context {
	if params == nil {
		panic("Cannot set nil as map")
	}
	return context.WithValue(ctx, ctxParamsKey, params)
}

// GetParam returns value and bool if key is present in the params.
func GetParam(ctx context.Context, key string) (val any, ok bool) {
	params := getParams(ctx)
	if params == nil {
		return nil, false
	}
	param, ok := params[key]
	return param, ok
}

// GetParamCopy returns a copy of params from the context.
// Please use the `GetParam` method if you need check for presence of a key or get its value.
func GetParamCopy(ctx context.Context) ParamsMap {
	params := getParams(ctx)
	copy := make(ParamsMap, len(params))
	for k, v := range params {
		copy[k] = v
	}
	return copy
}

func getParams(ctx context.Context) ParamsMap {
	if params, ok := ctx.Value(ctxParamsKey).(ParamsMap); ok {
		return params
	}
	return nil
}
