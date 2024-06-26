// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package resolver

import (
	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"infra/cros/cmd/common_lib/common"
)

type DynamicPlaceholderLookup map[string]string

func (lookup DynamicPlaceholderLookup) Get(key string) (val string, ok bool) {
	val, ok = lookup[key]
	return
}

// Resolve converts the provided dynamic update into a json string
// and applies placeholder resolution, then converts back into a dynamic update object.
func Resolve(dynamicUpdate *api.UserDefinedDynamicUpdate, lookup map[string]string) (*api.UserDefinedDynamicUpdate, error) {
	jsonBytes, err := protojson.Marshal(dynamicUpdate)
	if err != nil {
		return nil, errors.Annotate(err, "failed to marshal dynamic update").Err()
	}

	dynamicLookup := DynamicPlaceholderLookup(lookup)
	resolvedJsonStr := common.ResolvePlaceholders(string(jsonBytes), dynamicLookup)
	// Run one more time. Allows placeholders to be embedded
	// within another placeholder by one, and only one, level.
	resolvedJsonStr = common.ResolvePlaceholders(resolvedJsonStr, dynamicLookup)

	resolvedUpdate := &api.UserDefinedDynamicUpdate{}
	unmarshaller := protojson.UnmarshalOptions{
		DiscardUnknown: true,
		AllowPartial:   true,
	}
	err = unmarshaller.Unmarshal([]byte(resolvedJsonStr), resolvedUpdate)
	if err != nil {
		return nil, errors.Annotate(err, "failed to unmarshal dynamic update").Err()
	}

	return resolvedUpdate, nil
}
