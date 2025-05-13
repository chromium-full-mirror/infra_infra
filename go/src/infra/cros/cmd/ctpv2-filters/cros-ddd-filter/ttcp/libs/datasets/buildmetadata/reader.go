// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package buildmetada

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"

	ttcpSyntax "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/protos/ttcp/syntax"
	targetproperties "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/targetproperties"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/errors"
)

type PortageBuildTarget struct {
	OverlayName string
}

type BuildTarget struct {
	PortageBuildTarget PortageBuildTarget
}

func (BuildTarget *BuildTarget) Id() string {
	return BuildTarget.PortageBuildTarget.OverlayName
}

func (BuildTarget *BuildTarget) GetBoard() string {
	overlayName := BuildTarget.PortageBuildTarget.OverlayName
	board, _, _ := strings.Cut(overlayName, "-")
	return board
}

func (BuildTarget *BuildTarget) GetVariant() string {
	overlayName := BuildTarget.PortageBuildTarget.OverlayName
	_, variant, _ := strings.Cut(overlayName, "-")
	return variant
}

type PackageSummary struct {
	Info any
}

func (p *PackageSummary) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &p.Info)
}

type BuildMetadataEntry struct {
	BuildTarget     BuildTarget
	Package_Summary PackageSummary
}

func (buildMd *BuildMetadataEntry) Id() string {
	return buildMd.BuildTarget.Id()
}

func (buildMd *BuildMetadataEntry) GetBoard() string {
	return buildMd.BuildTarget.GetBoard()
}

func (buildMd *BuildMetadataEntry) GetVariant() string {
	return buildMd.BuildTarget.GetVariant()
}

type BuildMetadataSet struct {
	Values []BuildMetadataEntry
}

type BuildMetadataResources struct {
	// path to the location of file that lives in the repo
	// at /src/config-internal/build/generated/build_metadata.jsonproto
	Path string
}

func PaseAsMapPerBoard(resc BuildMetadataResources) (map[string][]BuildMetadataEntry, error) {
	metaDataSet, err := ParseAsList(resc)
	if err != nil {
		return map[string][]BuildMetadataEntry{}, err
	}
	results := map[string][]BuildMetadataEntry{}
	for _, entry := range metaDataSet.Values {
		board := strings.ToUpper(entry.GetBoard())
		buildImages, ok := results[board]
		if !ok {
			buildImages = []BuildMetadataEntry{entry}
		}
		buildImages = append(buildImages, entry)
		results[board] = buildImages
	}
	return results, nil
}

func ParseAsList(resc BuildMetadataResources) (BuildMetadataSet, error) {
	dataStream, err := os.Open(resc.Path)
	if err != nil {
		return BuildMetadataSet{}, errors.JoinError("Error in open the build metadata file "+resc.Path, errors.ApiError(err))
	}
	defer dataStream.Close()

	byteValue, err := ioutil.ReadAll(dataStream)
	if err != nil {
		return BuildMetadataSet{}, errors.JoinError("Error in reading the build metadata file.", errors.ApiError(err))
	}
	var dataSet BuildMetadataSet
	json.Unmarshal(byteValue, &dataSet)

	return dataSet, nil
}

func (metadata *BuildMetadataEntry) GetProperties() (targetproperties.TargetPropertiesValues, error) {
	targetProperties := targetproperties.CreateTargetPropertiesValues()
	propertiesAndValues := targetproperties.PropertiesAndValuesStore{}
	propertiesAndValues.AddPropertyValue("image:variant", metadata.GetVariant())
	err := propertiesAndValues.ExtractPropertiesAndValue(metadata.Package_Summary.Info, "image:")
	if err != nil {
		return targetProperties, err
	}
	for name, values := range propertiesAndValues {
		var buffer bytes.Buffer
		for val := range values {
			buffer.WriteString(val)
		}
		targetProperties.AddPropertyValue(name, "buildMetadata", buffer.String())
	}
	return targetProperties, nil
}

func (db *BuildMetadataSet) ExportCategories(categories *ttcpSyntax.Collection) error {
	propertiesAndValues := targetproperties.PropertiesAndValuesStore{}
	for _, metadata := range db.Values {
		propertiesAndValues.AddPropertyValue("image:variant", metadata.GetVariant())
		err := propertiesAndValues.ExtractPropertiesAndValue(metadata.Package_Summary.Info, "image:")
		if err != nil {
			return err
		}
	}
	propertiesAndValues.ExportCategories(categories)
	return nil
}
