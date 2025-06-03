// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"fmt"
	"sort"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
)

// ConvertProtoMapToAnyMap marshals each provided proto message into an anypb Any.
func ConvertProtoMapToAnyMap(m map[string]proto.Message) (map[string]*anypb.Any, error) {
	ret := map[string]*anypb.Any{}
	var allErrs error

	for key, val := range m {
		anyVal, err := anypb.New(val)
		if err != nil {
			allErrs = errors.Append(allErrs, err)
			continue
		}
		ret[key] = anyVal
	}

	return ret, allErrs
}

// generateDynamicDeps converts a UserContainerRequest's DynamicInputs
// into a dynamic dependency list.
func GenerateDynamicDeps(rpc string, depMap map[string]string, format string) []*api.DynamicDep {
	deps := []*api.DynamicDep{}

	for key, val := range depMap {
		deps = append(deps, &api.DynamicDep{
			Key:   fmt.Sprintf(format, rpc, key),
			Value: val,
		})
	}

	// Sort by key to ensure nested children are placed at later positions.
	// e.g. dut.cacheServer should be put after dut, but map doesn't guarantee any order in golang.
	sort.Slice(deps, func(i, j int) bool {
		return deps[i].GetKey() < deps[j].GetKey()
	})

	return deps
}

// SetPlaceholder returns the reference key with the
// defined placeholder format.
func SetPlaceholder(referenceKey string) string {
	return fmt.Sprintf("${%s}", referenceKey)
}
