// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package service provides the API handlers for ants publish.
package service

import (
	"context"
	"fmt"
	"log"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/api/metadata"
	common_utils "go.chromium.org/chromiumos/test/publish/cmd/common-utils"

	androidlib "infra/cros/cmd/common_lib/android_api"
	ants "infra/cros/cmd/common_lib/ants/androidbuildinternal/v3"
)

type AntsPublishService struct {
	metadata *metadata.PublishAntsMetadata
	results  []*api.TestCaseResult
	service  *androidlib.Service
}

// NewAntsPublishService creates a new publish service to interact with Ants.
func NewAntsPublishService(ctx context.Context, req *api.PublishRequest) (*AntsPublishService, error) {
	m, err := unpackMetadata(req)
	if err = validateAntsPublishRequest(req); err != nil {
		return nil, err
	}

	s, err := androidlib.NewAndroidBuildService(ctx, androidlib.CONTAINER_SATLAB)
	if err != nil {
		return nil, err
	}

	return &AntsPublishService{
		metadata: m,
		results:  req.GetTestResponse().GetTestCaseResults(),
		service:  s,
	}, nil
}

// createInvocation is used to create a default invocation
// This is used for testing only. Invocation Id should be received from ATP/CTP
func (aps *AntsPublishService) createInvocation() error {
	log.Println("creating invocation")

	build := &ants.BuildDescriptor{
		Branch:      "git_main-al-dev",
		BuildTarget: "brya-trunk_staging-userdebug",
		BuildId:     "12425286",
	}
	inv := &ants.Invocation{
		PrimaryBuild: build,
	}

	invocation, err := aps.service.InvocationService.Insert(inv)
	if err != nil {
		log.Println(err)
		return err
	}
	log.Println(invocation)
	return nil
}

func (aps *AntsPublishService) insertModuleWorkUnit(name string, wuType string) (*ants.WorkUnit, error) {
	wu := &ants.WorkUnit{
		Name:         name,
		Type:         wuType,
		ParentId:     aps.metadata.GetParentWorkUnitId(),
		InvocationId: aps.metadata.GetAntsInvocationId(),
	}

	return aps.service.WorkUnitService.Insert(wu)
}

// UploadToAnts uploads test results to Ants.
func (aps *AntsPublishService) UploadToAnts(ctx context.Context) error {
	log.Printf("Uploading to AnTS")

	// If we need to test locally, we can use aps.createInvocation() to test
	// everything below.

	modules := make(map[string]*ants.WorkUnit)
	testCases := make(map[string]*ants.WorkUnit)

	for _, result := range aps.results {
		names := strings.Split(result.GetTestCaseId().GetValue(), ".")
		moduleName := names[0]
		if _, ok := modules[moduleName]; !ok {
			mwu, err := aps.insertModuleWorkUnit(moduleName, "TF_MODULE")
			if err != nil {
				return err
			}
			modules[moduleName] = mwu
		}

		tcName := strings.Join(names[1:len(names)-1], ".")
		if _, ok := testCases[tcName]; !ok {
			tcwu, err := aps.insertModuleWorkUnit(tcName, "TF_TESTCASE")
			if err != nil {
				return err
			}
			testCases[tcName] = tcwu
		}

		testName := names[(len(names) - 1)]
		tr := &ants.TestResult{
			InvocationId: aps.metadata.GetAntsInvocationId(),
			WorkUnitId:   testCases[tcName].Id,
			TestIdentifier: &ants.TestIdentifier{
				Module:    moduleName,
				TestClass: testName,
			},
			TestStatus: antsTestStatus(result),
		}

		result, err := aps.service.TestResultService.Insert(tr)
		if err != nil {
			return err
		}
		log.Println("Insert result: ", result)
	}

	// Return nil error response to indicate success.
	return nil
}

func validateAntsPublishRequest(req *api.PublishRequest) error {
	if err := common_utils.ValidateGenericPublishRequest(req); err != nil {
		return err
	}

	if len(req.GetTestResponse().GetTestCaseResults()) == 0 {
		return fmt.Errorf("no test responses found")
	}

	m, err := unpackMetadata(req)
	if err != nil {
		return err
	}

	if m.GetAntsInvocationId() == "" {
		return fmt.Errorf("ants invocation id is required")
	} else if m.GetParentWorkUnitId() == "" {
		return fmt.Errorf("parent workunit id is required")
	} else if m.GetAccountId() == "" {
		return fmt.Errorf("partner account id is required")
	}

	return nil
}

// unpackMetadata unpacks the Any metadata field into PublishGcsMetadata
func unpackMetadata(req *api.PublishRequest) (*metadata.PublishAntsMetadata, error) {
	var m metadata.PublishAntsMetadata
	if err := req.Metadata.UnmarshalTo(&m); err != nil {
		return &m, fmt.Errorf("improperly formatted input proto metadata: %w", err)
	}
	return &m, nil
}
