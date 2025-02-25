// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package finders

import (
	"encoding/json"
	"fmt"

	"go.chromium.org/chromiumos/config/go/test/api"
)

type TFTestInfo struct {
	Version     string        `json:"version"`
	Suite       string        `json:"suite"`
	Branch      string        `json:"branch"`
	Targets     []string      `json:"targets"`
	BuildID     string        `json:"build_id"`
	Modules     []Module      `json:"modules"`
	TargetBuild []TargetBuild `json:"target_build"`
}

type Module struct {
	Name         string   `json:"name"`
	Parameters   []string `json:"parameters"`
	Abis         []string `json:"abis"`
	BugComponent []int32  `json:"bug_components"`
}

type TargetBuild struct {
	Target  string `json:"target"`
	BuildId string `json:"build_id"`
	Abi     string `json:"abi"`
}

func TranslateTFSrcToMetadata(src [][]byte) (metaData []*api.TestCaseMetadata) {
	for _, srcInfo := range src {
		var testInfoListLocal TFTestInfo
		err := json.Unmarshal(srcInfo, &testInfoListLocal)
		if err != nil {
			fmt.Println("issue", err)
			return nil
		}

		for _, testInfo := range testInfoListLocal.Modules {
			metaData = append(metaData, createTestCaseFromTFTestInfo(testInfo, testInfoListLocal.Suite, testInfoListLocal.Branch, testInfoListLocal.Targets, testInfoListLocal.BuildID, testInfoListLocal.TargetBuild, testInfoListLocal.Version))
		}
	}

	return metaData

}

func createTestCaseFromTFTestInfo(testInfo Module, suiteName string, branch string, targets []string, buildID string, targetBuilds []TargetBuild, version string) *api.TestCaseMetadata {
	return &api.TestCaseMetadata{
		TestCase: &api.TestCase{
			Name: formatName(testInfo.Name, suiteName),
			Tags: formatTFTags(suiteName, branch, buildID, testInfo.Parameters, targetBuilds, testInfo.Abis, version, targets),
			Id: &api.TestCase_Id{
				Value: fmt.Sprintf("tradefed.%s", formatName(testInfo.Name, suiteName)),
			},
		},
		TestCaseInfo: &api.TestCaseInfo{
			Owners:       []*api.Contact{},
			BugComponent: bugComponent(testInfo.BugComponent),
		},
		TestCaseExec: &api.TestCaseExec{
			TestHarness: &api.TestHarness{
				TestHarnessType: &api.TestHarness_Tradefed_{
					Tradefed: &api.TestHarness_Tradefed{},
				},
			},
		},
	}
}
func bugComponent(bugComponents []int32) *api.BugComponent {
	var bugComponentList []*api.BugComponent
	for _, bugComponent := range bugComponents {
		bugComponentList = append(bugComponentList, &api.BugComponent{
			Value: fmt.Sprintf("%d", bugComponent),
		})
	}
	if len(bugComponentList) == 0 {
		return nil
	} else {
		return bugComponentList[0]
	}

}

func formatName(name string, suite string) string {
	return fmt.Sprintf("%s.%s", suite, name)

}

func formatTFTags(
	suiteName string,
	branch string,
	buildID string,
	parameters []string,
	targetBuilds []TargetBuild,
	abis []string,
	version string,
	targets []string) (tagList []*api.TestCase_Tag) {
	for _, param := range parameters {
		tagList = append(tagList, &api.TestCase_Tag{Value: fmt.Sprintf("param:%s", param)})
	}
	if suiteName != "" {
		tagList = append(tagList, &api.TestCase_Tag{Value: fmt.Sprintf("suite:%s", suiteName)})
	}
	if branch != "" {
		tagList = append(tagList, &api.TestCase_Tag{Value: fmt.Sprintf("branch:%s", branch)})
	}
	if buildID != "" {
		tagList = append(tagList, &api.TestCase_Tag{Value: fmt.Sprintf("build_id:%s", buildID)})
	}

	for _, target := range targets {
		tagList = append(tagList, &api.TestCase_Tag{Value: fmt.Sprintf("target:%s", target)})
	}

	if version == "1.5" && len(targetBuilds) > 0 {
		for _, targetBuild := range targetBuilds {
			tagList = append(tagList, &api.TestCase_Tag{
				Value: fmt.Sprintf("target_build=target:%s,build_id:%s,abi:%s", targetBuild.Target, targetBuild.BuildId, targetBuild.Abi)},
			)
		}
	} else {
		for _, abi := range abis {
			tagList = append(tagList, &api.TestCase_Tag{Value: fmt.Sprintf("build_id:%s", abi)})
		}
	}

	return tagList
}

// func createExtraInfo(testInfo TestInfo) map[string]string {
// 	extraInfo := make(map[string]string)
// 	extraInfo["executable_name"] = testInfo.ExtraInfo[0].ExecutableName
// 	return extraInfo
// }
