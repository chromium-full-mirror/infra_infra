// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package swarmingdata

import (
	"log"
	"strings"

	ttcpSyntax "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/protos/ttcp/syntax"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/targetproperties"
)

type LabelObj struct {
	Label string `bigquery:"label"`
	Val   string `bigquery:"val"`
}
type SwarmingBot struct {
	Values []*LabelObj `bigquery:"labels"`
	DutId  string      `bigquery:"dut_id"`
}

type SwarmingdataEntry struct {
	Labels map[string][]string
	DutId  string
}

type SwarmingdataSet struct {
	Values []*SwarmingdataEntry
}

type SwarmDataResources struct {
	SwarmDb []*SwarmingBot
}

func (resc *SwarmDataResources) ParseAsList() (*SwarmingdataSet, error) {
	dataSet := &SwarmingdataSet{Values: []*SwarmingdataEntry{}}
	for _, swarmingBot := range resc.SwarmDb {
		entry := &SwarmingdataEntry{DutId: swarmingBot.DutId, Labels: map[string][]string{}}
		for _, label := range swarmingBot.Values {
			labelVals, ok := entry.Labels[label.Label]
			if !ok {
				labelVals = []string{}
			}
			entry.Labels[label.Label] = append(labelVals, label.Val)
		}
		dataSet.Values = append(dataSet.Values, entry)
	}
	return dataSet, nil
}

func (resc *SwarmDataResources) ParseAsMapPerHwid() (map[string][]*SwarmingdataEntry, error) {
	metaDataSet, err := resc.ParseAsList()
	if err != nil {
		return map[string][]*SwarmingdataEntry{}, err
	}
	results := map[string][]*SwarmingdataEntry{}
	for _, entry := range metaDataSet.Values {
		labels := entry.Labels
		hwid := strings.ToUpper(labels["hwid"][0])
		devices, ok := results[hwid]
		if !ok {
			devices = []*SwarmingdataEntry{}
		}
		devices = append(devices, entry)
		results[hwid] = devices
	}
	return results, nil
}

func (db *SwarmingdataSet) ExportCategories(categories *ttcpSyntax.Collection) error {
	propertiesAndValues := targetproperties.PropertiesAndValuesStore{}
	for _, metadata := range db.Values {
		err := propertiesAndValues.ExtractPropertiesAndValue(metadata.Labels, "swarming:")
		if err != nil {
			return err
		}
	}
	propertiesAndValues.ExportCategories(categories)
	return nil
}

func (metadata *SwarmingdataEntry) GetProperties() (*targetproperties.TargetPropertiesValues, error) {
	targetProperties := targetproperties.CreateTargetPropertiesValues()

	propertiesAndValues := targetproperties.PropertiesAndValuesStore{}
	err := propertiesAndValues.ExtractPropertiesAndValue(metadata.Labels, "swarming:")
	if err != nil {
		return targetProperties, err
	}
	for name, values := range propertiesAndValues {
		for val := range values {
			targetProperties.AddPropertyValue(name, "swarm_labels", val)
		}
	}
	return targetProperties, nil
}

func (db *SwarmingdataSet) GetUniqueHwids() []string {
	unique := map[string]bool{}
	uniqueHwids := []string{}
	for _, device := range db.Values {
		hwid, ok := device.Labels["hwid"]
		if !ok {
			log.Println("Hwid key missing from device:", device)
		} else {
			hwidVal := hwid[0]
			if _, ok := unique[hwidVal]; !ok {
				uniqueHwids = append(uniqueHwids, hwidVal)
				unique[hwidVal] = true
			}
		}
	}
	return uniqueHwids
}

func (metadata *SwarmingdataEntry) GetFlatPropValMap() map[string]bool {
	flatPropValMap := map[string]bool{}

	for label, vals := range metadata.Labels {
		for _, val := range vals {
			flatPropValMap[label+":"+val] = true
		}
	}
	return flatPropValMap
}
