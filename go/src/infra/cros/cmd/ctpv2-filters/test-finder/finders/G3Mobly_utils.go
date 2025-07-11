// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package finders

import (
	"encoding/json"
	"fmt"

	"go.chromium.org/chromiumos/config/go/test/api"
)

type TestInfo struct {
	Name         string      `json:"name"`
	Tags         []string    `json:"tags"`
	ExtraInfo    []ExtraInfo `json:"extra_info"`
	BugComponent []int32     `json:"bug_components"`
}

type ExtraInfo struct {
	ExecutableName string `json:"executable_name"`
}

func TranslateG3SrcToMetadata(src [][]byte) (metaData []*api.TestCaseMetadata) {
	var testInfoList []TestInfo

	for _, srcInfo := range src {
		var testInfoListLocal []TestInfo
		err := json.Unmarshal(srcInfo, &testInfoListLocal)
		testInfoList = append(testInfoList, testInfoListLocal...)
		if err != nil {
			return nil
		}
	}

	for _, testInfo := range testInfoList {
		tc := createTestCaseFromTestInfo(testInfo)
		metaData = append(metaData, tc)
	}
	return metaData
}

func createTestCaseFromTestInfo(testInfo TestInfo) *api.TestCaseMetadata {
	return &api.TestCaseMetadata{
		TestCase: &api.TestCase{
			Name: testInfo.Name,
			Tags: formatTags(testInfo.Tags),
			Id: &api.TestCase_Id{
				Value: testInfo.Name,
			},
		},
		TestCaseInfo: &api.TestCaseInfo{
			ExtraInfo:    createExtraInfo(testInfo),
			BugComponent: createBugComponent(testInfo.BugComponent),
		},
		TestCaseExec: &api.TestCaseExec{
			TestHarness: &api.TestHarness{
				TestHarnessType: &api.TestHarness_Mobly_{
					Mobly: &api.TestHarness_Mobly{},
				},
			},
		},
	}
}

func formatTags(tags []string) (tagList []*api.TestCase_Tag) {
	for _, tag := range tags {
		tagList = append(tagList, &api.TestCase_Tag{Value: tag})
	}
	return tagList
}

func createExtraInfo(testInfo TestInfo) map[string]string {
	extraInfo := make(map[string]string)
	extraInfo["executable_name"] = testInfo.ExtraInfo[0].ExecutableName
	return extraInfo
}

func createBugComponent(bugComponents []int32) *api.BugComponent {
	var bugComponentList []*api.BugComponent
	for _, bugComponent := range bugComponents {
		bugComponentList = append(bugComponentList, &api.BugComponent{
			Value: fmt.Sprintf("%d", bugComponent),
		})
	}
	if len(bugComponentList) == 0 {
		return nil
	}

	return bugComponentList[0]
}
