// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package testspec

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"go.chromium.org/chromiumos/infra/proto/go/chromite/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestGetReturnsPartialResults(t *testing.T) {
	fl := newFakeLoader()
	fl.AddSuites([]string{"suite", "non_existent_suite"})
	fl.AddTests([]string{"test", "non_existent_test"})
	ft := newFakeParseTestControlFn(map[string]*testMetadata{
		"test": testWithNameAndSuites("test", []string{}),
	})
	fs := newFakeParseSuiteControlFn(map[string]*api.AutotestSuite{
		"suite": {Name: "suite"},
	})
	g := getter{fl, ft, fs}
	resp, err := g.Get("ignored")
	if err == nil {
		t.Errorf("getter.Get() did not report parse failures")
	}

	wantTests := []string{"test"}
	gotTests := []string{}
	for _, t := range resp.GetAutotest().GetTests() {
		gotTests = append(gotTests, t.Name)
	}
	assert.That(t, gotTests, should.Match(wantTests))

	wantSuites := []string{"suite"}
	gotSuites := []string{}
	for _, s := range resp.GetAutotest().GetSuites() {
		gotSuites = append(gotSuites, s.Name)
	}
	assert.That(t, gotSuites, should.Match(wantSuites))
}

func TestGetSuiteWithoutTests(t *testing.T) {
	fl := newFakeLoader()
	fl.AddSuites([]string{"suite"})
	ft := newFakeParseTestControlFn(map[string]*testMetadata{})
	fs := newFakeParseSuiteControlFn(map[string]*api.AutotestSuite{
		"suite": {Name: "suite"},
	})
	g := getter{fl, ft, fs}
	resp, err := g.Get("ignored")
	if err != nil {
		t.Fatalf("getter.Get(): %s", err)
	}
	want := map[string][]string{"suite": {}}
	got := extractSuiteTests(resp.GetAutotest().GetSuites())
	assert.That(t, got, should.Match(want))
}

func TestGetSuiteWithOneTest(t *testing.T) {
	fl := newFakeLoader()
	fl.AddTests([]string{"test"})
	fl.AddSuites([]string{"suite"})
	ft := newFakeParseTestControlFn(map[string]*testMetadata{
		"test": testWithNameAndSuites("test", []string{"suite"}),
	})
	fs := newFakeParseSuiteControlFn(map[string]*api.AutotestSuite{
		"suite": {Name: "suite"},
	})
	g := getter{fl, ft, fs}
	resp, err := g.Get("root")
	if err != nil {
		t.Fatalf("getter.Get(): %s", err)
	}
	want := map[string][]string{"suite": {"test"}}
	got := extractSuiteTests(resp.GetAutotest().GetSuites())
	assert.That(t, got, should.Match(want))
}

func TestGetSuiteWithTwoTests(t *testing.T) {
	fl := newFakeLoader()
	fl.AddTests([]string{"test1", "test2"})
	fl.AddSuites([]string{"suite"})
	ft := newFakeParseTestControlFn(map[string]*testMetadata{
		"test1": testWithNameAndSuites("test1", []string{"suite"}),
		"test2": testWithNameAndSuites("test2", []string{"suite"}),
	})
	fs := newFakeParseSuiteControlFn(map[string]*api.AutotestSuite{
		"suite": {Name: "suite"},
	})
	g := getter{fl, ft, fs}
	resp, err := g.Get("root")
	if err != nil {
		t.Fatalf("getter.Get(): %s", err)
	}
	want := map[string][]string{"suite": {"test1", "test2"}}
	got := extractSuiteTests(resp.GetAutotest().GetSuites())
	assert.That(t, got, should.Match(want))
}

func TestGetTwoSuitesWithSameTest(t *testing.T) {
	fl := newFakeLoader()
	fl.AddTests([]string{"test"})
	fl.AddSuites([]string{"suite1", "suite2"})
	ft := newFakeParseTestControlFn(map[string]*testMetadata{
		"test": testWithNameAndSuites("test", []string{"suite1", "suite2"}),
	})
	fs := newFakeParseSuiteControlFn(map[string]*api.AutotestSuite{
		"suite1": {Name: "suite1"},
		"suite2": {Name: "suite2"},
	})
	g := getter{fl, ft, fs}
	resp, err := g.Get("root")
	if err != nil {
		t.Fatalf("getter.Get(): %s", err)
	}
	want := map[string][]string{
		"suite1": {"test"},
		"suite2": {"test"},
	}
	got := extractSuiteTests(resp.GetAutotest().GetSuites())
	assert.That(t, got, should.Match(want))
}

func TestGetTestInNonExistentSuite(t *testing.T) {
	fl := newFakeLoader()
	fl.AddTests([]string{"test"})
	ft := newFakeParseTestControlFn(map[string]*testMetadata{
		"test": testWithNameAndSuites("test", []string{"non_existent_suite"}),
	})
	fs := newFakeParseSuiteControlFn(map[string]*api.AutotestSuite{})
	g := getter{fl, ft, fs}
	resp, err := g.Get("ignored")
	if err != nil {
		t.Fatalf("getter.Get(): %s", err)
	}
	want := map[string][]string{}
	got := extractSuiteTests(resp.GetAutotest().GetSuites())
	assert.That(t, got, should.Match(want))
}

func TestGetValidatesTestName(t *testing.T) {
	fl := newFakeLoader()
	fl.AddTests([]string{"test", "corrupt_test"})
	ft := newFakeParseTestControlFn(map[string]*testMetadata{
		"test":         testWithNameAndSuites("test", []string{"non_existent_suite"}),
		"corrupt_test": testWithNameAndSuites("", []string{}),
	})
	fs := newFakeParseSuiteControlFn(map[string]*api.AutotestSuite{})
	g := getter{fl, ft, fs}
	resp, err := g.Get("ignored")
	if err == nil {
		t.Fatalf("getter.Get() did not report validation failure")
	}
	wantTests := []string{"test"}
	gotTests := []string{}
	for _, t := range resp.GetAutotest().GetTests() {
		gotTests = append(gotTests, t.Name)
	}
	assert.That(t, gotTests, should.Match(wantTests))
}

// newFakeParseTestControlFn returns a fake parseTestControlFn that returns
// canned parse results.
//
// canned must map *contents of the control file* to their parse results. The
// returned parseTestControlFn returns error for any control file not in canned.
func newFakeParseTestControlFn(canned map[string]*testMetadata) parseTestControlFn {
	return func(text string) (*testMetadata, errors.MultiError) {
		tm, ok := canned[text]
		if !ok {
			return nil, errors.NewMultiError(errors.Reason("uncanned control file: %s", text).Err())
		}
		return tm, nil
	}
}

func testWithNameAndSuites(name string, suites []string) *testMetadata {
	return &testMetadata{
		AutotestTest: api.AutotestTest{
			Name:                 name,
			ExecutionEnvironment: api.AutotestTest_EXECUTION_ENVIRONMENT_CLIENT,
		},
		Suites: suites,
	}
}

// newFakeParseSuiteControlFn returns a fake parseSuiteControlFn that returns
// canned parse results.
//
// canned must map *contents of the control file* to their parse results. The
// returned parseSuiteControlFn returns error for any control file not in
// canned.
func newFakeParseSuiteControlFn(canned map[string]*api.AutotestSuite) parseSuiteControlFn {
	return func(text string) (*api.AutotestSuite, errors.MultiError) {
		as, ok := canned[text]
		if !ok {
			return nil, errors.NewMultiError(errors.Reason("uncanned control file: %s", text).Err())
		}
		return as, nil
	}
}

func newFakeLoader() *fakeLoader {
	return &fakeLoader{
		tests:  make(map[string]io.Reader),
		suites: make(map[string]io.Reader),
	}
}

type fakeLoader struct {
	tests      map[string]io.Reader
	suites     map[string]io.Reader
	pathSuffix int
}

// AddTests adds the given texts as a test new control files at  arbitrary
// paths.
func (d *fakeLoader) AddTests(texts []string) {
	for _, t := range texts {
		d.tests[fmt.Sprintf("test%d", d.pathSuffix)] = strings.NewReader(t)
		d.pathSuffix++
	}
}

// RegisterSuite adds the given texts as a new suite control files at arbitrary
// paths.
func (d *fakeLoader) AddSuites(texts []string) {
	for _, t := range texts {
		d.suites[fmt.Sprintf("test%d", d.pathSuffix)] = strings.NewReader(t)
		d.pathSuffix++
	}
}

func (d *fakeLoader) Discover(string) error {
	return nil
}

func (d *fakeLoader) Tests() map[string]io.Reader {
	return d.tests
}

func (d *fakeLoader) Suites() map[string]io.Reader {
	return d.suites
}

func extractSuiteTests(suites []*api.AutotestSuite) map[string][]string {
	m := make(map[string][]string)
	for _, s := range suites {
		ts := []string{}
		for _, t := range s.GetTests() {
			ts = append(ts, t.Name)
		}
		m[s.Name] = ts
	}
	return m
}
