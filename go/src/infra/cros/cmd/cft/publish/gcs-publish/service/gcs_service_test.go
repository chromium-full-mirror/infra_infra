// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build !windows
// +build !windows

package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/mock/gomock"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/cros/cmd/cft/publish/commonutils/storage"
	"go.chromium.org/infra/cros/cmd/cft/publish/commonutils/storage/mock_storage"
)

func TestArchiveXTSREsultsWithChromeOSResult(t *testing.T) {
	ftt.Run("Test ArchiveXTSResults with ChromeOS result", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctx, _ = testclock.UseTime(ctx, testclock.TestTimeUTC)
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockStorage := mock_storage.NewMockStorageClientInterface(mockCtrl)
		gsClient := storage.NewGSTestClient(mockStorage)

		tempDir := t.TempDir()
		tc := struct {
			name              string
			localArtifactPath string
			resultDir         string
			metadata          *api.XtsArchiverMetadata
			createZipFile     bool
			createResultFile  bool
			wantErr           error
			wantMockCalls     []*gomock.Call
		}{

			name:              "success ChromeOS run",
			localArtifactPath: tempDir,
			resultDir:         filepath.Join(tempDir, "test_prefix", "cros-test", "results", "tauto", "results-1-cheets_CTS_12.internal.all.android.dpi", "cheets_CTS_12.android.dpi", "results", "cts-results", "2024.11.17_11.51.17.458_8891"),
			metadata: &api.XtsArchiverMetadata{
				AlRun:                false,
				ApfeGcsPrefix:        "gs://apfe-bucket",
				ResultsGcsPrefix:     "gs://result-bucket",
				Build:                "brya-release/R100-14543.0.0",
				Product:              "brya.brya",
				ParentSwarmingTaskId: "parent_job_id",
			},
			createZipFile:    true,
			createResultFile: true,
			wantMockCalls: []*gomock.Call{
				mockStorage.EXPECT().Write(
					gomock.Any(),
					filepath.Join(tempDir, "test_prefix", "cros-test", "results", "tauto", "results-1-cheets_CTS_12.internal.all.android.dpi", "cheets_CTS_12.android.dpi", "results", "cts-results", "2024.11.17_11.51.17.458_8891.zip"),
					storage.GSObject{
						Bucket: "apfe-bucket",
						Object: "brya.brya-release/R100-14543.0.0/parent_job_id/cheets_CTS_12.android.dpi/tauto_2024.11.17_11.51.17.458_8891/2024.11.17_11.51.17.458_8891.zip",
					}).Return(nil),
				mockStorage.EXPECT().Write(
					gomock.Any(), filepath.Join(tempDir, "test_prefix", "cros-test", "results", "tauto", "results-1-cheets_CTS_12.internal.all.android.dpi", "cheets_CTS_12.android.dpi", "results", "cts-results", "2024.11.17_11.51.17.458_8891", "test_result.xml.gz"),
					storage.GSObject{
						Bucket: "result-bucket",
						Object: "cheets_CTS_12.android.dpi/tauto_2024.11.17_11.51.17.458_8891/test_result.xml.gz",
					}).Return(nil),
			},
		}

		if tc.createZipFile {
			zipPath := fmt.Sprintf("%s.zip", tc.resultDir)
			if err := os.MkdirAll(tc.resultDir, 0755); err != nil {
				t.Fatalf("failed to create directory structure for file: %v", err)
			}
			if _, err := os.Create(zipPath); err != nil {
				t.Fatalf("failed to create test zip file: %v", err)
			}
		}
		if tc.createResultFile {
			resultPath := filepath.Join(tc.resultDir, "test_result.xml")
			createFile(t, resultPath)
		}
		defer os.RemoveAll(tc.resultDir)

		gs := &GcsPublishService{
			Client:                      gsClient,
			LocalArtifactPath:           tc.localArtifactPath,
			ServiceAccountCredsFilePath: "/path/to/credentials",
			XtsArchiverMetadata:         tc.metadata,
		}
		err := gs.ArchiveXTSResults(ctx)

		gomock.InOrder(tc.wantMockCalls...)
		assert.Loosely(t, err, should.Resemble(tc.wantErr))
	})
}

func TestArchiveXTSResultsWithALResult(t *testing.T) {
	ftt.Run("Test ArchiveXTSResults with AL result", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctx, _ = testclock.UseTime(ctx, testclock.TestTimeUTC)
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockStorage := mock_storage.NewMockStorageClientInterface(mockCtrl)
		gsClient := storage.NewGSTestClient(mockStorage)
		tempDir := t.TempDir()
		resultDir := filepath.Join(tempDir, "test_prefix", "cros-test", "results", "tradefed")
		gs := &GcsPublishService{
			Client:                      gsClient,
			LocalArtifactPath:           tempDir,
			ServiceAccountCredsFilePath: "/path/to/credentials",
			XtsArchiverMetadata: &api.XtsArchiverMetadata{
				AlRun:                true,
				ApfeGcsPrefix:        "gs://apfe-bucket",
				ResultsGcsPrefix:     "gs://result-bucket",
				Build:                "brya-trunk-userdebug/P1454321",
				Product:              "brya.brya",
				ParentSwarmingTaskId: "parent_job_id",
			},
		}
		wantMockCalls := []*gomock.Call{
			mockStorage.EXPECT().Write(
				gomock.Any(), filepath.Join(resultDir, "test_result.xml.gz"),
				storage.GSObject{
					Bucket: "result-bucket",
					Object: fmt.Sprintf("al_project/tradefed_%s/test_result.xml.gz", testclock.TestTimeUTC.Format(AL_PACKAGE_TIMESTAMP_FORMAT)),
				}).Return(nil)}

		if err := os.MkdirAll(resultDir, 0755); err != nil {
			t.Fatalf("failed to create directory structure for file: %v", err)
		}
		resultPath := filepath.Join(resultDir, "test_result.xml")
		createFile(t, resultPath)
		defer os.RemoveAll(resultDir)

		err := gs.ArchiveXTSResults(ctx)

		assert.Loosely(t, err, should.BeNil)
		gomock.InOrder(wantMockCalls...)
	})
}

func createFile(t testing.TB, path string) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create directory structure for file: %v", err)
	}
	if _, err := os.Create(path); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
}
