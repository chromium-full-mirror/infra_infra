// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlmmetadata

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	ttcpSyntax "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/protos/ttcp/syntax"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/targetproperties"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/errors"
)

type DlmMetadataEntry struct {
	DeviceId     int    `json:"device_id"`
	Soc          string `json:"soc"`
	FormFactor   string `json:"form_factor"`
	SocVendor    string `json:"soc_vendor"`
	Architecture string `json:"architecture"`
	DeviceType   string `json:"device_type"`
	BuildTargets string `json:"build_targets"`
	Model        string `json:"model"`
}

func (dlmMd *DlmMetadataEntry) GetDeviceId() int {
	return dlmMd.DeviceId
}

func (dlmMd *DlmMetadataEntry) GetSoc() string {
	return dlmMd.Soc
}

func (dlmMd *DlmMetadataEntry) GetFormFactor() string {
	return dlmMd.FormFactor
}

func (dlmMd *DlmMetadataEntry) GetSocVendor() string {
	return dlmMd.SocVendor
}

func (dlmMd *DlmMetadataEntry) GetArchitecture() string {
	return dlmMd.Architecture
}

func (dlmMd *DlmMetadataEntry) GetDeviceType() string {
	return dlmMd.DeviceType
}

func (dlmMd *DlmMetadataEntry) GetBuildTargets() string {
	return dlmMd.BuildTargets
}

func (dlmMd *DlmMetadataEntry) GetModel() string {
	return dlmMd.Model
}

type DlmMetadataSet struct {
	Values []*DlmMetadataEntry `json:"values"`
}

type DlmResources struct {
	// path to the location of file that lives in the repo
	// at /scripts/generated/dlm_devices.json
	Path string
}

func PaseAsMapPerModel(resc *DlmResources) (map[string][]*DlmMetadataEntry, error) {
	metaDataSet, err := ParseAsList(resc)
	if err != nil {
		return map[string][]*DlmMetadataEntry{}, err
	}
	results := map[string][]*DlmMetadataEntry{}
	for _, entry := range metaDataSet.Values {
		model := strings.ToUpper(entry.Model)
		devices, ok := results[model]
		if !ok {
			devices = []*DlmMetadataEntry{entry}
		}
		devices = append(devices, entry)
		results[model] = devices
	}
	return results, nil
}

func ParseAsList(resc *DlmResources) (*DlmMetadataSet, error) {
	dataStream, err := os.Open(resc.Path)

	if err != nil {
		return &DlmMetadataSet{}, errors.JoinError("Error in open the dlm metadata file "+resc.Path, errors.ApiError(err))
	}
	defer dataStream.Close()

	byteValue, err := ioutil.ReadAll(dataStream)
	if err != nil {
		return &DlmMetadataSet{}, errors.JoinError("Error in reading the build metadata file.", errors.ApiError(err))
	}
	var dataSet *DlmMetadataSet
	json.Unmarshal(byteValue, &dataSet)
	return dataSet, nil
}

func (metadata *DlmMetadataEntry) GetProperties() (*targetproperties.TargetPropertiesValues, error) {
	targetProperties := targetproperties.CreateTargetPropertiesValues()
	propertiesAndValues := targetproperties.PropertiesAndValuesStore{}
	propertiesAndValues.AddPropertyValue("dlm:device_id", strconv.Itoa(metadata.GetDeviceId()))
	propertiesAndValues.AddPropertyValue("dlm:soc", metadata.GetSoc())
	propertiesAndValues.AddPropertyValue("dlm:form_factor", metadata.GetFormFactor())
	propertiesAndValues.AddPropertyValue("dlm:soc_vendor", metadata.GetSocVendor())
	propertiesAndValues.AddPropertyValue("dlm:device_type", metadata.GetDeviceType())
	propertiesAndValues.AddPropertyValue("dlm:architecture", metadata.GetArchitecture())
	propertiesAndValues.AddPropertyValue("dlm:model", metadata.GetModel())
	propertiesAndValues.AddPropertyValue("dlm:build_targets", metadata.GetBuildTargets())
	for name, values := range propertiesAndValues {
		for val := range values {
			targetProperties.AddPropertyValue(name, "dlm_devices", val)
		}
	}
	return targetProperties, nil
}

func (db *DlmMetadataSet) ExportCategories(categories *ttcpSyntax.Collection) error {
	propertiesAndValues := targetproperties.PropertiesAndValuesStore{}
	for _, metadata := range db.Values {
		propertiesAndValues.AddPropertyValue("dlm:device_id", strconv.Itoa(metadata.GetDeviceId()))
		propertiesAndValues.AddPropertyValue("dlm:soc", metadata.GetSoc())
		propertiesAndValues.AddPropertyValue("dlm:form_factor", metadata.GetFormFactor())
		propertiesAndValues.AddPropertyValue("dlm:soc_vendor", metadata.GetSocVendor())
		propertiesAndValues.AddPropertyValue("dlm:device_type", metadata.GetDeviceType())
		propertiesAndValues.AddPropertyValue("dlm:architecture", metadata.GetArchitecture())
		propertiesAndValues.AddPropertyValue("dlm:model", metadata.GetModel())
		propertiesAndValues.AddPropertyValue("dlm:build_targets", metadata.GetBuildTargets())
	}
	propertiesAndValues.ExportCategories(categories)
	return nil
}
