// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package common is the common package.
package common

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"cloud.google.com/go/storage"

	"go.chromium.org/chromiumos/config/go/test/api"

	statuserrors "go.chromium.org/infra/cros/cmd/cft/common/errors"
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

// ObjectExists checks if an object exists at the specified path.
func ObjectExists(ctx context.Context, bucket *storage.BucketHandle, object string) (bool, error) {
	if _, err := bucket.Object(object).Attrs(ctx); errors.Is(err, storage.ErrObjectNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// PullFilesFromGcsDirLocal will fetch all artifacts listed an pull them to a local directory.
func PullFilesFromGcsDirLocal(ctx context.Context, logger *log.Logger, bucket *storage.BucketHandle, inDir, outDir string, files []string) error {
	for _, artifact := range files {
		gcsPath := filepath.Join(inDir, artifact)
		logger.Printf("Fetching artifact %s from GCS", gcsPath)

		// Get the file from GCS
		rc, err := bucket.Object(gcsPath).NewReader(ctx)
		if err != nil {
			return fmt.Errorf("error getting reader for %s: %w", gcsPath, err)
		}
		defer rc.Close()

		// Create a local file for the artifact
		localPath := filepath.Join(outDir, artifact)
		file, err := os.Create(localPath)
		if err != nil {
			return statuserrors.NewStatusError(statuserrors.IOCreateError, fmt.Errorf("failed to create local file %v: %w", localPath, err))
		}
		defer file.Close()

		// Copy the GCS file to the local file
		if _, err := io.Copy(file, rc); err != nil {
			return fmt.Errorf("failed to copy GCS file %s to local file %s: %w", gcsPath, localPath, err)
		}
		logger.Printf("Successfully fetched and saved file to %s", localPath)
	}
	return nil
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

// AndroidBuildIDFromTestplan attempts to find a build_id in the TestPlan (req).
func AndroidBuildIDFromTestplan(req *api.InternalTestplan) (string, error) {
	args := req.GetSuiteInfo().GetSuiteMetadata().GetExecutionMetadata().GetArgs()
	for _, arg := range args {
		if arg.GetFlag() == "build_id" {
			return arg.GetValue(), nil
		}
	}
	return "", fmt.Errorf("no build_id present in suite metadata")
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

// UnzipFile unzips a file and returns a list of files found.
func UnzipFile(srcFile, dstPath string) ([]string, error) {
	r, e := zip.OpenReader(srcFile)
	if e != nil {
		return nil, e
	}
	defer r.Close()

	var files []string
	for _, f := range r.File {
		file := filepath.Join(dstPath, f.Name)
		files = append(files, file)
		if f.FileInfo().IsDir() {
			os.MkdirAll(file, os.ModePerm)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(file), os.ModePerm); err != nil {
			return nil, err
		}
		var err error
		var dstFile *os.File
		if dstFile, err = os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode()); err == nil {
			var fileInArchive io.ReadCloser
			if fileInArchive, err = f.Open(); err == nil {
				_, err = io.Copy(dstFile, fileInArchive)
				fileInArchive.Close()
			}
			dstFile.Close()
		}
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}
