// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stableversion

import (
	"fmt"
	"sort"
	"strings"

	"github.com/golang/protobuf/jsonpb"
	proto "github.com/golang/protobuf/proto"

	sv "go.chromium.org/chromiumos/infra/proto/go/lab_platform"
)

// WriteSVToString marshals stable version information into a string.
func WriteSVToString(s *sv.StableVersions) (string, error) {
	all := proto.Clone(s).(*sv.StableVersions)
	sortSV(all)
	return (&jsonpb.Marshaler{Indent: "\t"}).MarshalToString(all)
}

// sortSV sorts all the individual entries in a stable version config file.
func sortSV(s *sv.StableVersions) {
	c := s.GetVersions()
	if c == nil {
		return
	}
	sort.SliceStable(c, func(i, j int) bool {
		ki := TargetToKey(c[i]).String()
		kj := TargetToKey(c[j]).String()
		return strings.ToLower(ki) < strings.ToLower(kj)
	})
}

const separator = ";"

// JoinBuildTargetModel -- join a buildTarget string and a model string to produce a combined key
func JoinBuildTargetModel(buildTarget string, model string) (string, error) {
	b := strings.TrimSpace(strings.ToLower(buildTarget))
	m := strings.TrimSpace(strings.ToLower(model))
	if err := ValidateJoinBuildTargetModel(b, m); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%s%s", b, separator, m), nil
}

// FallbackBuildTargetKey creates the key based on the given build target
// This kind of key should only ever be used as a fallback when looking up a stable version.
func FallbackBuildTargetKey(buildTarget string) string {
	return strings.ToLower(buildTarget)
}

// ValidateJoinBuildTargetModel -- checks that a buildTarget and model are valid
// The model is explicitly allowed to be empty.
func ValidateJoinBuildTargetModel(buildTarget string, model string) error {
	if buildTarget == "" {
		return fmt.Errorf("ValidateJoinBuildTargetModel: buildTarget cannot be empty")
	}
	if model == "" {
		return fmt.Errorf("ValidateJoinBuildTargetModel: model cannot be empty")
	}
	if strings.Contains(buildTarget, separator) {
		return fmt.Errorf("ValidateJoinBuildTargetModel: buildTarget cannot contain separator(%s)", separator)
	}
	if strings.Contains(model, separator) {
		return fmt.Errorf("ValidateJoinBuildTargetModel: model cannot contain separator(%s)", separator)
	}
	return nil
}
