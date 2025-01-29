// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package centralizedsuite

import (
	"fmt"
	"os"

	"go.chromium.org/chromiumos/config/go/test/api"
	"google.golang.org/protobuf/proto"
)

const (
	defaultSuiteSetsFile = "/tmp/test/suite_sets.pb"
	defaultSuitesFile    = "/tmp/test/suites.pb"
)

var errCentralizedSuiteNotFound = fmt.Errorf("centralized suite not found")
var errReadingFile = fmt.Errorf("error reading file")
var errUnmarshalingProto = fmt.Errorf("error unmarshaling proto message")

// MappingsLoader represents an object that can load Suite/SuiteSetMappings.
type MappingsLoader interface {
	Load() (Mappings, error)
}

// InMemoryLoader is an implementation of MappingsLoader that is useful for
// mocking a MappingsLoader for testing purposes.
type InMemoryLoader struct {
	mappings Mappings
}

// NewInMemoryLoader creates an InMemoryLoader that loads the given mappings.
func NewInMemoryLoader(rawSuiteSets *api.SuiteSetList, rawSuites *api.SuiteList) *InMemoryLoader {
	return &InMemoryLoader{
		mappings: newMappings(rawSuiteSets, rawSuites),
	}
}

// Load returns the Mappings defined in the InMemoryLoader.
func (i *InMemoryLoader) Load() (Mappings, error) {
	return i.mappings, nil
}

// FileLoader loads the Mappings from Suite and Suite proto files.
type FileLoader struct {
	suiteSetsFile string
	suitesFile    string
}

// NewFileLoader creates a FileLoader that will load the mappings from
// the default file locations.
func NewFileLoader() *FileLoader {
	return &FileLoader{
		suiteSetsFile: defaultSuiteSetsFile,
		suitesFile:    defaultSuitesFile,
	}
}

// NewCustomFileLoader creates a FileLoader that will load the mappings from
// the given files.
func NewCustomFileLoader(suiteSetsFile, suitesFile string) *FileLoader {
	return &FileLoader{
		suiteSetsFile: suiteSetsFile,
		suitesFile:    suitesFile,
	}
}

// Load reads the Suite/SuiteSet mappings from the file locations specified in
// the FileLoader.
func (f *FileLoader) Load() (Mappings, error) {
	suiteSets, err := f.loadRawSuiteSets()
	if err != nil {
		return nil, err
	}
	suites, err := f.loadRawSuites()
	if err != nil {
		return nil, err
	}
	return newMappings(suiteSets, suites), nil
}

func (f *FileLoader) loadRawSuiteSets() (*api.SuiteSetList, error) {
	data, err := os.ReadFile(f.suiteSetsFile)
	if err != nil {
		return nil, fmt.Errorf("%w '%v': %w", errReadingFile, f.suiteSetsFile, err)
	}
	rawSuiteSets := &api.SuiteSetList{}
	if err := proto.Unmarshal(data, rawSuiteSets); err != nil {
		return nil, fmt.Errorf("%w: %w", errUnmarshalingProto, err)
	}
	return rawSuiteSets, nil
}

func (f *FileLoader) loadRawSuites() (*api.SuiteList, error) {
	data, err := os.ReadFile(f.suitesFile)
	if err != nil {
		return nil, fmt.Errorf("%w '%v': %w", errReadingFile, f.suitesFile, err)
	}
	rawSuites := &api.SuiteList{}
	if err := proto.Unmarshal(data, rawSuites); err != nil {
		return nil, fmt.Errorf("%w: %w", errUnmarshalingProto, err)
	}
	return rawSuites, nil
}

// Mappings is the mappings of all the SuiteSets to their respective
// Suites/SuiteSets and all the Suites to their respective tests.
type Mappings map[string]centralizedSuite

func newMappings(rawSuiteSets *api.SuiteSetList, rawSuites *api.SuiteList) Mappings {
	suiteSets := newSuiteSets(rawSuiteSets)
	suites := newSuites(rawSuites)
	m := make(Mappings, len(suiteSets)+len(suites))
	for id, suiteSet := range suiteSets {
		m[id] = suiteSet
	}
	for id, suite := range suites {
		m[id] = suite
	}
	return m
}

func newSuiteSets(rawSuiteSets *api.SuiteSetList) map[string]suiteSet {
	suiteSets := make(map[string]suiteSet, len(rawSuiteSets.GetSuiteSets()))
	for _, rawSuiteSet := range rawSuiteSets.GetSuiteSets() {
		suiteSets[rawSuiteSet.GetId().GetValue()] = newSuiteSet(rawSuiteSet)
	}
	return suiteSets
}

func newSuites(rawSuites *api.SuiteList) map[string]suite {
	suites := make(map[string]suite, len(rawSuites.GetSuites()))
	for _, rawSuite := range rawSuites.GetSuites() {
		suites[rawSuite.GetId().GetValue()] = newSuite(rawSuite)
	}
	return suites
}

// TestsIn returns a set of all the tests that are in the given centralized
// suite; returns a errCentralizedSuiteNotFound if the given centralized suite
// is not in the mappings.
func (m Mappings) TestsIn(centralizedSuiteID string) (map[string]struct{}, error) {
	centralizedSuite, ok := m[centralizedSuiteID]
	if !ok {
		return nil, fmt.Errorf("%w: '%v'", errCentralizedSuiteNotFound, centralizedSuiteID)
	}
	return centralizedSuite.getTests(m)
}

type centralizedSuite interface {
	getTests(mappings Mappings) (map[string]struct{}, error)
}

type suiteSet struct {
	centralizedSuites map[string]struct{}
}

func newSuiteSet(rawSuiteSet *api.SuiteSet) suiteSet {
	centralizedSuites := make(map[string]struct{}, len(rawSuiteSet.GetSuiteSets())+len(rawSuiteSet.GetSuites()))
	for _, subSuiteSet := range rawSuiteSet.GetSuiteSets() {
		centralizedSuites[subSuiteSet.GetValue()] = struct{}{}
	}
	for _, subSuite := range rawSuiteSet.GetSuites() {
		centralizedSuites[subSuite.GetValue()] = struct{}{}
	}
	return suiteSet{
		centralizedSuites: centralizedSuites,
	}
}

func (s suiteSet) getTests(m Mappings) (map[string]struct{}, error) {
	tests := make(map[string]struct{}, 0)
	for centralizedSuiteID := range s.centralizedSuites {
		testsInCentralizedSuite, err := m.TestsIn(centralizedSuiteID)
		if err != nil {
			return nil, err
		}
		for test := range testsInCentralizedSuite {
			tests[test] = struct{}{}
		}
	}
	return tests, nil
}

type suite struct {
	tests map[string]struct{}
}

func newSuite(rawSuite *api.Suite) suite {
	tests := make(map[string]struct{}, len(rawSuite.GetTests()))
	for _, test := range rawSuite.GetTests() {
		tests[test.GetValue()] = struct{}{}
	}
	return suite{
		tests: tests,
	}
}

func (s suite) getTests(m Mappings) (map[string]struct{}, error) {
	return s.tests, nil
}
