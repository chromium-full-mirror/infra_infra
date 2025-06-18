// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"container/list"
	"context"
	"encoding/json"
	"fmt"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"

	buildapi "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/logging"
)

var (
	TtcpContainerName                    = "cros-ddd-filter" // ttcp-demo
	LegacyHWContainerName                = "cros-legacy-hw-filter"
	AshChromeProvisionContainerName      = "ash-chrome-provision-filter"
	ProvisionContainerName               = "provision-filter"
	TestFinderContainerName              = "cros-test-finder"
	ALTestFinderName                     = "test-finder"
	UseFlagFilterContainerName           = "use_flag_filter"
	PreProcessFilterContainerName        = "pre_process_filter"
	AutoVMTestShifterFilterContainerName = "autovm_test_shifter_filter"
	PartnerStagingContainerName          = "partner-staging"
	LsNexusFilterContainerName           = "lsnexus-filter"

	hwPlaceHolder = "PLACEHOLDER"
	// DefaultKarbonFilterNames defines Default karbon filters (SetDefaultFilters may add/remove)
	DefaultKarbonFilterNames = []string{TestFinderContainerName, ProvisionContainerName, hwPlaceHolder, LsNexusFilterContainerName}
)

type FilterAuthInterface interface {
	GetTokenSource(credentialPaths []string, scopes ...string) oauth2.TokenSource
}

type CommonFilterParams struct {
	// Common supporting structs
	AuthHelper FilterAuthInterface

	// Common Args
	FirestoreDatabaseName string
	Environment           string
}

func GetDefaultFilterContainerImageInfosMap(ctx context.Context, creds, ctpVersion string, defaultFilterNames []string, contMetadataMap map[string]*buildapi.ContainerImageInfo, build int, firestoreDBName string) map[string]*buildapi.ContainerImageInfo {
	defaultFilters := map[string]*buildapi.ContainerImageInfo{}

	for _, defaultFilterName := range defaultFilterNames {
		logging.Infof(ctx, "Getting default filter for %s", defaultFilterName)

		// Try and grab the filter from the firestore DB
		// of infra/infra containers.
		if containerInfo, err := FetchContainerInfoFromFirestore(ctx, firestoreDBName, ctpVersion, defaultFilterName, option.WithCredentialsFile(creds)); err == nil && containerInfo != nil {
			logging.Infof(ctx, "Found filter inside the firestore for %s", defaultFilterName)
			if containerInfo.GetContainer().GetName() == "" {
				containerInfo.Container.Name = defaultFilterName
			}
			defaultFilters[defaultFilterName] = containerInfo.GetContainer()
			continue
		}

		// If not found, expect to be in the build's container metadata
		// which will be checked at the time of CTPFilter construction.
	}

	return defaultFilters
}

// MakeDefaultFilters sets/appends proper default filters; in their required order.
func MakeDefaultFilters(ctx context.Context, suiteReq *api.SuiteRequest, experiments []string, isPartner, isAlRun, hasAshChrome bool) []*api.CTPFilter {
	hwFilter := ""
	if suiteReq.GetDddSuite() {
		hwFilter = TtcpContainerName
	} else {
		hwFilter = LegacyHWContainerName
	}

	filters := []string{}
	if isPartner && !isAlRun {
		filters = append(filters, PartnerStagingContainerName)
	}
	for _, filter := range DefaultKarbonFilterNames {
		if filter == hwPlaceHolder {
			filters = append(filters, hwFilter)
		} else {
			filters = append(filters, filter)

		}
	}
	if hasAshChrome {
		filters = append(filters, AshChromeProvisionContainerName)
	}
	if isExperimentEnabled("chromeos.cros_infra_config.autovm_test_shifter", experiments) && isSuiteSchedulerConfig(suiteReq) && !isAlRun {
		filters = append(filters, AutoVMTestShifterFilterContainerName)
	}
	if !isAlRun {
		filters = append(filters, UseFlagFilterContainerName, PreProcessFilterContainerName)
	}

	ctpFilters := []*api.CTPFilter{}
	for _, filter := range filters {
		ctpFilters = append(ctpFilters, &api.CTPFilter{
			ContainerInfo: &api.ContainerInfo{
				Container: &buildapi.ContainerImageInfo{
					Name: filter,
				},
			},
		})
	}
	return ctpFilters
}

// ConstructCtpFilters constructs default and non-default ctp filters.
func ConstructCtpFilters(ctx context.Context, defaultFilter []*api.CTPFilter, filtersToAdd []*api.CTPFilter) []*api.CTPFilter {
	filters := make([]*api.CTPFilter, 0)

	// Add default filters
	defFiltersIndexMap := map[string]int{}
	for i, defaultFilter := range defaultFilter {
		defFiltersIndexMap[defaultFilter.GetContainerInfo().GetContainer().GetName()] = i
	}

	nonDefaultFilters := []*api.CTPFilter{}
	for _, filter := range filtersToAdd {
		filterName := filter.GetContainerInfo().GetContainer().GetName()
		// Special logic for swapping out cros-test-finder with the test-finder filter.
		if filterName == ALTestFinderName {
			filterName = TestFinderContainerName
		}
		if i, ok := defFiltersIndexMap[filterName]; ok {
			defaultFilter[i] = filter
		} else {
			nonDefaultFilters = append(nonDefaultFilters, filter)
		}
	}

	// Default filters run first, then non default filters.
	filters = append(defaultFilter, nonDefaultFilters...)

	return filters
}

// CreateContainerRequest creates container request from provided ctp filter.
func CreateContainerRequest(requestedFilter *api.CTPFilter) *api.ContainerRequest {
	imagePath, _ := CreateImagePath(requestedFilter.GetContainerInfo().GetContainer())

	return &api.ContainerRequest{
		DynamicIdentifier: requestedFilter.GetContainerInfo().GetContainer().GetName(),
		Container: &api.Template{
			Container: &api.Template_Generic{
				Generic: &api.GenericTemplate{
					// TODO (azrahman): Finalize the format of the this dir. Ideally, it should be /tmp/<container_name>.
					// So keeping it as comment for now.
					//DockerArtifactDir: fmt.Sprintf("/tmp/%s", filter.GetContainer().GetName()),
					DockerArtifactDir: "/tmp/filters",
					BinaryArgs: []string{
						"server", "-port", "0",
					},
					BinaryName:        requestedFilter.GetContainerInfo().GetBinaryName(),
					AdditionalVolumes: []string{"/creds/service_accounts/:/creds/service_accounts/"},
					Env:               GceMetadataEnvVars(),
				},
			},
		},
		// TODO (azrahman): figure this out (not being used right now).
		ContainerImageKey:  requestedFilter.GetContainerInfo().GetContainer().GetName(),
		ContainerImagePath: imagePath,
		Network:            "host",
	}
}

// ListToJSON creates json bytes from provided list.
func ListToJSON(list *list.List) []byte {
	retBytes := make([]byte, 0)
	for e := list.Front(); e != nil; e = e.Next() {
		bytes, _ := json.MarshalIndent(e, "", "\t")
		retBytes = append(retBytes, bytes...)
	}

	return retBytes
}

// isExperimentEnabled checks is a given exp is present in experiments list
func isExperimentEnabled(exp string, experiments []string) bool {
	for _, e := range experiments {
		if e == exp {
			return true
		}
	}
	return false
}

// isSuiteSchedulerConfig checks is a given request is coming from Suite scheduler config
func isSuiteSchedulerConfig(suiteReq *api.SuiteRequest) bool {
	return suiteReq.GetAnalyticsName() != ""
}

// ProcessContainerPath processes a provided path and determines whether it needs to
// pull from the firestoreDatabase provided.
func ProcessContainerPath(ctx context.Context, commonParams *CommonFilterParams, path, firestoreName string) (processedPath string, err error) {
	if path != "" {
		return path, nil
	}

	var env string
	switch commonParams.Environment {
	case Prod.String(), LabelProd:
		env = LabelProd
	default:
		env = LabelStaging
	}

	tokenSource := commonParams.AuthHelper.GetTokenSource(CTPv2DockerKeyFileLocations, DatastoreScope)
	testContainer, err := FetchFilterFromFirestore(ctx, commonParams.FirestoreDatabaseName, env, firestoreName, option.WithTokenSource(tokenSource))
	if err != nil {
		return "", fmt.Errorf("failed to fetch %s, %w", firestoreName, err)
	}
	processedPath, err = CreateImagePath(testContainer.GetContainerInfo().GetContainer())
	if err != nil {
		return "", fmt.Errorf("failed to create image path, %w", err)
	}

	return
}
