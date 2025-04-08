// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package finder find all matched tests from test metadata based on test criteria.
package finder

import (
	"fmt"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/proto"

	"go.chromium.org/chromiumos/config/go/test/api"
)

type tagMatcher struct {
	tags             map[string]struct{}
	excludes         map[string]struct{}
	testNames        map[string]struct{}
	testNameExcludes map[string]struct{}
}

func newTagMatcher(criteria *api.TestSuite_TestCaseTagCriteria) *tagMatcher {
	tags := make(map[string]struct{})
	for _, tag := range criteria.Tags {
		tags[tag] = struct{}{}
	}
	excludes := make(map[string]struct{})
	for _, tag := range criteria.TagExcludes {
		excludes[tag] = struct{}{}
	}
	testNames := make(map[string]struct{})
	for _, testName := range criteria.TestNames {
		testNames[testName] = struct{}{}
	}

	testNameExcludes := make(map[string]struct{})
	for _, testNameExclude := range criteria.TestNameExcludes {
		testNameExcludes[testNameExclude] = struct{}{}
	}
	return &tagMatcher{
		tags:             tags,
		excludes:         excludes,
		testNames:        testNames,
		testNameExcludes: testNameExcludes,
	}
}

// Only should be used in the case of special names which we expect partial matches for.
func (tm *tagMatcher) matchAndreturn(md *api.TestCaseMetadata) []string {
	matches := []string{}
	for test := range tm.testNames {
		newMatcher := &tagMatcher{
			tags:             tm.tags,
			excludes:         tm.excludes,
			testNames:        map[string]struct{}{test: {}},
			testNameExcludes: tm.testNameExcludes,
		}
		if newMatcher.match(md) {
			matches = append(matches, test)

		}

	}
	return matches
}
func (tm *tagMatcher) match(md *api.TestCaseMetadata) bool {
	if len(md.TestCase.Tags) < len(tm.tags) {
		return false
	}
	// If the test id is in ANY excluded names, do not match it
	for testNameExclude := range tm.testNameExcludes {
		if matched, _ := filepath.Match(testNameExclude, md.TestCase.Id.Value); matched {
			return false
		}
	}
	// If the test tag is in ANY of the excludes, do not match
	// Every tag in the matcher must be found in the TC.
	// EG: if a test has: `tag1, tag2`` and the matcher has `tag1, tag2, tag3` the matcher will not hit.
	// Additionally, if a test has: `tag1, tag2` and the matcher has `tag1, ag3`, the matcher will not hit.
	matchedTags := make(map[string]struct{})
	for _, tag := range md.TestCase.Tags {
		if _, ok := tm.excludes[tag.Value]; ok {
			return false
		}
		if _, ok := tm.tags[tag.Value]; ok {
			matchedTags[tag.Value] = struct{}{}
		}
	}

	// If the ANY names are provided, and the test matches ANY name, include it
	matchTestNames := len(tm.testNames) == 0
	for testName := range tm.testNames {
		// Just match the the name pre " ". We only want to match up to the first given space (for now)
		tName := getfirstName(testName)
		if matched, _ := filepath.Match(tName, md.TestCase.Id.Value); matched {
			matchTestNames = true
			break
		}
	}
	return len(matchedTags) == len(tm.tags) && matchTestNames
}

// MatchedTestsForSuites finds all test metadata that match the specified suites.
func MatchedTestsForSuites(metadataList []*api.TestCaseMetadata, suites []*api.TestSuite) (tmList []*api.TestCaseMetadata, err error) {
	// We will allow one metadata object to point to several tests.
	// this is for the case where we only know about a module level test, and not the children.
	tests := make(map[string][]string)
	var tagMatchers []*tagMatcher
	for _, s := range suites {
		tcIds := s.GetTestCaseIds()
		fmt.Println("looking for tcIds", tcIds)
		if tcIds != nil {
			for _, t := range tcIds.TestCaseIds {
				// Here is the given name.
				clean := getfirstName(t.Value)
				value, found := tests[clean]
				if !found {
					tests[clean] = []string{t.Value}
				} else {
					tests[clean] = append(value, t.Value)
				}
			}
		}
		criteria := s.GetTestCaseTagCriteria()
		if criteria != nil {
			// create one tag matcher for each test suite that has tags
			tagMatchers = append(tagMatchers, newTagMatcher(criteria))
		}
	}
	defer func() {
		// Get all the metadata for matched tests.
		for _, tm := range metadataList {
			// Here we build up the list from the MD, based on the testMatch
			if mappedCases, ok := tests[tm.GetTestCase().GetId().GetValue()]; ok {
				// In the normal case, this is not going to change much (1 request > 1 test)
				// In the case where several requests come in for 1 known entry, we need to copy the MD
				// and point the "ID" at the full value.
				for _, test := range mappedCases {
					copyRequired := tm.GetTestCase().GetId().GetValue() != test
					if copyRequired {
						md := proto.Clone(tm).(*api.TestCaseMetadata)
						md.TestCase.Id.Value = test
						tmList = append(tmList, md)
					} else {
						tmList = append(tmList, tm)
					}
				}
				delete(tests, tm.GetTestCase().GetId().GetValue())
			} else if _, ok := tests[getfirstName(tm.GetTestCase().GetId().GetValue())]; ok {
				// This codepath is hit in the edge case where a testname contains spaces.
				// getfirstName() will index the test case up to the first space, but will
				// still append the true test name as the value. We hit this issue in b/392702193.
				//
				// In order to address, this, assuming we fail the lookup of the whole test name,
				// we lookup the name of the testname up to the first space, it should match.
				// We then remove the entry in the slice if it exists. If the slice is empty, we
				// remove it.
				// This should only ever get called in Mobly test cases.
				keyToRemove := getfirstName(tm.GetTestCase().GetId().GetValue())
				valToRemove := tm.GetTestCase().GetId().GetValue()

				md := proto.Clone(tm).(*api.TestCaseMetadata)
				md.TestCase.Id.Value = valToRemove
				tmList = append(tmList, md)
				if slice, ok := tests[keyToRemove]; ok {
					indexToRemove := -1
					for i, str := range slice {
						if str == valToRemove {
							indexToRemove = i
							break
						}
					}
					if indexToRemove != -1 {
						slice[indexToRemove] = slice[len(slice)-1]
						slice = slice[:len(slice)-1]

						if len(slice) == 0 {
							delete(tests, keyToRemove)
						} else {
							tests[keyToRemove] = slice
						}
					}
				}
			}
		}
		if len(tests) > 0 {
			// There are unmatched tests cases.
			var unmatched []string
			for t := range tests {
				unmatched = append(unmatched, t)
			}
			err = fmt.Errorf("following test ids have no metadata %v", unmatched)
		}
	}()
	if len(tagMatchers) == 0 {
		return tmList, nil
	}

	for _, tm := range metadataList {
		clean := tm.GetTestCase().GetId().GetValue()
		if _, ok := tests[clean]; ok {
			// The test has already been included.
			continue
		}
		for _, matcher := range tagMatchers {
			// AL tests can come in like this:
			// names = ["module1 class1#case1", "module1 class1#case2"], where both can map back to the metadata for just
			// "module1". Thus we need to keep a map of [module1] = ["module1 class1#case1", "module1 class1#case2"]
			if ALSpecial(matcher.testNames) {
				matches := matcher.matchAndreturn(tm)
				tests[tm.GetTestCase().GetId().GetValue()] = matches
			} else {
				if matcher.match(tm) {
					tests[tm.GetTestCase().GetId().GetValue()] = []string{tm.GetTestCase().GetId().GetValue()}
				}
				break
			}
		}
	}
	return tmList, nil
}

// Get the first name when a name has a " "(space) in it.
func getfirstName(id string) string {
	return strings.Split(id, " ")[0]
}

func ALSpecial(testNames map[string]struct{}) bool {
	for testName := range testNames {
		if testName != getfirstName(testName) {
			return true
		}
	}
	return false
}
