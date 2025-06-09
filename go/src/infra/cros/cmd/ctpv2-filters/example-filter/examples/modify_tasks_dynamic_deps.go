// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package examples

import (
	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates"
	dynamiccommon "go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/generators"
)

func ModifyTestTasksDynamicDeps(req *api.InternalTestplan) error {
	generator := generators.NewModifyGenerator(
		dynamiccommon.FindByDynamicIdentifier(common.CrosTest))

	generator.AddModification(
		// Translation:
		//	Inside the test's request, modify the test case ids to
		//	include a new test case, example_test_case.
		&api.DynamicDep{
			Key:   "testRequest.testSuites.testCaseIds.testCaseIds",
			Value: `JSON={"value":"example_test_case"}`,
		},
		map[string]string{
			// Translation:
			//	At Test's Dynamic Deps, insert the full modification payload,
			// 	which is a dynamic dep. FYI: Pointing a singular object to an array
			//	will append the object to the array.
			"test.dynamicDeps": "",
		},
	)

	return dynamic_updates.AppendUserDefinedDynamicUpdates(&req.SuiteInfo.SuiteMetadata.DynamicUpdates, generator.Generate)
}
