// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package run

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"go.chromium.org/chromiumos/infra/proto/go/test_platform"

	"go.chromium.org/infra/cmd/crosfleet/internal/common"
)

var testTestPlanForTestsData = []struct {
	testArgs, testHarness string
	testNames             []string
	wantTestPlan          *test_platform.Request_TestPlan
}{
	{"",
		"",
		[]string{"foo"},
		&test_platform.Request_TestPlan{
			Test: []*test_platform.Request_Test{
				{
					Harness: &test_platform.Request_Test_Autotest_{
						Autotest: &test_platform.Request_Test_Autotest{
							Name: "foo",
						},
					},
				},
			},
		},
	},
	{"foo=bar",
		"",
		[]string{"foo", "bar"},
		&test_platform.Request_TestPlan{
			Test: []*test_platform.Request_Test{
				{
					Harness: &test_platform.Request_Test_Autotest_{
						Autotest: &test_platform.Request_Test_Autotest{
							Name:     "foo",
							TestArgs: "dummy=crbug/984103 foo=bar",
						},
					},
				},
				{
					Harness: &test_platform.Request_Test_Autotest_{
						Autotest: &test_platform.Request_Test_Autotest{
							Name:     "bar",
							TestArgs: "dummy=crbug/984103 foo=bar",
						},
					},
				},
			},
		},
	},
	{"",
		"foo-harness",
		[]string{"bar", "baz"},
		&test_platform.Request_TestPlan{
			Test: []*test_platform.Request_Test{
				{
					Harness: &test_platform.Request_Test_Autotest_{
						Autotest: &test_platform.Request_Test_Autotest{
							Name: "foo-harness.bar",
						},
					},
				}, {
					Harness: &test_platform.Request_Test_Autotest_{
						Autotest: &test_platform.Request_Test_Autotest{
							Name: "foo-harness.baz",
						},
					},
				},
			},
		},
	},
}

func TestTestPlanForTests(t *testing.T) {
	t.Parallel()
	for _, tt := range testTestPlanForTestsData {
		tt := tt
		t.Run(fmt.Sprintf("(%s/%s/%s)", tt.testArgs, tt.testHarness, tt.testNames), func(t *testing.T) {
			t.Parallel()
			gotTestPlan := testPlanForTests(tt.testArgs, tt.testHarness, tt.testNames, 0)
			if diff := cmp.Diff(tt.wantTestPlan, gotTestPlan, common.CmpOpts); diff != "" {
				t.Errorf("unexpected diff (%s)", diff)
			}
		})
	}
}
