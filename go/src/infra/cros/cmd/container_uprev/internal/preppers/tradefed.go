// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package preppers contains preppers for configured containers.
package preppers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/golang/protobuf/proto"
	"google.golang.org/api/option"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"infra/cros/cmd/common_lib/common"
)

const (
	TradefedMetadataFile = "tradefed_metadata.pb"
)

type Module struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Abis          []string `json:"abis"`
	Parameters    []string `json:"parameters"`
	BugComponents []int    `json:"bug_components"`
	Owners        []string `json:"owners"`
}

type TargetBuild struct {
	Target  string `json:"target"`
	BuildId string `json:"build_id"`
	Abi     string `json:"abi"`
}

type TradefedMetadata struct {
	Version      string        `json:"version"`
	Suite        string        `json:"suite"`
	Branch       string        `json:"branch"`
	BuildId      string        `json:"build_id"`
	Targets      []string      `json:"targets"`
	Modules      []Module      `json:"modules"`
	TargetBuilds []TargetBuild `json:"target_builds"`
}

func parseTradefedMetadataJson(file string) (*TradefedMetadata, error) {
	metadataData, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("Error reading Tradefed metadata file: %w", err)
	}

	metadata := &TradefedMetadata{}
	if err := json.Unmarshal(metadataData, metadata); err != nil {
		return nil, fmt.Errorf("Error parsing json: %w", err)
	}

	return metadata, nil
}

func compileMetadata(metadata *TradefedMetadata) []*api.TestCaseMetadata {
	md := []*api.TestCaseMetadata{}
	for _, module := range metadata.Modules {
		tcName := metadata.Suite + "." + module.Name
		tags := []*api.TestCase_Tag{
			{Value: "suite:" + metadata.Suite},
			{Value: "branch:" + metadata.Branch},
			{Value: "build_id:" + metadata.BuildId},
		}
		for _, target := range metadata.Targets {
			tags = append(tags, &api.TestCase_Tag{Value: "target:" + target})
		}
		for _, param := range module.Parameters {
			tags = append(tags, &api.TestCase_Tag{Value: "param:" + param})
		}
		if len(metadata.TargetBuilds) > 0 {
			for _, tb := range metadata.TargetBuilds {
				tbTag := fmt.Sprintf("target_build=target:%s,build_id:%s,abi:%s", tb.Target, tb.BuildId, tb.Abi)
				tags = append(tags, &api.TestCase_Tag{Value: tbTag})
			}
		} else {
			for _, abi := range module.Abis {
				tags = append(tags, &api.TestCase_Tag{Value: "abi:" + abi})
			}
		}
		testCase := &api.TestCase{
			Id:   &api.TestCase_Id{Value: "tradefed." + tcName},
			Name: tcName,
			Tags: tags,
		}

		contacts := []*api.Contact{}
		for _, owner := range module.Owners {
			contacts = append(contacts, &api.Contact{Email: owner})
		}
		testCaseInfo := &api.TestCaseInfo{Owners: contacts}
		if len(module.BugComponents) > 0 {
			testCaseInfo.BugComponent = &api.BugComponent{Value: strconv.Itoa(module.BugComponents[0])}
		}

		testHarness := &api.TestHarness{TestHarnessType: &api.TestHarness_Tradefed_{Tradefed: &api.TestHarness_Tradefed{}}}
		testCaseMetadata := &api.TestCaseMetadata{
			TestCase:     testCase,
			TestCaseInfo: testCaseInfo,
			TestCaseExec: &api.TestCaseExec{TestHarness: testHarness},
		}
		md = append(md, testCaseMetadata)
	}

	return md
}

// InternalTF implements the prepper for tradefed internal container.
func InternalTF(ctx context.Context, dir string) error {
	downloadOptions := []option.ClientOption{}
	credentialFile := common.VMLabDockerKeyFileLocation // common.LabDockerKeyFileLocation
	if _, err := os.Stat(credentialFile); err == nil {
		downloadOptions = append(downloadOptions, option.WithCredentialsFile(credentialFile))
	}

	// 1. Download metadata JSONs from GCS.
	fileNames := map[string]string{
		"cts.json":     "gs://cros-xts-metadata/cts-metadata-git_main-latest.json",
		"dts.json":     "gs://cros-xts-metadata/dts-metadata-git_main-al-dev-latest.json",
		"vts.json":     "gs://cros-xts-metadata/vts-metadata-git_main-latest.json",
		"gts.json":     "gs://cros-xts-metadata/gts-metadata-git_main-latest.json",
		"general.json": "gs://cros-xts-metadata/general-metadata-git_main-latest.json",
	}
	var mdFiles []string
	for lp, gp := range fileNames {
		localFilePath := filepath.Join(dir, lp)
		mdFiles = append(mdFiles, localFilePath)
		logging.Infof(ctx, "TF prepper: downloading metadata file %q", gp)
		if err := common.DownloadGcsFileAsLocalFile(ctx, gp, localFilePath, downloadOptions...); err != nil {
			return errors.Annotate(err, "TF prepper: download %q", gp).Err()
		}
		logging.Infof(ctx, "TF prepper: downloaded as %q", localFilePath)
	}

	// 2. Compile metadata
	md := []*api.TestCaseMetadata{}
	for _, lp := range mdFiles {
		if tfMetadata, err := parseTradefedMetadataJson(lp); err == nil {
			md = append(md, compileMetadata(tfMetadata)...)
		} else {
			return errors.Annotate(err, "TF prepper: compile %q", lp).Err()
		}
	}

	// 3. Write down compiled metadata to file.
	outPath := filepath.Join(dir, TradefedMetadataFile)
	logging.Infof(ctx, "TF prepper: writing to %q", outPath)
	bytes, err := proto.Marshal(&api.TestCaseMetadataList{Values: md})
	if err != nil {
		return errors.Annotate(err, "TF prepper: marshalling proto failed").Err()
	}
	return os.WriteFile(outPath, bytes, 0600)
}
