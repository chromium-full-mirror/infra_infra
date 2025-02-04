// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"fmt"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"
	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/luciexe/build"
)

// AddLinksToStepSummaryMarkdown adds provided links to provided step summary.
func AddLinksToStepSummaryMarkdown(
	step *build.Step,
	testhausURL, gcsLink, antsInvocationID string, isProd bool) {

	links := []string{}
	if testhausURL != "" {
		links = append(links, fmt.Sprintf("* [Testhaus Link](%s)", testhausURL))
	}
	if gcsLink != "" {
		links = append(links, fmt.Sprintf("* [Test Artifacts Gcs Link](%s)", gcsLink))
	}
	if antsInvocationID != "" {
		var apiStack string
		// Append the url field in staging.
		if !isProd {
			apiStack = "?api-stack=atp"
		}
		links = append(links, fmt.Sprintf("* [ATI Results](%s/%s/%s)", ATILink, antsInvocationID, apiStack))
	}
	if len(links) > 0 {
		step.SetSummaryMarkdown(strings.Join(links, "\n"))
	}
}

// CreateStepWithStatus creates a new step and sets step status based on
// provided flags. If failParent is true, the returned error will have build
// failure status attached to it for caller to bubble up appropriately.
func CreateStepWithStatus(
	ctx context.Context,
	stepName string,
	summary string,
	isFailure bool,
	failParentStep bool) (err error) {

	if stepName == "" {
		return nil
	}

	var stepErr error
	step, _ := build.StartStep(ctx, stepName)
	defer func() {
		step.End(build.AttachStatus(stepErr, bbpb.Status_FAILURE, nil))
	}()

	if summary != "" {
		step.SetSummaryMarkdown(summary)
	}

	if isFailure {
		stepErr = fmt.Errorf("%s: %s", stepName, summary)
	}

	if isFailure && failParentStep {
		err = stepErr
	}

	return err
}

func GetFlattenedTestCases(ctpTestCases []*api.TestCase_Id) []string {
	testCasesNames := []string{}
	for _, testCase := range ctpTestCases {
		testCasesNames = append(testCasesNames, testCase.GetValue())
	}

	return testCasesNames
}

func ExtractModulesIfSubModulesProvided(testCases []string) []string {
	resultedTcs := []string{}
	moduleMap := map[string]bool{}
	for _, testCase := range testCases {
		moduleName := strings.Split(testCase, " ")[0]
		if _, ok := moduleMap[moduleName]; !ok {
			moduleMap[moduleName] = true
			resultedTcs = append(resultedTcs, moduleName)
		}
	}

	return resultedTcs
}
