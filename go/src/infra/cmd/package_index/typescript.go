// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"path/filepath"
	"strings"

	"go.chromium.org/luci/common/logging"

	kpb "go.chromium.org/infra/cmd/package_index/kythe/proto"
)

type tsTarget struct {
	args        []string
	rootDir     string
	outDir      string
	corpus      string
	buildConfig string
	targetName  string
	tsconfig    string
	target      gnTargetInfo
	hashMap     *FileHashMap
	ctx         context.Context
}

func getTSConfig(target gnTargetInfo, ctx context.Context, rootDir, outDir string) (string, error) {
	for i := 0; i < len(target.Args)-1; i++ {
		if target.Args[i] == "--tsconfig_output_location" {
			tsconfig := target.Args[i+1]
			if strings.HasPrefix(tsconfig, "//") {
				gn, err := convertGnPath(ctx, tsconfig, outDir)
				if err != nil {
					return "", err
				}
				return filepath.Join(rootDir, outDir, gn), nil
			} else {
				return filepath.Join(rootDir, outDir, tsconfig), nil
			}
		}
	}
	return "", errNotSupported
}

func newTSTarget(ctx context.Context,
	targetName string, hashMap *FileHashMap, rootDir, outDir, corpus,
	buildConfig string) (*tsTarget, error) {
	target := gnTargetsMap[targetName]
	tsconfig, err := getTSConfig(target, ctx, rootDir, outDir)
	if err != nil {
		return nil, err
	}
	m := &tsTarget{
		ctx:         ctx,
		targetName:  targetName,
		rootDir:     rootDir,
		outDir:      outDir,
		corpus:      corpus,
		buildConfig: buildConfig,
		hashMap:     hashMap,
		tsconfig:    tsconfig,
	}
	m.target = target
	m.args = gnTargetsMap[m.targetName].Args
	return m, nil
}

func (m *tsTarget) getImportedFiles() []string {
	dependencies := make(map[string][]string)
	var queue []string
	queue = append(queue, m.target.Deps...)

	for len(queue) > 0 {
		front := queue[0]
		if dependencies[front] == nil {
			target := gnTargetsMap[front]
			if isTSTargetInfo(target) {
				dependencies[front] = target.Sources
			}
			queue = append(queue, target.Deps...)
		}
		queue = queue[1:]
	}

	var importedFiles []string
	for t := range dependencies {
		importedFiles = append(importedFiles, dependencies[t]...)
	}
	return importedFiles
}

func (m *tsTarget) convertGnPaths(paths []string) ([]string, error) {
	var converted []string
	for _, src := range paths {
		gn, err := convertGnPath(m.ctx, src, m.outDir)
		if err != nil {
			return nil, err
		}
		p := filepath.Join(m.outDir, gn)
		converted = append(converted, convertPathToForwardSlashes(p))
	}

	return converted, nil
}

func (m *tsTarget) getUnit() (*kpb.CompilationUnit, error) {
	unitProto := &kpb.CompilationUnit{}
	var sourceFiles []string
	tsconfig, err := filepath.Rel(m.rootDir, m.tsconfig)
	if err != nil {
		return nil, err
	}
	sourceFiles = append(sourceFiles, convertPathToForwardSlashes(tsconfig))

	convertedSourceFiles, err := m.convertGnPaths(m.target.Sources)
	if err != nil {
		return nil, err
	}
	sourceFiles = append(sourceFiles, convertedSourceFiles...)

	importedFiles, err := m.convertGnPaths(m.getImportedFiles())
	if err != nil {
		return nil, err
	}
	sourceFiles = append(sourceFiles, importedFiles...)

	unitProto.Argument = append(unitProto.Argument, "@@"+convertPathToForwardSlashes(tsconfig))
	unitProto.SourceFile = sourceFiles
	unitProto.VName = &kpb.VName{Corpus: m.corpus, Language: "typescript"}

	for _, requiredFile := range sourceFiles {
		p, err := filepath.Abs(filepath.Join(m.rootDir, requiredFile))
		if err != nil {
			return nil, err
		}
		h, ok := m.hashMap.Filehash(p)
		if !ok {
			logging.Warningf(m.ctx, "Missing from filehashes %s\n", p)
			continue
		}

		vname := &kpb.VName{}
		setVnameForFile(vname, convertPathToForwardSlashes(requiredFile), m.corpus)
		requiredInput := &kpb.CompilationUnit_FileInput{
			VName: vname,
			Info: &kpb.FileInfo{
				Digest: h,
				Path:   convertPathToForwardSlashes(requiredFile),
			},
		}
		unitProto.RequiredInput = append(unitProto.GetRequiredInput(),
			requiredInput)
	}

	return unitProto, nil
}

func (m *tsTarget) getFiles() ([]string, error) {
	var dataFiles []string
	for _, src := range m.target.Sources {
		gn, err := convertGnPath(m.ctx, src, m.outDir)
		if err != nil {
			return nil, err
		}
		dataFiles = append(dataFiles, filepath.Join(m.rootDir, m.outDir, gn))
	}

	importedFiles, err := m.convertGnPaths(m.getImportedFiles())
	if err != nil {
		return nil, err
	}
	for _, importedFile := range importedFiles {
		p, err := filepath.Abs(filepath.Join(m.rootDir, importedFile))
		if err != nil {
			return nil, err
		}
		dataFiles = append(dataFiles, p)
	}

	dataFiles = append(dataFiles, m.tsconfig)
	return dataFiles, nil
}

func tsTargetProcessor(ctx context.Context, rootPath, outDir, corpus,
	buildConfig string, hashMaps *FileHashMap, t *gnTarget) (
	GnTargetInterface, error) {

	if !strings.HasSuffix(t.targetName, "indexer_tsconfig") || !isTSTarget(t) {
		return nil, errNotSupported
	}

	return newTSTarget(ctx, t.targetName, hashMaps,
		rootPath, outDir, corpus, buildConfig)
}

func isTSTargetInfo(t gnTargetInfo) bool {
	return t.Script == "//third_party/devtools-frontend/src/scripts/build/typescript/ts_library.py"
}

func isTSTarget(t *gnTarget) bool {
	return isTSTargetInfo(t.targetInfo)
}
