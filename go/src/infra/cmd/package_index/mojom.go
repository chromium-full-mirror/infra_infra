// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"go.chromium.org/luci/common/data/stringset"
	"go.chromium.org/luci/common/logging"

	kpb "go.chromium.org/infra/cmd/package_index/kythe/proto"
)

const mojomScript = "/mojom_bindings_generator.py"

// mojomImportRe is used for finding required_input for a Mojom target,
// by finding the imports in its source.
// This could possibly have false positives (e.g. if there's an import in a
// C-style multiline comment), but it shouldn't have any false negatives, which
// is more important here.
var mojomImportRe = regexp.MustCompile(`(?m)^\s*import\s*"([^"]*)"`)

// mojomTarget contains all information needed to process a mojom target.
type mojomTarget struct {
	imports     []string
	args        []string
	rootDir     string
	outDir      string
	corpus      string
	buildConfig string
	targetName  string
	target      gnTargetInfo
	hashMap     *FileHashMap
	ctx         context.Context
}

// newMojomTarget initializes a new mojomTarget struct.
func newMojomTarget(ctx context.Context, gnTargetDict map[string]gnTargetInfo, targetName string,
	hashMap *FileHashMap, rootDir, outDir, corpus, buildConfig string) (*mojomTarget, error) {
	m := &mojomTarget{
		ctx:         ctx,
		targetName:  targetName,
		rootDir:     rootDir,
		outDir:      outDir,
		corpus:      corpus,
		buildConfig: buildConfig,
		hashMap:     hashMap,
	}
	m.target = gnTargetDict[m.targetName]
	m.args = m.mergeFeatureArgs(gnTargetDict)
	imp, err := m.findMojomImports()
	if err != nil {
		return nil, err
	}
	m.imports = imp
	return m, nil
}

// mergeFeatureArgs returns a combined list of args from the Mojom target targetName,
// stored in gnTargetsMap, and args from a parser target based on targetName.
//
// The Mojom toolchain works in two phases, first parsing the file with one tool
// which dumps the AST, then feeding the AST into the bindings generator. The
// Kythe indexer, however, works in one phase, and hence needs some arguments
// from each of these tools. In particular, definitions gated on disabled
// features are removed from the AST directly by the parser tool.
func (m *mojomTarget) mergeFeatureArgs(gnTargetDict map[string]gnTargetInfo) []string {
	args := gnTargetDict[m.targetName].Args

	if !strings.HasSuffix(gnTargetDict[m.targetName].Script, mojomScript) {
		// mojom script uses some kind of wrapper, unwrap it
		for i, arg := range args {
			if strings.HasSuffix(arg, mojomScript) {
				args = args[i+1:]
				break
			}
		}
	}
	parserTarget := m.targetName[:len(m.targetName)-len("__generator")] + "__parser"
	parserTargetDict := gnTargetDict[parserTarget]
	parserTargetArgs := parserTargetDict.Args
	for i := range len(parserTargetArgs) - 1 {
		if parserTargetArgs[i] == "--enable_feature" {
			args = append(args, parserTargetArgs[i:i+2]...)
		}
	}
	return args
}

// findMojomImports finds the direct imports of a Mojom target.
//
// We do this by using a quick and dirty regex to extract files that are
// actually imported, rather than using the gn dependency structure. A Mojom
// file is allowed to import any file it transitively depends on, which usually
// includes way more files than it actually includes.
func (m *mojomTarget) findMojomImports() ([]string, error) {
	var importPaths []string

	// Mojom imports may contain generated imports.
	imp, err := filepath.Abs(filepath.Join(m.rootDir, m.outDir, "gen"))
	if err != nil {
		return nil, err
	}
	importPaths = append(importPaths, imp)

	args := m.target.Args
	for i := range len(args) - 1 {
		if args[i] == "-I" {
			imp, err := filepath.Abs(filepath.Join(m.rootDir, m.outDir, args[i+1]))
			if err != nil {
				return nil, err
			}
			importPaths = append(importPaths, imp)
		}
	}

	imports := stringset.New(0)
	for _, src := range m.target.Sources {
		gn, err := convertGnPath(m.ctx, src, m.outDir)
		if err != nil {
			return nil, err
		}
		p := filepath.Join(m.rootDir, m.outDir, gn)
		imports.AddAll(findImports(m.ctx, mojomImportRe, p, importPaths))
	}

	return imports.ToSlice(), nil
}

// isMojomFile checks if extenion matches mojom file extension.
func isMojomFile(filename string) bool {
	return strings.HasSuffix(filename, ".mojom")
}

// getUnit returns a compilation unit for a mojom target.
func (m *mojomTarget) getUnit() (*kpb.CompilationUnit, error) {
	unitProto := &kpb.CompilationUnit{}
	var sourceFiles []string

	for _, src := range m.target.Sources {
		gn, err := convertGnPath(m.ctx, src, m.outDir)
		if err != nil {
			return nil, err
		}
		if !isMojomFile(gn) {
			continue
		}
		sourceFiles = append(sourceFiles, convertPathToForwardSlashes(gn))
	}
	unitProto.SourceFile = sourceFiles

	// gn produces an unsubstituted {{response_file_name}} for filelist. We
	// can't work with this, so we remove it and add the source files as a
	// positional argument instead.
	for _, arg := range m.args {
		if !strings.HasPrefix(arg, "--filelist=") {
			unitProto.Argument = append(unitProto.Argument, arg)
		}
	}
	unitProto.Argument = append(unitProto.Argument, sourceFiles...)
	unitProto.VName = &kpb.VName{Corpus: m.corpus, Language: "mojom"}
	if m.buildConfig != "" {
		injectUnitBuildDetails(m.ctx, unitProto, m.buildConfig)
	}

	// Files in a module might import other files in the same module. Don't
	// include the file twice if so.
	var importedFiles []string
	srcSet := stringset.NewFromSlice(sourceFiles...)
	for _, imp := range m.imports {
		p, err := filepath.Rel(filepath.Join(m.rootDir, m.outDir), convertPathToForwardSlashes(imp))
		if err != nil {
			return nil, err
		}
		if !srcSet.Has(p) {
			importedFiles = append(importedFiles, p)
		}
	}

	for _, requiredFile := range append(sourceFiles, importedFiles...) {
		p, err := filepath.Abs(filepath.Join(m.rootDir, m.outDir, requiredFile))
		if err != nil {
			return nil, err
		}
		// We don't want to fail completely if the file doesn't exist.
		h, ok := m.hashMap.Filehash(p)
		if !ok {
			logging.Warningf(m.ctx, "Missing from filehashes %s\n", p)
			continue
		}

		vname := &kpb.VName{}
		setVnameForFile(vname, convertPathToForwardSlashes(
			normalizePath(m.outDir, requiredFile)), m.corpus)
		requiredInput := &kpb.CompilationUnit_FileInput{
			VName: vname,
			Info: &kpb.FileInfo{
				Digest: h,
				Path:   convertPathToForwardSlashes(requiredFile),
			},
		}
		unitProto.RequiredInput = append(unitProto.GetRequiredInput(), requiredInput)
	}
	return unitProto, nil
}

// getFiles retrieves a list of all files that are required for compilation of a target.
// Returns a list of all included files, in their absolute paths.
func (m *mojomTarget) getFiles() ([]string, error) {
	var dataFiles []string
	for _, src := range m.target.Sources {
		gn, err := convertGnPath(m.ctx, src, m.outDir)
		if err != nil {
			return nil, err
		}
		if !isMojomFile(gn) {
			// Only add .mojom files; we don't care about any other
			// one, e.g. zip that currently mojom targets have.
			continue
		}
		dataFiles = append(dataFiles, filepath.Join(m.rootDir, m.outDir, gn))
	}
	return dataFiles, nil
}

// mojomTargetProcessor takes in a target and either returns an error if the target
// isn't a mojom target or returns a processedTarget struct.
//
// If files is true, process the target files. Otherwise, process the target unit.
func mojomTargetProcessor(ctx context.Context, rootPath, outDir, corpus, buildConfig string,
	hashMaps *FileHashMap, t *gnTarget) (GnTargetInterface, error) {
	if !isMojomTarget(t) {
		return nil, errNotSupported
	}

	return newMojomTarget(ctx, gnTargetsMap, t.targetName, hashMaps, rootPath, outDir, corpus, buildConfig)
}

// IsMojomTarget checks if a GN target is a Mojom target.
//
// Note that there are multiple GN targets for each Mojom build rule, due to
// how the mojom.gni rules are defined. We pick a single canonical one out of
// this set, namely the __generator target, which generates the standard C++
// bindings.
func isMojomTarget(t *gnTarget) bool {
	if !strings.HasSuffix(t.targetName, "__generator") {
		return false
	}

	isMojom := strings.HasSuffix(t.targetInfo.Script, mojomScript)

	// Determine if wrapper script is used. If it is, extract actual script
	// which is located at the very first argument.
	if !isMojom && strings.HasSuffix(t.targetInfo.Script, "/action_remote.py") && len(t.targetInfo.Args) > 0 {
		// handle reclient
		for _, arg := range t.targetInfo.Args[1:] {
			if strings.HasSuffix(arg, mojomScript) {
				isMojom = true
				break
			}
		}
	}

	if !isMojom {
		return false
	}

	args := t.targetInfo.Args
	argsSet := stringset.NewFromSlice(args...)
	if !argsSet.Has("generate") || argsSet.Has("--variant") ||
		argsSet.Has("--generate_non_variant_code") || argsSet.Has("--generate_message_ids") {
		return false
	}

	// For now we don't support xrefs for languages other than C++, so
	// the mojom analyzer only bothers with the C++ output.
	argsCpp := false
	for i := range len(args) - 1 {
		if args[i] == "-g" && args[i+1] == "c++" {
			argsCpp = true
			break
		}
	}

	if !argsCpp {
		return false
	}

	for _, src := range t.targetInfo.Sources {
		if isMojomFile(src) {
			return true
		}
	}
	return false
}
