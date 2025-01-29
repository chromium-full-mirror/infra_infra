// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package centralizedsuite

import (
	"errors"
	"os"
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.chromium.org/chromiumos/config/go/test/api"
	"google.golang.org/protobuf/proto"
)

var testSuiteSetList = &api.SuiteSetList{
	SuiteSets: []*api.SuiteSet{
		{
			Id: &api.SuiteSet_Id{Value: "power"},
			Suites: []*api.Suite_Id{
				{Value: "battery"},
				{Value: "boot"},
			},
		},
		{
			Id: &api.SuiteSet_Id{Value: "evt"},
			SuiteSets: []*api.SuiteSet_Id{
				{Value: "power"},
			},
			Suites: []*api.Suite_Id{
				{Value: "display"},
			},
		},
	},
}
var testSuiteList = &api.SuiteList{
	Suites: []*api.Suite{
		{
			Id: &api.Suite_Id{Value: "battery"},
			Tests: []*api.TestCase_Id{
				{Value: "bat-life"},
			},
		},
		{
			Id: &api.Suite_Id{Value: "boot"},
			Tests: []*api.TestCase_Id{
				{Value: "boot-perf"},
				{Value: "power-load"},
			},
		},
		{
			Id: &api.Suite_Id{Value: "display"},
			Tests: []*api.TestCase_Id{
				{Value: "video-out"},
				{Value: "external-disp"},
				{Value: "power-load"},
			},
		},
	},
}
var testMappings = Mappings{
	"power": suiteSet{
		centralizedSuites: map[string]struct{}{
			"battery": {},
			"boot":    {},
		},
	},
	"evt": suiteSet{
		centralizedSuites: map[string]struct{}{
			"power":   {},
			"display": {},
		},
	},
	"battery": suite{
		tests: map[string]struct{}{
			"bat-life": {},
		},
	},
	"boot": suite{
		tests: map[string]struct{}{
			"boot-perf":  {},
			"power-load": {},
		},
	},
	"display": suite{
		tests: map[string]struct{}{
			"video-out":     {},
			"external-disp": {},
			"power-load":    {},
		},
	},
}

var structComparer = cmp.Comparer(func(a, b struct{}) bool { return true })
var strSetComparer = mapComparer[string, struct{}](structComparer)
var suiteComparer = cmp.Comparer(func(a, b suite) bool {
	return cmp.Equal(a.tests, b.tests, strSetComparer)
})
var suiteSetComparer = cmp.Comparer(func(a, b suiteSet) bool {
	return cmp.Equal(a.centralizedSuites, b.centralizedSuites, strSetComparer)
})
var centralizedSuiteComparer = cmp.Comparer(func(a, b centralizedSuite) bool {
	switch valA := a.(type) {
	case suiteSet:
		switch valB := b.(type) {
		case suiteSet:
			return cmp.Equal(valA, valB, suiteSetComparer)
		}
	case suite:
		switch valB := b.(type) {
		case suite:
			return cmp.Equal(valA, valB, suiteComparer)
		}
	}
	return false
})
var mappingsComparer = mapComparer[string, centralizedSuite](centralizedSuiteComparer)

func mapComparer[X comparable, Y any](comparer cmp.Option) cmp.Option {
	return cmp.Comparer(func(a, b map[X]Y) bool {
		if len(a) != len(b) {
			return false
		}
		for key, valA := range a {
			valB, ok := b[key]
			if !ok {
				return false
			}
			if !cmp.Equal(valA, valB, comparer) {
				return false
			}
		}
		return true
	})
}

func TestTestsInSuiteSet(t *testing.T) {
	testCases := []struct {
		name               string
		centralizedSuiteID string
		wantTests          map[string]struct{}
		wantErr            error
	}{
		{
			name:               "tests in power suiteSet",
			centralizedSuiteID: "power",
			wantTests: map[string]struct{}{
				"bat-life":   {},
				"boot-perf":  {},
				"power-load": {},
			},
		},
		{
			name:               "tests in evt suiteSet",
			centralizedSuiteID: "evt",
			wantTests: map[string]struct{}{
				"bat-life":      {},
				"boot-perf":     {},
				"video-out":     {},
				"external-disp": {},
				"power-load":    {},
			},
		},
		{
			name:               "missing suiteSet",
			centralizedSuiteID: "missing",
			wantErr:            errCentralizedSuiteNotFound,
		},
		{
			name:               "tests in battery suite",
			centralizedSuiteID: "battery",
			wantTests: map[string]struct{}{
				"bat-life": {},
			},
		},
		{
			name:               "tests in display suite",
			centralizedSuiteID: "display",
			wantTests: map[string]struct{}{
				"video-out":     {},
				"external-disp": {},
				"power-load":    {},
			},
		},
		{
			name:               "missing suite",
			centralizedSuiteID: "missing",
			wantErr:            errCentralizedSuiteNotFound,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotTests, gotErr := testMappings.TestsIn(testCase.centralizedSuiteID)
			if !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("returned error does not match expected: want %v, got %v", testCase.wantErr, gotErr)
			}
			if testCase.wantErr != nil {
				return
			}
			if !cmp.Equal(testCase.wantTests, gotTests, strSetComparer) {
				t.Fatalf("computed tests do not match expected, want: %v, got %v", testCase.wantTests, gotTests)
			}
		})
	}
}

func writeProtoToFile(t *testing.T, msg proto.Message, file string) {
	data, err := proto.Marshal(msg)
	if err != nil {
		t.Fatalf("error when marshaling proto message: %v", err)
	}
	writeBytesToFile(t, data, file)
}

func writeBytesToFile(t *testing.T, data []byte, file string) {
	if err := os.WriteFile(file, data, 0644); err != nil {
		t.Fatalf("error when creating proto file: %v", err)
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	validSuiteSetsFile := path.Join(dir, "suitesets.pb")
	writeProtoToFile(t, testSuiteSetList, validSuiteSetsFile)
	validSuitesFile := path.Join(dir, "suites.pb")
	writeProtoToFile(t, testSuiteList, validSuitesFile)
	unparsableFile := path.Join(dir, "unparsable.pb")
	writeBytesToFile(t, []byte("unparsable: {"), unparsableFile)
	missingFile := path.Join(dir, "missing.pb")

	testCases := []struct {
		name         string
		loader       MappingsLoader
		wantMappings Mappings
		wantErr      error
	}{
		{
			name:         "valid mappings",
			loader:       NewCustomFileLoader(validSuiteSetsFile, validSuitesFile),
			wantMappings: testMappings,
		},
		{
			name:    "suiteSets file dne",
			loader:  NewCustomFileLoader(missingFile, validSuitesFile),
			wantErr: errReadingFile,
		},
		{
			name:    "error parsing suiteSets",
			loader:  NewCustomFileLoader(unparsableFile, validSuitesFile),
			wantErr: errUnmarshalingProto,
		},
		{
			name:    "suites file dne",
			loader:  NewCustomFileLoader(validSuiteSetsFile, missingFile),
			wantErr: errReadingFile,
		},
		{
			name:    "error parsing suites",
			loader:  NewCustomFileLoader(validSuiteSetsFile, unparsableFile),
			wantErr: errUnmarshalingProto,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotMappings, gotErr := testCase.loader.Load()
			if !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("returned error does not match expected: want %v, got %v", testCase.wantErr, gotErr)
			}
			if testCase.wantErr != nil {
				return
			}
			if !cmp.Equal(testCase.wantMappings, gotMappings, mappingsComparer) {
				t.Fatalf("computed tests do not match expected, want: %v, got %v", testCase.wantMappings, gotMappings)
			}
		})
	}
}
