// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package parse implements parsing of proto files for Centralized Suites and SuiteSets
package parse

import (
	"os"

	"google.golang.org/protobuf/proto"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/suite_publisher/internal/suite"
)

// ReadSuitesAndSuiteSets reads the proto files and returns a map of
// bqusuites.CentralizedSuite where key is the ID of the Suite or SuiteSet
// and value is an interface to the Suite or SuiteSet.
func ReadSuitesAndSuiteSets(suiteProtoPath, suiteSetProtoPath string) (map[string]suite.CentralizedSuite, error) {
	// read proto files
	suites, err := readSuite(suiteProtoPath)
	if err != nil {
		return nil, err
	}
	suiteSets, err := readSuiteSet(suiteSetProtoPath)
	if err != nil {
		return nil, err
	}
	ret := make(map[string]suite.CentralizedSuite)
	for _, s := range suites.GetSuites() {
		s := suite.NewSuite(s)
		ret[s.ID()] = s
	}
	for _, ss := range suiteSets.GetSuiteSets() {
		s := suite.NewSuiteSet(ss)
		ret[s.ID()] = s
	}
	return ret, nil
}

func readSuite(path string) (*api.SuiteList, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	suiteList := &api.SuiteList{}
	if err := proto.Unmarshal(data, suiteList); err != nil {
		return nil, err
	}

	return suiteList, nil
}

func readSuiteSet(path string) (*api.SuiteSetList, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// decode the protobuf file
	suiteSetList := &api.SuiteSetList{}
	if err := proto.Unmarshal(data, suiteSetList); err != nil {
		return nil, err
	}

	return suiteSetList, nil
}
