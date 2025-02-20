// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package finders contains the implementations of the abstract finder interface.
package finders

import (
	"fmt"
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/test-finder/common"
)

// intentionally commented out. Useful for debugging E2E on real artifacts, but not suited for a real unittest.
// func TestFoo(t *testing.T) {
// 	te, err := getSourceData(context.Background(), "mobly_priv_artifacts/out")
// 	fmt.Println(err)

// 	metadata := translateSrcToMetadata(te)
// 	// fmt.Println(metadata)

// 	internalTestPlan := &api.InternalTestplan{
// 		SuiteInfo: common.GenerateSuiteInfo(),
// 	}
// 	tests, err := matchTests(metadata, internalTestPlan)
// 	if err != nil {
// 		fmt.Println(err)
// 	}
// 	fmt.Println(tests)

// 	t.Fatalf("intentionalFail")

// 	return
// }

func TestSuiteFromTestplan(t *testing.T) {
	internalTestPlan := &api.InternalTestplan{
		SuiteInfo: common.GenerateSuiteInfo(),
	}
	fmt.Println(internalTestPlan)
	tests, err := matchTests(common.GenerateTestMetadata(), internalTestPlan)
	if err != nil {
		t.Fatalf("got err: %s", err)
	}
	fmt.Println(tests)
	if len(tests) != 1 {
		t.Fatal("Found incorrect num of tests")
	}
	if tests[0].GetTestCase().GetName() != "fakeTest1" {
		t.Fatalf("wrong test matched")
	}

}
