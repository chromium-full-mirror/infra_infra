// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package db

import (
	"fmt"
	"path/filepath"
	"strings"

	ttcpSyntax "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/protos/ttcp/syntax"
)

const HwidSource = "HWID database"

type HwidDb struct {
	DescriptorPaths   []string
	ProjectsIndexPath string
	ProjectsIndex     map[string]string
	DescriptorProject map[string]*HwidDescriptor
}

type HwidDbResources struct {
	DescriptorsPaths []string
	ProjectIndexPath string
}

func InitializeHwidDb(resc *HwidDbResources) *HwidDb {
	result := &HwidDb{
		DescriptorPaths:   resc.DescriptorsPaths,
		ProjectsIndexPath: resc.ProjectIndexPath,
		ProjectsIndex:     loadProjectToBoardIndex(resc.ProjectIndexPath),
		DescriptorProject: LoadDescriptors(resc.DescriptorsPaths),
	}
	return result
}

func (db *HwidDb) GetBoardForProject(project string) string {
	return db.ProjectsIndex[project]
}

type descriptorType struct {
	model    string
	internal bool
	path     string
}

const internalLabel = ".internal"

// returns the model name and if the descriptor is an internal version
func parseDescriptorPath(path string) *descriptorType {
	internal := strings.HasSuffix(path, internalLabel)
	normalizedPath := path
	if internal {
		normalizedPath = path[0 : len(path)-len(internalLabel)]
	}
	_, model := filepath.Split(normalizedPath)
	return &descriptorType{
		model:    model,
		internal: internal,
		path:     path,
	}
}

func LoadDescriptors(paths []string) map[string]*HwidDescriptor {
	descriptors := map[string]*HwidDescriptor{}

	todo := map[string]*descriptorType{}
	for _, path := range paths {
		t := parseDescriptorPath(path)
		current, ok := todo[t.model]
		if !ok {
			todo[t.model] = t
		} else {
			if !current.internal && t.internal {
				todo[t.model] = t
			}
		}
	}
	for model, t := range todo {
		descriptor := LoadDescriptor(t.path)
		descriptors[model] = descriptor
	}
	return descriptors
}

func (db *HwidDb) GetDescriptor(modelBrand string) (*HwidDescriptor, error) {
	descriptor, ok := db.DescriptorProject[modelBrand]
	if !ok {
		return &HwidDescriptor{}, fmt.Errorf("model %s has no descriptor", modelBrand)
	}
	return descriptor, nil
}

func (db *HwidDb) ExportCategories(categories *ttcpSyntax.Collection) {
	properties := CreateHwidProperties()
	for _, descriptorPath := range db.DescriptorPaths {
		descriptor := LoadDescriptor(descriptorPath)
		descriptor.collectDescriptorPropertyValues(&properties)
	}

	for _, board := range db.ProjectsIndex {
		properties.addPropertyValue("board", HwidSource, board)
	}

	properties.exportCategories(categories)
}
