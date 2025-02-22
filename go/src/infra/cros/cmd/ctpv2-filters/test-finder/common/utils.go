// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package common is the common package.
package common

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"cloud.google.com/go/storage"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/libs/skylab/inventory/autotest/labels"
	s "go.chromium.org/infra/libs/skylab/inventory/swarming"
)

func FindNewestDirInGcsBucket(ctx context.Context, gcsBasePath string, bucket *storage.BucketHandle, pf string) (string, error) {
	// List subdirectories under the prefix directory.
	it := bucket.Objects(ctx, &storage.Query{Prefix: pf, Delimiter: "/"})

	var subdirs []string
	for {
		attrs, err := it.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			break
		}
		if attrs.Prefix != "" && strings.HasPrefix(path.Base(attrs.Prefix), "cros") {
			subdirs = append(subdirs, attrs.Prefix)
		}
	}

	if len(subdirs) == 0 {
		return "", fmt.Errorf("no subdirectories found under %s", gcsBasePath)
	}

	// Sort subdirectories to find the newest one (assuming lexicographical order corresponds to time).
	sort.Strings(subdirs)
	newestDir := subdirs[len(subdirs)-1]
	return newestDir, nil
}

// PullAllFilesFromGcsDir will grab all the files from a dir matching the postfix
func PullAllFilesFromGcsDir(ctx context.Context, bucket *storage.BucketHandle, dir string, postfix string) ([][]byte, error) {
	var it *storage.ObjectIterator
	if dir != "" {
		it = bucket.Objects(ctx, &storage.Query{Prefix: dir})

	} else {
		it = bucket.Objects(ctx, &storage.Query{})
	}
	var data [][]byte // Accumulate data from all JSON files
	for {
		attrs, err := it.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			break
		}
		if strings.HasSuffix(attrs.Name, postfix) {
			r, err := bucket.Object(attrs.Name).NewReader(ctx)
			if err != nil {
				return nil, fmt.Errorf("creating reader for %s: %w", attrs.Name, err)
			}
			defer r.Close()

			jsonData, err := io.ReadAll(r)
			if err != nil {
				return nil, fmt.Errorf("reading data from %s: %w", attrs.Name, err)
			}

			data = append(data, jsonData) // Append data from current file
		}
	}

	if data == nil {
		return nil, fmt.Errorf("no files found in newest directory %s", dir)
	}
	return data, nil
}

func ExtractBucketAndPrefixFromPath(gcsPath string) (bucketName, prefix string, err error) {
	parts := strings.SplitN(gcsPath, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid GCS path: %s", gcsPath)
	}
	bucketName = parts[0]
	prefix = fmt.Sprintf("%s/", parts[1])
	return
}

// TranslateTCMtoCTPTC will translate []*api.TestCaseMetadata to []*api.CTPTestCase
func TranslateTCMtoCTPTC(matchingTests []*api.TestCaseMetadata) (CTPTCs []*api.CTPTestCase) {
	for _, metadata := range matchingTests {
		CTPTCs = append(CTPTCs, tfToCTPTestCase(metadata))
	}
	return CTPTCs
}

func tfToCTPTestCase(metadata *api.TestCaseMetadata) *api.CTPTestCase {
	tc := &api.CTPTestCase{
		Name:     metadata.GetTestCase().GetId().GetValue(),
		Metadata: metadata,
	}

	deps := Converter(tc.GetMetadata().GetTestCase().GetDependencies())
	if len(deps) != 0 {
		tc.Metadata.TestCase.Dependencies = deps
	}
	return tc
}

// Converter will convert and revert the TC dependency in case its in the legacy style.
func Converter(deps []*api.TestCase_Dependency) []*api.TestCase_Dependency {
	convertedDeps := []string{}
	for _, dep := range deps {
		f := dep.GetValue()
		converted := convertDep(f)
		// If the dep can't be converted, let it flow through naturally. Bot params should handle the case where its invalid
		if len(converted) == 0 {
			convertedDeps = append(convertedDeps, f)
		} else {
			convertedDeps = append(convertedDeps, converted...)
		}
	}
	finalDeps := []*api.TestCase_Dependency{}
	for _, dep := range convertedDeps {
		tcD := &api.TestCase_Dependency{
			Value: dep,
		}
		finalDeps = append(finalDeps, tcD)
	}
	return finalDeps
}

func convertDep(dep string) []string {
	deps := []string{dep}
	parsedDeps := labels.Revert(deps)

	depsf := []string{}
	for k, v := range s.Convert(parsedDeps) {
		for _, innerv := range v {
			depsf = append(depsf, fmt.Sprintf("%s:%s", k, innerv))

		}
	}
	return depsf
}

// TestSuiteFromTestplan will create the TestSuite from the TestPlan (req).
func TestSuiteFromTestplan(req *api.InternalTestplan) ([]*api.TestSuite, error) {
	// Not going to implement C-Suite implementation; but if it does become needed; there will be several changes needed in this file.
	requestedSuite, ok := req.GetSuiteInfo().GetSuiteRequest().GetSuiteRequest().(*api.SuiteRequest_TestSuite)
	if !ok {
		return nil, fmt.Errorf("SuiteRequest is not TestSuite")
	}
	testSuite := requestedSuite.TestSuite

	TestSuites := []*api.TestSuite{testSuite}
	return TestSuites, nil

}

// append the magical `FlexibleTF` key on the suiteArgs to be processed by cros-test.
func AddFlexibleTFFlag(tp *api.InternalTestplan) {
	existingMD := tp.GetSuiteInfo().GetSuiteMetadata().GetExecutionMetadata()
	if existingMD == nil || len(existingMD.Args) == 0 {
		existingMD = &api.ExecutionMetadata{Args: []*api.Arg{}}
	}
	existingMD.Args = append(existingMD.Args, &api.Arg{
		Flag:  "FlexibleTF",
		Value: "true",
	})
	if tp.GetSuiteInfo().GetSuiteMetadata() != nil {
		tp.SuiteInfo.SuiteMetadata.ExecutionMetadata = existingMD
	} else {

		tp.SuiteInfo.SuiteMetadata = &api.SuiteMetadata{ExecutionMetadata: existingMD}
	}
}
