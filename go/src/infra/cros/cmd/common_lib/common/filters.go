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
	"golang.org/x/oauth2"
	"google.golang.org/api/option"

	buildapi "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
)

var (
	TtcpContainerName                    = "cros-ddd-filter" // ttcp-demo
	LegacyHWContainerName                = "cros-legacy-hw-filter"
	ProvisionContainerName               = "provision-filter"
	TestFinderContainerName              = "cros-test-finder"
	UseFlagFilterContainerName           = "use_flag_filter"
	PreProcessFilterContainerName        = "pre_process_filter"
	AutoVMTestShifterFilterContainerName = "autovm_test_shifter_filter"
	PartnerStagingContainerName          = "partner-staging"

	hwPlaceHolder = "PLACEHOLDER"
	// DefaultKarbonFilterNames defines Default karbon filters (SetDefaultFilters may add/remove)
	DefaultKarbonFilterNames = []string{TestFinderContainerName, ProvisionContainerName, hwPlaceHolder}

	// DefaultKoffeeFilterNames defines Default koffee filters (SetDefaultFilters may add/remove)
	// Deprecated: Falls under KarbonFilters.
	DefaultKoffeeFilterNames = []string{}

	binaryLookup = map[string]string{
		TestFinderContainerName:              "test_finder_filter",
		PreProcessFilterContainerName:        "pre_process_filter",
		AutoVMTestShifterFilterContainerName: "autovm_test_shifter_filter",
	}

	binaryArgsLookup = map[string][]string{
		TtcpContainerName: {"-creds", "/creds/service_accounts/service-account-chromeos.json"},
	}
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
func MakeDefaultFilters(ctx context.Context, suiteReq *api.SuiteRequest, experiments []string, isPartner, isAlRun bool) []string {
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
	if isExperimentEnabled("chromeos.cros_infra_config.autovm_test_shifter", experiments) && isSuiteSchedulerConfig(suiteReq) && !isAlRun {
		filters = append(filters, AutoVMTestShifterFilterContainerName)
	}
	if !isAlRun {
		filters = append(filters, UseFlagFilterContainerName, PreProcessFilterContainerName)
	}

	return filters
}

// GetDefaultFilters constructs ctp filters for provided default filters.
func GetDefaultFilters(ctx context.Context, defaultFilterNames []string, contMetadataMap map[string]*buildapi.ContainerImageInfo, build int) ([]*api.CTPFilter, error) {
	defaultFilters := make([]*api.CTPFilter, 0)
	logging.Infof(ctx, "Inside Default Filters: %s", defaultFilterNames)
	for _, filterName := range defaultFilterNames {
		var ctpFilter *api.CTPFilter
		var err error

		logging.Infof(ctx, "Checking container metadata map for %s", filterName)
		// Attempt to map the filter from the known container metadata.
		ctpFilter, err = CreateCTPFilterWithContainerName(ctx, filterName, contMetadataMap, build, false)
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
		return nil, fmt.Errorf("incompatible metadata build for name: %s, build: %d", name, build)
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
		ctpFilter, err := CreateCTPFilterWithContainerName(ctx, filterName, contMetadataMap, build, false)
		if err != nil {
			logging.Infof(ctx, "failed to create ctp filter for %s", filterName)
			return filters, errors.Annotate(err, "failed to create ctp filter for %s, %s", filterName, err).Err()
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
func CreateContainerRequest(requestedFilter *api.CTPFilter) *api.ContainerRequest {
	defaultBinaryArgs := binaryArgsLookup[requestedFilter.GetContainerInfo().GetContainer().GetName()]
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
					BinaryArgs: append([]string{
						"server", "-port", "0",
					}, append(defaultBinaryArgs, requestedFilter.GetContainerInfo().GetBinaryArgs()...)...),
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

func needBackwardsCompatibility(build int) bool {
	// TODO (dbeckett/azrahamn): set this to the proper build # once the compatibility
	// changes land in the OS src tree and have assigned build #s.
	return build < 20000
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
