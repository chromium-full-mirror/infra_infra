// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"container/list"
	"context"
	"encoding/json"
	"fmt"

	"golang.org/x/exp/slices"

	buildapi "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
)

var (
	TtcpContainerName          = "cros-ddd-filter" // ttcp-demo
	LegacyHWContainerName      = "cros-legacy-hw-filter"
	ProvisionContainerName     = "provision-filter"
	TestFinderContainerName    = "cros-test-finder"
	UseFlagFilterContainerName = "use_flag_filter"

	hwPlaceHolder = "PLACEHOLDER"
	// DefaultKarbonFilterNames defines Default karbon filters (SetDefaultFilters may add/remove)
	DefaultKarbonFilterNames = []string{TestFinderContainerName, ProvisionContainerName, hwPlaceHolder, UseFlagFilterContainerName}

	// DefaultKoffeeFilterNames defines Default koffee filters (SetDefaultFilters may add/remove)
	DefaultKoffeeFilterNames = []string{}

	// Default shas for backwards compatibility
	defaultLegacyHWSha      = "0d2d1b14940de10b4e1985065876357bf5bfe9d04f103892265d17e0b0b86350"
	defaultTTCPSha          = "8f31523fcf887aeeacf92c5ed6198ace0bbe3ffc2eef31d242a22a06dd45c42e"
	defaultUseFlagFilterSha = "9109345c742f5fc6c8575e6f41b36b480be70eb5f85b8098c04e3fa7398f0536"
	prodShas                = map[string]string{
		TtcpContainerName:          defaultTTCPSha,
		LegacyHWContainerName:      defaultLegacyHWSha,
		UseFlagFilterContainerName: defaultUseFlagFilterSha}

	binaryLookup = map[string]string{
		LegacyHWContainerName:      "legacy_hw_filter",
		TtcpContainerName:          "solver_service",
		TestFinderContainerName:    "test_finder_filter",
		UseFlagFilterContainerName: "use_flag_filter",
	}
)

// MakeDefaultFilters sets/appends proper default filters; in their required order.
func MakeDefaultFilters(ctx context.Context, suiteReq *api.SuiteRequest) []string {
	hwFilter := ""
	if suiteReq.GetDddSuite() {
		hwFilter = TtcpContainerName
	} else {
		hwFilter = LegacyHWContainerName
	}

	filters := []string{}
	for _, filter := range DefaultKarbonFilterNames {
		if filter == hwPlaceHolder {
			filters = append(filters, hwFilter)
		} else {
			filters = append(filters, filter)

		}
	}

	return filters
}

// GetDefaultFilters constructs ctp filters for provided default filters.
func GetDefaultFilters(ctx context.Context, defaultFilterNames []string, contMetadataMap map[string]*buildapi.ContainerImageInfo, build int) ([]*api.CTPFilter, error) {
	defaultFilters := make([]*api.CTPFilter, 0)
	logging.Infof(ctx, "Inside Default Filters: %s", defaultFilterNames)
	for _, filterName := range defaultFilterNames {
		logging.Infof(ctx, "Checking container metadata map for %s", filterName)
		// Attempt to map the filter from the known container metadata.
		ctpFilter, err := CreateCTPFilterWithContainerName(ctx, filterName, contMetadataMap, build, true)
		if err == nil {
			defaultFilters = append(defaultFilters, ctpFilter)
			continue
		}

		logging.Infof(ctx, "Inside backwards compat check.")
		// Test-Finder must always come from the contMetadataMap. Thus if we do not have the "filter" version,
		// We will setup to run the legacy test-finder.
		if filterName == TestFinderContainerName {
			TFFilter, err := CreateCTPFilterWithContainerName(ctx, TestFinderContainerName, contMetadataMap, build, false)
			if err != nil {
				return nil, errors.Annotate(err, "failed to create test-finder default filter").Err()
			}
			defaultFilters = append(defaultFilters, TFFilter)
			continue
		}

		// Otherwise, build the other default filters off prod containers.
		digest, ok := prodShas[filterName]
		if ok {
			logging.Infof(ctx, "Making default container for: %s", filterName)
			ctpFilter, err = CreateCTPDefaultWithContainerName(filterName, digest, build)
			if err != nil {
				return nil, errors.Annotate(err, "failed to create default default filter: ").Err()
			}
			defaultFilters = append(defaultFilters, ctpFilter)
			continue
		}
		return nil, errors.Annotate(err, "failed to create default filter: ").Err()
	}

	return defaultFilters, nil
}

func CreateCTPDefaultWithContainerName(name string, digest string, build int) (*api.CTPFilter, error) {
	c := CreateTestServicesContainer(name, digest)

	binaryName := binaryName(name, build)

	return &api.CTPFilter{ContainerInfo: &api.ContainerInfo{Container: c, BinaryName: binaryName}}, nil

}

func defaultName(ctx context.Context, name string) bool {
	logging.Infof(ctx, "checking name: ", name)
	for fn, defName := range binaryLookup {
		logging.Infof(ctx, "checking name: ", name)

		if name == defName || name == fn {
			return true
		}
	}
	return false
}

// CreateCTPFilterWithContainerName creates ctp filter for provided container name through provided container metadata.
func CreateCTPFilterWithContainerName(ctx context.Context, name string, contMetadataMap map[string]*buildapi.ContainerImageInfo, build int, buildCheck bool) (*api.CTPFilter, error) {
	// This error will be caught and pushed into the default prod container flow.
	if defaultName(ctx, name) && buildCheck && needBackwardsCompatibility(build) {
		return nil, fmt.Errorf("incompatible metadata build")
	}
	if _, ok := contMetadataMap[name]; !ok {
		return nil, errors.Reason("could not find container image info for %s in provided map", name).Err()
	}
	binaryName := binaryName(name, build)
	return &api.CTPFilter{ContainerInfo: &api.ContainerInfo{Container: contMetadataMap[name], BinaryName: binaryName}}, nil
}

// ConstructCtpFilters constructs default and non-default ctp filters.
func ConstructCtpFilters(ctx context.Context, defaultFilterNames []string, contMetadataMap map[string]*buildapi.ContainerImageInfo, filtersToAdd []*api.CTPFilter, build int) ([]*api.CTPFilter, error) {
	filters := make([]*api.CTPFilter, 0)

	// Add default filters
	logging.Infof(ctx, "Inside ConstructCtpFilters.")

	defFilters, err := GetDefaultFilters(ctx, defaultFilterNames, contMetadataMap, build)
	if err != nil {
		return filters, errors.Annotate(err, "failed to get default filters: ").Err()
	}
	logging.Infof(ctx, "After GetDefaultFilters. %s", defFilters)

	defFiltersIndexMap := map[string]int{}
	for i, defFilter := range defFilters {
		defFiltersIndexMap[defFilter.GetContainerInfo().GetContainer().GetName()] = i
	}

	nonDefFilters := []*api.CTPFilter{}
	for _, filter := range filtersToAdd {
		filterName := filter.GetContainerInfo().GetContainer().GetName()
		ctpFilter, err := CreateCTPFilterWithContainerName(ctx, filterName, contMetadataMap, build, true)
		if err != nil {
			logging.Infof(ctx, "failed to create ctp filter for %s", filterName)
			continue
		}
		// BinaryName is assumed to be same as FilterName.
		// If this is not the case, it can be resolved by the input.
		if filter.GetContainerInfo().GetBinaryName() != "" {
			ctpFilter.ContainerInfo.BinaryName = filter.GetContainerInfo().GetBinaryName()
		}
		ctpFilter.ContainerInfo.BinaryArgs = filter.GetContainerInfo().GetBinaryArgs()
		// Overwrite the default filter with the user defined filter.
		if slices.Contains(defaultFilterNames, filterName) {
			defFilters[defFiltersIndexMap[filterName]] = ctpFilter
		} else {
			nonDefFilters = append(nonDefFilters, ctpFilter)
		}
	}

	// Default filters run first, then non default filters.
	filters = append(defFilters, nonDefFilters...)

	return filters, nil
}

func binaryName(name string, build int) string {
	if name == TestFinderContainerName && needBackwardsCompatibility(build) {
		return "cros-test-finder"
	}

	binName, ok := binaryLookup[name]
	// If no name is found, then assume the container name is the same as the binary.
	// TODO expose the binary name and connect it from the input request.
	if !ok {
		return name
	}
	return binName
}

// CreateContainerRequest creates container request from provided ctp filter.
func CreateContainerRequest(requestedFilter *api.CTPFilter, build int) *api.ContainerRequest {
	return &api.ContainerRequest{
		DynamicIdentifier: requestedFilter.GetContainerInfo().GetContainer().GetName(),
		Container: &api.Template{
			Container: &api.Template_Generic{
				Generic: &api.GenericTemplate{
					// TODO (azrahman): Finalize the format of the this dir. Ideally, it should be /tmp/<container_name>.
					// So keeping it as comment for now.
					//DockerArtifactDir: fmt.Sprintf("/tmp/%s", filter.GetContainer().GetName()),
					DockerArtifactDir: "/tmp/filters",
					BinaryArgs: append([]string{
						"server", "-port", "0",
					}, requestedFilter.GetContainerInfo().GetBinaryArgs()...),
					BinaryName:        requestedFilter.GetContainerInfo().GetBinaryName(),
					AdditionalVolumes: []string{"/creds/service_accounts/:/creds/service_accounts/"},
				},
			},
		},
		// TODO (azrahman): figure this out (not being used right now).
		ContainerImageKey: requestedFilter.GetContainerInfo().GetContainer().GetName(),
		Network:           "host",
	}
}

func needBackwardsCompatibility(build int) bool {
	// TODO (dbeckett/azrahamn): set this to the proper build # once the compatibility
	// changes land in the OS src tree and have assigned build #s.
	return build < 16000
}

// CreateTTCPContainerRequest creates container request from provided ctp filter.
// TODO (azrahman): Merge this into a generic container request creator that will
// work for all containers.
func CreateTTCPContainerRequest(requestedFilter *api.CTPFilter) *api.ContainerRequest {
	return &api.ContainerRequest{
		DynamicIdentifier: requestedFilter.GetContainerInfo().GetContainer().GetName(),
		Container: &api.Template{
			Container: &api.Template_Generic{
				Generic: &api.GenericTemplate{
					// TODO (azrahman): Finalize the format of the this dir. Ideally, it should be /tmp/<container_name>.
					// So keeping it as comment for now.
					//DockerArtifactDir: fmt.Sprintf("/tmp/%s", filter.GetContainer().GetName()),
					DockerArtifactDir: "/tmp/filters",
					BinaryArgs: append([]string{
						"-port", "0",
						"-log", "/tmp/filters",
						"-creds", "/creds/service_accounts/service-account-chromeos.json",
					}, requestedFilter.GetContainerInfo().GetBinaryArgs()...),
					// TODO (azrahman): Get binary name from new field of CTPFilter proto.
					BinaryName:        "/solver_service",
					AdditionalVolumes: []string{"/creds/service_accounts/:/creds/service_accounts/"},
				},
			},
		},
		// TODO (azrahman): figure this out (not being used right now).
		ContainerImageKey: requestedFilter.GetContainerInfo().GetContainer().GetName(),
		Network:           "host",
	}
}

// ListToJson creates json bytes from provided list.
func ListToJson(list *list.List) []byte {
	retBytes := make([]byte, 0)
	for e := list.Front(); e != nil; e = e.Next() {
		bytes, _ := json.MarshalIndent(e, "", "\t")
		retBytes = append(retBytes, bytes...)
	}

	return retBytes
}
