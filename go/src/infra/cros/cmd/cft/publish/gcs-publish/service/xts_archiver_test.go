// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build !windows
// +build !windows

package service

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/clock/testclock"
)

func TestArchiveInfos(t *testing.T) {
	Convey("Test archive info", t, func() {
		now := testclock.TestTimeUTC
		nowStr := now.Format(AL_PACKAGE_TIMESTAMP_FORMAT)
		tempDir := t.TempDir()
		testCases := []struct {
			name             string
			rootDir          string
			resultFileDir    string
			metadata         *api.XtsArchiverMetadata
			wantInstr        []ArchiveInfo
			wantErr          error
			createZipFile    bool
			createResultFile bool
			createTGZFile    bool
		}{
			{
				name:          "success with GTS",
				rootDir:       filepath.Join(tempDir, "cros-test", "result", "tauto"),
				resultFileDir: filepath.Join(tempDir, "cros-test", "result", "tauto", "results-1-cheets_GTS_12.internal.all.android.dpi", "cheets_GTS_12.android.dpi", "results", "cts-results", "2024.11.17_11.51.17.458_8891"),
				metadata: &api.XtsArchiverMetadata{
					ApfeGcsPrefix:        "gs://apfe-bucket",
					ResultsGcsPrefix:     "gs://result-bucket",
					Build:                "brya-release/R100-14543.0.0",
					Product:              "brya.brya",
					ParentSwarmingTaskId: "parent_job_id",
				},
				createZipFile:    true,
				createResultFile: true,
				createTGZFile:    true,
				wantInstr: []ArchiveInfo{
					{
						Name:        "apfe:brya.brya-release/R100-14543.0.0/parent_job_id/cheets_GTS_12.android.dpi/tauto_2024.11.17_11.51.17.458_8891",
						Source:      filepath.Join(tempDir, "cros-test", "result", "tauto", "results-1-cheets_GTS_12.internal.all.android.dpi", "cheets_GTS_12.android.dpi", "results", "cts-results", "2024.11.17_11.51.17.458_8891.zip"),
						Destination: "gs://apfe-bucket/brya.brya-release/R100-14543.0.0/parent_job_id/cheets_GTS_12.android.dpi/tauto_2024.11.17_11.51.17.458_8891/",
					},
					{
						Name:        "results:cheets_GTS_12.android.dpi/tauto_2024.11.17_11.51.17.458_8891",
						Source:      filepath.Join(tempDir, "cros-test", "result", "tauto", "results-1-cheets_GTS_12.internal.all.android.dpi", "cheets_GTS_12.android.dpi", "results", "cts-results", "2024.11.17_11.51.17.458_8891", "test_result.xml.gz"),
						Destination: "gs://result-bucket/cheets_GTS_12.android.dpi/tauto_2024.11.17_11.51.17.458_8891/",
					}, {
						Name:        "results:cheets_GTS_12.android.dpi/tauto_2024.11.17_11.51.17.458_8891",
						Source:      filepath.Join(tempDir, "cros-test", "result", "tauto", "results-1-cheets_GTS_12.internal.all.android.dpi", "cheets_GTS_12.android.dpi", "results", "cts-results", "2024.11.17_11.51.17.458_8891", "test_result.xml.gz"),
						Destination: "gs://result-bucket/cheets_GTS_12.android.dpi/tauto_2024.11.17_11.51.17.458_8891/",
					},
				},
			},
			{
				name:          "success with CTS",
				rootDir:       filepath.Join(tempDir, "cros-test", "result", "tauto"),
				resultFileDir: filepath.Join(tempDir, "cros-test", "result", "tauto", "results-1-cheets_CTS_12.internal.all.android.dpi", "cheets_CTS_12.android.dpi", "results", "cts-results", "2024.11.17_11.51.17.458_8891"),
				metadata: &api.XtsArchiverMetadata{
					ApfeGcsPrefix:        "gs://apfe-bucket",
					ResultsGcsPrefix:     "gs://result-bucket",
					Build:                "brya-release/R100-14543.0.0",
					Product:              "brya.brya",
					ParentSwarmingTaskId: "parent_job_id",
				},
				createZipFile:    true,
				createResultFile: true,
				createTGZFile:    true,
				wantInstr: []ArchiveInfo{
					{
						Name:        "apfe:brya.brya-release/R100-14543.0.0/parent_job_id/cheets_CTS_12.android.dpi/tauto_2024.11.17_11.51.17.458_8891",
						Source:      filepath.Join(tempDir, "cros-test", "result", "tauto", "results-1-cheets_CTS_12.internal.all.android.dpi", "cheets_CTS_12.android.dpi", "results", "cts-results", "2024.11.17_11.51.17.458_8891.zip"),
						Destination: "gs://apfe-bucket/brya.brya-release/R100-14543.0.0/parent_job_id/cheets_CTS_12.android.dpi/tauto_2024.11.17_11.51.17.458_8891/",
					},
					{
						Name:        "results:cheets_CTS_12.android.dpi/tauto_2024.11.17_11.51.17.458_8891",
						Source:      filepath.Join(tempDir, "cros-test", "result", "tauto", "results-1-cheets_CTS_12.internal.all.android.dpi", "cheets_CTS_12.android.dpi", "results", "cts-results", "2024.11.17_11.51.17.458_8891", "test_result.xml.gz"),
						Destination: "gs://result-bucket/cheets_CTS_12.android.dpi/tauto_2024.11.17_11.51.17.458_8891/",
					}, {
						Name:        "results:cheets_CTS_12.android.dpi/tauto_2024.11.17_11.51.17.458_8891",
						Source:      filepath.Join(tempDir, "cros-test", "result", "tauto", "results-1-cheets_CTS_12.internal.all.android.dpi", "cheets_CTS_12.android.dpi", "results", "cts-results", "2024.11.17_11.51.17.458_8891", "test_result.xml.gz"),
						Destination: "gs://result-bucket/cheets_CTS_12.android.dpi/tauto_2024.11.17_11.51.17.458_8891/",
					},
				},
			},
			{
				name:          "success with AL run",
				rootDir:       filepath.Join(tempDir, "cros-test", "result", "tradefed"),
				resultFileDir: filepath.Join(tempDir, "cros-test", "result", "tradefed"),
				metadata: &api.XtsArchiverMetadata{
					AlRun:                true,
					ApfeGcsPrefix:        "gs://apfe-bucket",
					ResultsGcsPrefix:     "gs://result-bucket",
					Build:                "brya-userdebug/P454321",
					Product:              "brya.brya",
					ParentSwarmingTaskId: "parent_job_id",
				},
				createResultFile: true,
				createZipFile:    true,
				wantInstr: []ArchiveInfo{
					{
						Name:        fmt.Sprintf("results:al_project/tradefed_%s", nowStr),
						Source:      filepath.Join(tempDir, "cros-test", "result", "tradefed", "test_result.xml.gz"),
						Destination: fmt.Sprintf("gs://result-bucket/al_project/tradefed_%s/", nowStr),
					},
				},
			},
			{
				name:    "non-release build",
				rootDir: tempDir,
				metadata: &api.XtsArchiverMetadata{
					ApfeGcsPrefix:        "gs://apfe-bucket",
					ResultsGcsPrefix:     "gs://result-bucket",
					Build:                "brya-nonrelease/R100-14543.0.0",
					Product:              "brya.brya",
					ParentSwarmingTaskId: "parent_job_id",
				},
				wantInstr: nil,
				wantErr:   nil,
			},
		}
		for _, tc := range testCases {
			Convey(tc.name, func() {
				if tc.createZipFile {
					zipPath := fmt.Sprintf("%s.zip", tc.resultFileDir)
					if err := os.MkdirAll(tc.resultFileDir, 0755); err != nil {
						t.Fatalf("failed to create directory structure for file: %v", err)
					}
					if _, err := os.Create(zipPath); err != nil {
						t.Fatalf("Failed to create test zip file: %v", err)
					}
				}

				if tc.createResultFile {
					resultPath := filepath.Join(tc.resultFileDir, XTS_RESULT_FILENAME)
					createFile(t, resultPath)
				}
				if tc.createTGZFile {
					tgzResultPath := filepath.Join(tc.resultFileDir, XTS_RESULT_FILENAME_COMPRESSED)
					createTGZFile(t, tgzResultPath, XTS_RESULT_FILENAME)
				}
				defer os.RemoveAll(tc.resultFileDir)

				instr, err := archiveInfo(tc.rootDir, tc.metadata, now)

				if tc.wantErr != nil {
					So(err, ShouldResemble, tc.wantErr)
				} else {
					So(err, ShouldBeNil)
					So(instr, ShouldResemble, tc.wantInstr)
				}
			})
		}
	})
}

func TestArchiveInfoForTest(t *testing.T) {
	Convey("Test archive info for test", t, func() {
		now := testclock.TestTimeUTC
		nowStr := now.Format(AL_PACKAGE_TIMESTAMP_FORMAT)
		tempDir := t.TempDir()
		testCases := []struct {
			name             string
			resultDir        string
			resultFilename   string
			metadata         *api.XtsArchiverMetadata
			wantInstr        []ArchiveInfo
			wantError        error
			createZipFile    bool
			createResultFile bool
		}{
			{
				name:           "success",
				resultDir:      filepath.Join(tempDir, "tauto", "results-1-cheets_CTS_12.android.api", "cheets_CTS.android.dpi", "results", "cts-results", "2024.11.17_15.15.51.364_5161"),
				resultFilename: "test_result.xml.tgz",
				metadata: &api.XtsArchiverMetadata{
					ApfeGcsPrefix:        "gs://apfe-bucket",
					ResultsGcsPrefix:     "gs://result-bucket",
					Build:                "brya-release/R133-16103.0.0",
					Product:              "brya.brya",
					ParentSwarmingTaskId: "parent_job_id",
				},
				createZipFile:    true,
				createResultFile: true,
				wantInstr: []ArchiveInfo{
					{
						Name:        "apfe:brya.brya-release/R133-16103.0.0/parent_job_id/cheets_CTS.android.dpi/tauto_2024.11.17_15.15.51.364_5161",
						Source:      filepath.Join(tempDir, "tauto", "results-1-cheets_CTS_12.android.api", "cheets_CTS.android.dpi", "results", "cts-results", "2024.11.17_15.15.51.364_5161.zip"),
						Destination: "gs://apfe-bucket/brya.brya-release/R133-16103.0.0/parent_job_id/cheets_CTS.android.dpi/tauto_2024.11.17_15.15.51.364_5161/",
					},
					{
						Name:        "results:cheets_CTS.android.dpi/tauto_2024.11.17_15.15.51.364_5161",
						Source:      filepath.Join(tempDir, "tauto", "results-1-cheets_CTS_12.android.api", "cheets_CTS.android.dpi", "results", "cts-results", "2024.11.17_15.15.51.364_5161", "test_result.xml.gz"),
						Destination: "gs://result-bucket/cheets_CTS.android.dpi/tauto_2024.11.17_15.15.51.364_5161/",
					},
				},
			},
			{
				name:           "success with tgz result file",
				resultDir:      filepath.Join(tempDir, "tauto", "results-1-cheets_CTS_12.android.api", "cheets_CTS.android.dpi", "results", "cts-results", "2024.11.17_15.15.51.364_5161"),
				resultFilename: "test_result.xml.tgz",
				metadata: &api.XtsArchiverMetadata{
					ApfeGcsPrefix:        "gs://apfe-bucket",
					ResultsGcsPrefix:     "gs://result-bucket",
					Build:                "brya-release/R133-16103.0.0",
					Product:              "brya.brya",
					ParentSwarmingTaskId: "parent_job_id",
				},
				createZipFile:    true,
				createResultFile: true,
				wantInstr: []ArchiveInfo{
					{
						Name:        "apfe:brya.brya-release/R133-16103.0.0/parent_job_id/cheets_CTS.android.dpi/tauto_2024.11.17_15.15.51.364_5161",
						Source:      filepath.Join(tempDir, "tauto", "results-1-cheets_CTS_12.android.api", "cheets_CTS.android.dpi", "results", "cts-results", "2024.11.17_15.15.51.364_5161.zip"),
						Destination: "gs://apfe-bucket/brya.brya-release/R133-16103.0.0/parent_job_id/cheets_CTS.android.dpi/tauto_2024.11.17_15.15.51.364_5161/",
					},
					{
						Name:        "results:cheets_CTS.android.dpi/tauto_2024.11.17_15.15.51.364_5161",
						Source:      filepath.Join(tempDir, "tauto", "results-1-cheets_CTS_12.android.api", "cheets_CTS.android.dpi", "results", "cts-results", "2024.11.17_15.15.51.364_5161", "test_result.xml.gz"),
						Destination: "gs://result-bucket/cheets_CTS.android.dpi/tauto_2024.11.17_15.15.51.364_5161/",
					},
				},
			},
			{
				name:           "isALRun, skip apfe upload",
				resultDir:      filepath.Join(tempDir, "tradefed"),
				resultFilename: "test_result.xml",
				metadata: &api.XtsArchiverMetadata{
					ApfeGcsPrefix:        "gs://apfe-bucket",
					ResultsGcsPrefix:     "gs://result-bucket",
					Build:                "brya-trunk-userdebug/P16103",
					Product:              "brya.brya",
					ParentSwarmingTaskId: "parent_job_id",
					AlRun:                true,
				},
				createZipFile:    false,
				createResultFile: true,
				wantInstr: []ArchiveInfo{
					{
						Name:        fmt.Sprintf("results:al_project/tradefed_%s", nowStr),
						Source:      filepath.Join(tempDir, "tradefed", "test_result.xml.gz"),
						Destination: fmt.Sprintf("gs://result-bucket/al_project/tradefed_%s/", nowStr),
					},
				},
			},
			{
				name:             "collect-test package with empty result, skip both apfe and result upload",
				resultDir:        filepath.Join(tempDir, "tauto", "results-1-cheets_CTS_12.android.api", "cheets_CTS.android.dpi", "results", "cts-results", "2024.11.17_15.15.51.364_5161"),
				resultFilename:   "test_result.xml",
				createResultFile: false,
				metadata: &api.XtsArchiverMetadata{
					ApfeGcsPrefix:        "gs://apfe-bucket",
					ResultsGcsPrefix:     "gs://result-bucket",
					Build:                "brya-release/R133-16103.0.0",
					Product:              "brya.brya",
					ParentSwarmingTaskId: "parent_job_id",
				},
				createZipFile: false,
			},
			{
				name:      "invalid result dir",
				resultDir: filepath.Join("invalid", "path"),
				metadata: &api.XtsArchiverMetadata{
					ApfeGcsPrefix:        "gs://apfe-bucket",
					ResultsGcsPrefix:     "gs://result-bucket",
					Build:                "brya-release/R133-16103.0.0",
					Product:              "brya.brya",
					ParentSwarmingTaskId: "parent_job_id",
				},
				wantError: fmt.Errorf("invalid tauto path: %s", filepath.Join("invalid", "path")),
			},
		}

		for _, tc := range testCases {
			Convey(tc.name, func() {
				// Create the zip file for APFE instructions
				if tc.createZipFile {
					if err := os.MkdirAll(tc.resultDir, 0755); err != nil {
						t.Fatalf("failed to create directory structure for zip file: %v", err)
					}

					zipPath := fmt.Sprintf("%s.zip", tc.resultDir)
					if _, err := os.Create(zipPath); err != nil {
						t.Fatalf("Failed to create test zip file: %v", err)
					}
				}

				// Create the result file for result info
				if tc.createResultFile {
					resultPath := filepath.Join(tc.resultDir, tc.resultFilename)
					if filepath.Ext(tc.resultFilename) == ".tgz" {
						createTGZFile(t, resultPath, strings.TrimSuffix(tc.resultFilename, ".tgz"))
					} else {
						createFile(t, resultPath)
					}
				}
				defer os.RemoveAll(tc.resultDir)

				instr, err := archiveInfoForTest(tc.resultDir, []string{tc.resultFilename}, tc.metadata, now)

				if tc.wantError != nil {
					So(err, ShouldResemble, tc.wantError)
				} else {
					So(err, ShouldBeNil)
					So(instr, ShouldResemble, tc.wantInstr)
				}
			})
		}
	})
}

func TestApfeInfo(t *testing.T) {
	Convey("Test APFE info", t, func() {
		// Create dir1 under test temp dir
		tempDir := t.TempDir()
		testDir, err := os.MkdirTemp(tempDir, "dir1")
		if err != nil {
			t.Fatal(err)
		}
		zipFilepath := fmt.Sprintf("%s.zip", testDir)
		defer os.RemoveAll(testDir)

		testCases := []struct {
			name          string
			filePrefix    string
			metadata      *api.XtsArchiverMetadata
			createZipFile bool
			wantInstr     []ArchiveInfo
			wantError     error
		}{
			{
				name:       "success",
				filePrefix: "test_prefix",
				metadata: &api.XtsArchiverMetadata{
					ApfeGcsPrefix:        "gs://apfe-bucket",
					Build:                "brya-release/R133-16103.0.0",
					Product:              "brya.brya",
					ParentSwarmingTaskId: "parent_job_id",
				},
				createZipFile: true,
				wantInstr: []ArchiveInfo{
					{
						Name:        "apfe:brya.brya-release/R133-16103.0.0/parent_job_id/test_prefix",
						Source:      zipFilepath,
						Destination: "gs://apfe-bucket/brya.brya-release/R133-16103.0.0/parent_job_id/test_prefix/",
					},
				},
			},
			{
				name:       "no zip files",
				filePrefix: "test_prefix",
				metadata: &api.XtsArchiverMetadata{
					ApfeGcsPrefix:        "gs://apfe-bucket",
					Build:                "brya.brya-release/R133-16103.0.0",
					Product:              "brya.brya",
					ParentSwarmingTaskId: "parent_job_id",
				},
				wantInstr: []ArchiveInfo{},
			},
			{
				name:       "invalid build string",
				filePrefix: "test_prefix",
				metadata: &api.XtsArchiverMetadata{
					ApfeGcsPrefix:        "gs://apfe-bucket",
					Build:                "invalid-build",
					Product:              "brya.brya",
					ParentSwarmingTaskId: "parent_job_id",
				},
				wantError: fmt.Errorf("failed to parse build string: invalid-build"),
			},
		}

		for _, tc := range testCases {
			Convey(tc.name, func() {
				if tc.createZipFile {
					zipFile, err := os.Create(zipFilepath)
					if err != nil {
						t.Fatalf("Failed to create test zip file: %v", err)
					}
					zipFile.Close()
				}

				instr, err := apfeInfo(testDir, tc.filePrefix, tc.metadata)

				if tc.wantError != nil {
					So(err, ShouldResemble, tc.wantError)
				} else {
					So(err, ShouldBeNil)
					So(instr, ShouldResemble, tc.wantInstr)
				}
			})
		}
	})
}

func TestResultsInfo(t *testing.T) {
	Convey("Test result info", t, func() {
		resultDir := t.TempDir()
		filePrefix := "test_prefix"
		metadata := &api.XtsArchiverMetadata{
			ResultsGcsPrefix: "gs://result-bucket",
		}
		destination := fmt.Sprintf("%s/%s/", metadata.ResultsGcsPrefix, filePrefix)

		testCases := []struct {
			name           string
			resultDir      string
			resultFilename string
			createFile     bool
			wantInstr      []ArchiveInfo
			wantError      error
		}{
			{
				name:           "success",
				resultFilename: "testResult.xml",
				createFile:     true,
				wantInstr: []ArchiveInfo{
					{
						Name:        "results:test_prefix",
						Source:      filepath.Join(resultDir, "testResult.xml.gz"),
						Destination: destination,
					},
				},
			},
			{
				name:           "success with tgz extraction",
				resultFilename: "test_result.xml.tgz",
				createFile:     true,
				wantInstr: []ArchiveInfo{
					{
						Name:        "results:test_prefix",
						Source:      filepath.Join(resultDir, "test_result.xml.gz"),
						Destination: destination,
					},
				},
			},
			{
				name:           "no result files",
				resultFilename: "nonexistent.xml",
				createFile:     false,
				wantInstr:      []ArchiveInfo{},
			},
		}

		for _, tc := range testCases {
			Convey(tc.name, func() {
				if tc.createFile {
					filePath := filepath.Join(resultDir, tc.resultFilename)
					if filepath.Ext(tc.resultFilename) == ".tgz" {
						createTGZFile(t, filePath, strings.TrimSuffix(tc.resultFilename, ".tgz"))
					} else {
						createFile(t, filePath)
					}
					defer os.RemoveAll(resultDir)
				}

				instr, err := resultsInfo(resultDir, tc.resultFilename, filePrefix, metadata)

				if tc.wantError != nil {
					So(err, ShouldResemble, tc.wantError)
				} else {
					So(err, ShouldBeNil)
					So(instr, ShouldResemble, tc.wantInstr)
				}
			})
		}
	})
}

func TestParseJobResultsFilePath(t *testing.T) {
	Convey("Test parse job results filepath", t, func() {
		now := testclock.TestTimeUTC
		testCases := []struct {
			name        string
			path        string
			isALRun     bool
			wantHarness string
			wantPkg     string
			wantTs      string
			wantError   error
		}{
			{
				name:        "tauto path",
				path:        "/cros-test/results/tauto/results-1-cheets_GTS_12.internal.all.WvtsDeviceTestCases/cheets_GTS_12.internal.all.WvtsDeviceTestCases/results/android-gts/2024.11.17_15.15.51.364_5161",
				wantHarness: "tauto",
				wantPkg:     "cheets_GTS_12.internal.all.WvtsDeviceTestCases",
				wantTs:      "2024.11.17_15.15.51.364_5161",
			},
			{
				name:      "invalid tauto path: less than 6",
				path:      "cros-test/results/tauto/results-1-cheets_GTS_12.internal.all.WvtsDeviceTestCases",
				wantError: fmt.Errorf("invalid tauto path: cros-test/results/tauto/results-1-cheets_GTS_12.internal.all.WvtsDeviceTestCases"),
			},
			{
				name:        "tradefed path",
				path:        "/cros-test/results/tradefed",
				isALRun:     true,
				wantHarness: "tradefed",
				wantPkg:     AL_PROJECT_PREFIX,
				wantTs:      now.Format(AL_PACKAGE_TIMESTAMP_FORMAT),
			},
			{
				name:      "invalid tradefed path: empty",
				path:      "",
				isALRun:   true,
				wantError: fmt.Errorf("invalid tradefed path: "),
			},
		}

		for _, tc := range testCases {
			Convey(tc.name, func() {
				harness, packageName, timestamp, err := parseJobResultsFilePath(tc.path, tc.isALRun, now)

				if tc.wantError != nil {
					So(err, ShouldResemble, tc.wantError)
				} else {
					So(err, ShouldBeNil)
					So(harness, ShouldEqual, tc.wantHarness)
					So(packageName, ShouldEqual, tc.wantPkg)
					So(timestamp, ShouldEqual, tc.wantTs)
				}
			})
		}
	})
}

func createTGZFile(t testing.TB, tgzFile, xmlFilename string) {
	file, err := os.Create(tgzFile)
	if err != nil {
		t.Fatalf("failed to create test tgz file: %v", err)
	}
	defer file.Close()

	gzw := gzip.NewWriter(file)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	xmlBytes := []byte("<test></test>")
	hdr := &tar.Header{
		Name: xmlFilename,
		Mode: 0600,
		Size: int64(len(xmlBytes)),
	}

	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("failed to write tar header for test xml file: %v", err)
	}

	if _, err := tw.Write(xmlBytes); err != nil {
		t.Fatalf("failed to write tar file contents for test xml file: %v", err)
	}
}

func TestIsReleaseBuild(t *testing.T) {
	Convey("Test isReleaseBuild", t, func() {
		testCases := []struct {
			build     string
			isALRun   bool
			isRelease bool
		}{
			{
				build:     "brya-release/R99-12345.0.0",
				isALRun:   false,
				isRelease: true,
			},
			{
				build:     "brya-release/R99-12345.0.0-test",
				isALRun:   false,
				isRelease: true,
			},
			{
				build:     "a-very-long-board.name-release/R99-12345.0.0",
				isALRun:   false,
				isRelease: true,
			},
			{
				build:     "brya-release.staging/R99-12345.0.0",
				isALRun:   false,
				isRelease: false,
			},
			{
				build:     "trybot-brya-release/R99-12345.0.0",
				isALRun:   false,
				isRelease: false,
			},
			{
				build:     "brya-kernelnext/R99-12345.0.0",
				isALRun:   false,
				isRelease: false,
			},
			{
				build:     "brya/R99-12345.0.0",
				isALRun:   false,
				isRelease: false,
			},
			{
				build:     "brya-userdebug/P12345",
				isALRun:   true,
				isRelease: true,
			},
			{
				build:     "brya-trunk-userdebug/P12345",
				isALRun:   true,
				isRelease: true,
			},
			{
				build:     "brya-next-userdebug/P12345",
				isALRun:   true,
				isRelease: true,
			},
			{
				build:     "brya-trunk_staging-userdebug/P12345",
				isALRun:   true,
				isRelease: false,
			},
			{
				build:     "brya-trunk_food-userdebug/P12345",
				isALRun:   true,
				isRelease: false,
			},
		}

		for _, tc := range testCases {
			Convey(tc.build, func() {
				gotIsRelease := isReleaseBuild(tc.build, tc.isALRun)
				So(gotIsRelease, ShouldEqual, tc.isRelease)
			})
		}
	})
}

func TestValidateXTSArchiverMetadata(t *testing.T) {
	Convey("Test validateXTSArchiverMetadata", t, func() {
		testCases := []struct {
			name     string
			metadata *api.XtsArchiverMetadata
			wantErr  error
		}{
			{
				name: "success",
				metadata: &api.XtsArchiverMetadata{
					Build:            "build",
					ResultsGcsPrefix: "results_gcs_prefix",
					ApfeGcsPrefix:    "apfe_gcs_prefix",
				},
				wantErr: nil,
			},
			{
				name: "missing build",
				metadata: &api.XtsArchiverMetadata{
					ResultsGcsPrefix: "results_gcs_prefix",
					ApfeGcsPrefix:    "apfe_gcs_prefix",
				},
				wantErr: fmt.Errorf("build is required in metadata for XTS archiver"),
			},
			{
				name: "missing results gcs prefix",
				metadata: &api.XtsArchiverMetadata{
					Build:         "build",
					ApfeGcsPrefix: "apfe_gcs_prefix",
				},
				wantErr: fmt.Errorf("results GCS prefix is required in metadata for XTS archiver"),
			},
			{
				name: "missing apfe gcs prefix",
				metadata: &api.XtsArchiverMetadata{
					Build:            "build",
					ResultsGcsPrefix: "results_gcs_prefix",
				},
				wantErr: fmt.Errorf("APFE GCS prefix is required in metadata for XTS archiver"),
			},
		}

		for _, tc := range testCases {
			Convey(tc.name, func() {
				err := validateXTSArchiverMetadata(tc.metadata)
				So(err, ShouldResemble, tc.wantErr)
			})
		}
	})
}
