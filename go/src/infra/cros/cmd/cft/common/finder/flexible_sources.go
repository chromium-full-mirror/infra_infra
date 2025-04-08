// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package finder find all matched tests from test metadata based on test criteria.
package finder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"cloud.google.com/go/storage"

	"go.chromium.org/chromiumos/config/go/test/api"
)

func GetTFSourceData(ctx context.Context, gcsBasePath string) ([][]byte, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage.NewClient: %w", err)
	}
	defer client.Close()

	bucket := client.Bucket(gcsBasePath)

	data, err := PullAllFilesFromGcsDir(ctx, bucket, "", "latest.json")
	if err != nil {
		return nil, err
	}
	return data, nil
}

func GetSourceData(ctx context.Context, gcsBasePath string) ([][]byte, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage.NewClient: %w", err)
	}
	defer client.Close()

	bucketName, pf, err := ExtractBucketAndPrefixFromPath(gcsBasePath)
	if err != nil {
		return nil, err
	}

	bucket := client.Bucket(bucketName)

	// Currently G3Mobly content is fetched off the latest.
	// TODO, we probably should expose a way to pass in a desired path to allow repeatability on tests here.
	newestDir, err := FindNewestDirInGcsBucket(ctx, gcsBasePath, bucket, pf)
	if err != nil {
		return nil, err
	}
	data, err := PullAllFilesFromGcsDir(ctx, bucket, newestDir, ".json")
	if err != nil {
		return nil, err
	}
	return data, nil
}

func FindNewestDirInGcsBucket(ctx context.Context, gcsBasePath string, bucket *storage.BucketHandle, pf string) (string, error) {
	// List subdirectories under the prefix directory.
	it := bucket.Objects(ctx, &storage.Query{Prefix: pf, Delimiter: "/"})

	var subdirs []string
	for {
		attrs, err := it.Next()
		if errors.Is(err, io.EOF) {
			fmt.Println("eof")
			break
		}
		if err != nil {
			break
		}
		if attrs.Prefix != "" && strings.HasPrefix(path.Base(attrs.Prefix), "cros") {
			subdirs = append(subdirs, attrs.Prefix)
		}
	}

	if len(subdirs) == 0 {
		return "", fmt.Errorf("no subdirectories found under %s", gcsBasePath)
	}

	// Sort subdirectories to find the newest one (assuming lexicographical order corresponds to time).
	sort.Strings(subdirs)
	newestDir := subdirs[len(subdirs)-1]
	return newestDir, nil
}

// PullAllFilesFromGcsDir will grab all the files from a dir matching the postfix
func PullAllFilesFromGcsDir(ctx context.Context, bucket *storage.BucketHandle, dir string, postfix string) ([][]byte, error) {
	it := bucket.Objects(ctx, &storage.Query{Prefix: dir})
	var data [][]byte // Accumulate data from all files
	for {
		attrs, err := it.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			break
		}
		fmt.Println(attrs.Name)
		if strings.HasSuffix(attrs.Name, postfix) {
			r, err := bucket.Object(attrs.Name).NewReader(ctx)
			if err != nil {
				return nil, fmt.Errorf("creating reader for %s: %w", attrs.Name, err)
			}
			defer r.Close()

			fileBytes, err := io.ReadAll(r)
			if err != nil {
				return nil, fmt.Errorf("reading data from %s: %w", attrs.Name, err)
			}
			data = append(data, fileBytes) // Append data from current file
		}
	}

	if data == nil {
		return nil, fmt.Errorf("no files found in newest directory %s", dir)
	}
	return data, nil
}

func ExtractBucketAndPrefixFromPath(gcsPath string) (bucketName, prefix string, err error) {
	parts := strings.SplitN(gcsPath, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid GCS path: %s", gcsPath)
	}
	bucketName = parts[0]
	prefix = fmt.Sprintf("%s/", parts[1])
	return
}

type TestInfo struct {
	Name      string      `json:"name"`
	Tags      []string    `json:"tags"`
	ExtraInfo []ExtraInfo `json:"extra_info"`
}

type ExtraInfo struct {
	ExecutableName string `json:"executable_name"`
}

func TranslateMoblySrcToMetadata(src [][]byte) (metaData []*api.TestCaseMetadata) {
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
			ExtraInfo: createExtraInfo(testInfo),
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
