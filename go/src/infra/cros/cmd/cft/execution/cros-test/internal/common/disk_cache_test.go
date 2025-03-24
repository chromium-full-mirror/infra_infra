// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Helper function to create a test file
func createTestFile(t *testing.T, filePath string, content string) {
	t.Helper()
	err := os.MkdirAll(filepath.Dir(filePath), 0755)
	if err != nil {
		t.Fatalf("error pre-creating cache directory: %v", err)
	}
	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
}

func TestNewDiskCache(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "test_cache_TestNewDiskCache")
	maxSize := int64(1024 * 1024)

	cache, err := NewDiskCache(maxSize, cacheDir)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	if cache.maxSize != maxSize {
		t.Errorf("NewDiskCache: expected maxSize %d, got %d", maxSize, cache.maxSize)
	}
	if cache.cacheDir != cacheDir {
		t.Errorf("NewDiskCache: expected cacheDir %s, got %s", cacheDir, cache.cacheDir)
	}
	if cache.currentSize != 0 {
		t.Errorf("NewDiskCache: expected currentSize 0, got %d", cache.currentSize)
	}
	if len(cache.files) != 0 {
		t.Errorf("NewDiskCache: expected empty files map, got %d", len(cache.files))
	}
	if len(cache.accessTimes) != 0 {
		t.Errorf("NewDiskCache: expected empty accessTimes map, got %d", len(cache.accessTimes))
	}

	// Clean up test directory
	os.RemoveAll(cacheDir)
}

func TestNewDiskCacheWithExistingFiles(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "test_cache_TestNewDiskCacheWithExistingFiles")
	maxSize := int64(1024 * 1024)

	// Create a test file in the cache directory
	testFile := filepath.Join(cacheDir, "1__test_file")
	createTestFile(t, testFile, "test content")

	cache, err := NewDiskCache(maxSize, cacheDir)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}
	if cache.currentSize == 0 {
		t.Errorf("NewDiskCache: expected currentSize > 0, got %d", cache.currentSize)
	}
	if len(cache.files) != 1 {
		t.Errorf("NewDiskCache: expected files map with 1 entry, got %d", len(cache.files))
	}
	if len(cache.accessTimes) != 1 {
		t.Errorf("NewDiskCache: expected accessTimes map with 1 entry, got %d", len(cache.accessTimes))
	}

	_, exists := cache.files["test_file"][1]
	if !exists {
		t.Error("NewDiskCache: expected test_file to exist in files map")
	}

	_, exists = cache.accessTimes["test_file"][1]
	if !exists {
		t.Error("NewDiskCache: expected test_file to exist in accessTimes map")
	}

	// Clean up test directory
	os.RemoveAll(cacheDir)
}

func TestCreateFile(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "test_cache_TestCreateFile")
	maxSize := int64(1024 * 1024)
	cache, err := NewDiskCache(maxSize, cacheDir)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	filename := "test_file"
	buildID := 1
	expectedSize := int64(1024)

	targetFile, err := cache.CreateFile(filename, buildID, expectedSize, true)
	if targetFile == nil {
		t.Fatalf("CreateFile failed: %v", err)
	}
	if err != nil {
		t.Fatalf("CreateFile failed: %v", err)
	}

	cachePath := filepath.Join(cacheDir, fmt.Sprintf("%d__%s", buildID, filename))
	fileInfo, err := os.Stat(cachePath)
	if err != nil {
		t.Fatalf("Failed to stat created file: %v", err)
	}
	if fileInfo.Size() != expectedSize {
		t.Errorf("CreateFile: expected file size %d, got %d", expectedSize, fileInfo.Size())
	}
	if cache.currentSize != expectedSize {
		t.Errorf("CreateFile: expected currentSize %d, got %d", expectedSize, cache.currentSize)
	}
	if len(cache.files) != 1 {
		t.Errorf("CreateFile: expected 1 file in files map, got %d", len(cache.files))
	}
	if len(cache.accessTimes) != 1 {
		t.Errorf("CreateFile: expected 1 file in accessTimes map, got %d", len(cache.accessTimes))
	}
	_, exists := cache.files[filename][buildID]
	if !exists {
		t.Errorf("CreateFile: expected file to exist in files map")
	}
	_, exists = cache.accessTimes[filename][buildID]
	if !exists {
		t.Errorf("CreateFile: expected file to exist in accessTimes map")
	}

	// Clean up test directory
	os.RemoveAll(cacheDir)
}

func TestCreateFileCollision(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "test_cache_TestCreateFileCollision")
	maxSize := int64(1024 * 1024)
	cache, err := NewDiskCache(maxSize, cacheDir)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	filename := "test_file"
	buildID := 1
	expectedSize := int64(1024)

	targetFile, err := cache.CreateFile(filename, buildID, expectedSize, true)
	if targetFile == nil {
		t.Fatalf("CreateFile failed: %v", err)
	}
	if err != nil {
		t.Fatalf("CreateFile failed: %v", err)
	}

	targetFile, err = cache.CreateFile(filename, buildID, expectedSize, true)
	if targetFile == nil {
		t.Fatalf("CreateFile failed: %v", err)
	}
	if err != nil {
		t.Fatalf("CreateFile failed: %v", err)
	}

	cachePath := filepath.Join(cacheDir, fmt.Sprintf("%d__1__%s", buildID, filename))

	_, err = os.Stat(cachePath)
	if err != nil {
		t.Fatalf("Failed to stat created file: %v", err)
	}

	if len(cache.files) != 1 {
		t.Errorf("CreateFile: expected 1 file in files map, got %d", len(cache.files))
	}
	if len(cache.accessTimes) != 1 {
		t.Errorf("CreateFile: expected 1 file in accessTimes map, got %d", len(cache.accessTimes))
	}

	if cache.currentSize != expectedSize {
		t.Errorf("CreateFile: expected currentSize %d, got %d", expectedSize, cache.currentSize)
	}
	_, exists := cache.files[filename][buildID]
	if !exists {
		t.Errorf("CreateFile: expected file to exist in files map")
	}
	_, exists = cache.accessTimes[filename][buildID]
	if !exists {
		t.Errorf("CreateFile: expected file to exist in accessTimes map")
	}

	// Clean up test directory
	os.RemoveAll(cacheDir)
}

func TestCreateFileExceedsLimit(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "test_cache_TestCreateFileExceedsLimit")
	maxSize := int64(2048)
	cache, err := NewDiskCache(maxSize, cacheDir)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	filename1 := "test_file1"
	buildID1 := 1
	expectedSize1 := int64(1024)

	targetFile, err := cache.CreateFile(filename1, buildID1, expectedSize1, true)
	if targetFile == nil {
		t.Fatalf("CreateFile failed: %v", err)
	}
	if err != nil {
		t.Fatalf("CreateFile failed: %v", err)
	}

	filename2 := "test_file2"
	buildID2 := 1
	expectedSize2 := int64(2048)

	targetFile, err = cache.CreateFile(filename2, buildID2, expectedSize2, true)
	if targetFile == nil {
		t.Fatalf("CreateFile failed: %v", err)
	}
	if err != nil {
		t.Fatalf("CreateFile failed: %v", err)
	}

	cachePath1 := filepath.Join(cacheDir, fmt.Sprintf("%d__%s", buildID1, filename1))
	cachePath2 := filepath.Join(cacheDir, fmt.Sprintf("%d__%s", buildID2, filename2))

	_, err = os.Stat(cachePath1)
	if !os.IsNotExist(err) {
		t.Errorf("CreateFile: expected file %s to not exist", cachePath1)
	}
	_, err = os.Stat(cachePath2)
	if err != nil {
		t.Fatalf("Failed to stat created file: %v", err)
	}

	if cache.currentSize != expectedSize2 {
		t.Errorf("CreateFile: expected currentSize %d, got %d", expectedSize2, cache.currentSize)
	}
	if len(cache.files) != 1 {
		t.Errorf("CreateFile: expected 1 file in files map, got %d", len(cache.files))
	}
	if len(cache.accessTimes) != 1 {
		t.Errorf("CreateFile: expected 1 file in accessTimes map, got %d", len(cache.accessTimes))
	}
	_, exists := cache.files[filename2][buildID2]
	if !exists {
		t.Errorf("CreateFile: expected file to exist in files map")
	}
	_, exists = cache.accessTimes[filename2][buildID2]
	if !exists {
		t.Errorf("CreateFile: expected file to exist in accessTimes map")
	}

	// Clean up test directory
	os.RemoveAll(cacheDir)
}

func TestAddFile(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "test_cache_TestAddFile")
	maxSize := int64(1024 * 1024)
	cache, err := NewDiskCache(maxSize, cacheDir)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	filename := "test_file"
	buildID := 1
	testFilePath := filepath.Join(t.TempDir(), "test_file_content_TestAddFile")
	createTestFile(t, testFilePath, "test content")

	err = cache.AddFile(filename, buildID, testFilePath)
	if err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	cachePath := filepath.Join(cacheDir, fmt.Sprintf("%d__%s", buildID, filename))
	fileInfo, err := os.Stat(cachePath)
	if err != nil {
		t.Fatalf("Failed to stat created file: %v", err)
	}
	if fileInfo.Size() != 12 {
		t.Errorf("AddFile: expected file size %d, got %d", 12, fileInfo.Size())
	}
	if cache.currentSize != 12 {
		t.Errorf("AddFile: expected currentSize %d, got %d", 12, cache.currentSize)
	}
	if len(cache.files) != 1 {
		t.Errorf("AddFile: expected 1 file in files map, got %d", len(cache.files))
	}
	if len(cache.accessTimes) != 1 {
		t.Errorf("AddFile: expected 1 file in accessTimes map, got %d", len(cache.accessTimes))
	}
	_, exists := cache.files[filename][buildID]
	if !exists {
		t.Errorf("AddFile: expected file to exist in files map")
	}
	_, exists = cache.accessTimes[filename][buildID]
	if !exists {
		t.Errorf("AddFile: expected file to exist in accessTimes map")
	}
	// Clean up test directory
	os.RemoveAll(cacheDir)
}

func TestAddFileCollision(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "test_cache_TestAddFileCollision")
	maxSize := int64(1024 * 1024)
	cache, err := NewDiskCache(maxSize, cacheDir)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	filename := "test_file"
	buildID := 1
	testFilePath1 := filepath.Join(t.TempDir(), "test_file_content1_TestAddFileCollision")
	createTestFile(t, testFilePath1, "test content")

	testFilePath2 := filepath.Join(t.TempDir(), "test_file_content2_TestAddFileCollision")
	createTestFile(t, testFilePath2, "different content")

	err = cache.AddFile(filename, buildID, testFilePath1)
	if err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	err = cache.AddFile(filename, buildID, testFilePath2)
	if err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	cachePath := filepath.Join(cacheDir, fmt.Sprintf("%d__1__%s", buildID, filename))

	fileInfo, err := os.Stat(cachePath)
	if err != nil {
		t.Fatalf("Failed to stat created file: %v", err)
	}
	if fileInfo.Size() != 17 {
		t.Errorf("AddFile: expected file size %d, got %d", 17, fileInfo.Size())
	}
	if cache.currentSize != 17 {
		t.Errorf("AddFile: expected currentSize %d, got %d", 17, cache.currentSize)
	}
	if len(cache.files) != 1 {
		t.Errorf("AddFile: expected 1 file in files map, got %d", len(cache.files))
	}
	if len(cache.accessTimes) != 1 {
		t.Errorf("AddFile: expected 1 file in accessTimes map, got %d", len(cache.accessTimes))
	}
	_, exists := cache.files[filename][buildID]
	if !exists {
		t.Errorf("AddFile: expected file to exist in files map")
	}
	_, exists = cache.accessTimes[filename][buildID]
	if !exists {
		t.Errorf("AddFile: expected file to exist in accessTimes map")
	}

	// Clean up test directory
	os.RemoveAll(cacheDir)
}

func TestAddFileExceedsLimit(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "test_cache_TestAddFileExceedsLimit")
	maxSize := int64(20)
	cache, err := NewDiskCache(maxSize, cacheDir)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	filename1 := "test_file1"
	buildID1 := 1
	testFilePath1 := filepath.Join(t.TempDir(), "test_file_content1_TestAddFileExceedsLimit")
	createTestFile(t, testFilePath1, "test content")

	err = cache.AddFile(filename1, buildID1, testFilePath1)
	if err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	filename2 := "test_file2"
	buildID2 := 1
	testFilePath2 := filepath.Join(t.TempDir(), "test_file_content2_TestAddFileExceedsLimit")
	createTestFile(t, testFilePath2, "longer test content")

	err = cache.AddFile(filename2, buildID2, testFilePath2)
	if err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	cachePath1 := filepath.Join(cacheDir, fmt.Sprintf("%d__%s", buildID1, filename1))
	cachePath2 := filepath.Join(cacheDir, fmt.Sprintf("%d__%s", buildID2, filename2))

	_, err = os.Stat(cachePath1)
	if !os.IsNotExist(err) {
		t.Errorf("AddFile: expected file %s to not exist", cachePath1)
	}
	fileInfo, err := os.Stat(cachePath2)
	if err != nil {
		t.Fatalf("Failed to stat created file: %v", err)
	}
	if fileInfo.Size() != 19 {
		t.Errorf("AddFile: expected file size %d, got %d", 19, fileInfo.Size())
	}

	if cache.currentSize != 19 {
		t.Errorf("AddFile: expected currentSize %d, got %d", 19, cache.currentSize)
	}
	if len(cache.files) != 1 {
		t.Errorf("AddFile: expected 1 file in files map, got %d", len(cache.files))
	}
	if len(cache.accessTimes) != 1 {
		t.Errorf("AddFile: expected 1 file in accessTimes map, got %d", len(cache.accessTimes))
	}
	_, exists := cache.files[filename2][buildID2]
	if !exists {
		t.Errorf("AddFile: expected file to exist in files map")
	}
	_, exists = cache.accessTimes[filename2][buildID2]
	if !exists {
		t.Errorf("AddFile: expected file to exist in accessTimes map")
	}

	// Clean up test directory
	os.RemoveAll(cacheDir)

}

func TestGetFile(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "test_cache_TestGetFile")
	maxSize := int64(1024 * 1024)
	cache, err := NewDiskCache(maxSize, cacheDir)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	filename := "test_file"
	buildID := 1
	testFilePath := filepath.Join(t.TempDir(), "test_file_content_TestGetFile")
	createTestFile(t, testFilePath, "test content")
	err = cache.AddFile(filename, buildID, testFilePath)
	if err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	filePath, exists := cache.GetFile(filename, buildID)
	if !exists {
		t.Fatalf("GetFile: expected file to exist")
	}
	expectedPath := filepath.Join(cacheDir, fmt.Sprintf("%d__%s", buildID, filename))
	if filePath != expectedPath {
		t.Errorf("GetFile: expected path %s, got %s", expectedPath, filePath)
	}

	_, exists = cache.accessTimes[filename][buildID]
	if !exists {
		t.Errorf("GetFile: expected access time to exist")
	}

	// Clean up test directory
	os.RemoveAll(cacheDir)
}

func TestGetFileNotExists(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "test_cache_TestGetFileNotExists")
	maxSize := int64(1024 * 1024)
	cache, err := NewDiskCache(maxSize, cacheDir)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	filename := "test_file"
	buildID := 1
	filePath, exists := cache.GetFile(filename, buildID)
	if exists {
		t.Errorf("GetFile: expected file to not exist, got %s", filePath)
	}

	_, exists = cache.accessTimes[filename][buildID]
	if exists {
		t.Errorf("GetFile: expected access time to not exist")
	}
	// Clean up test directory
	os.RemoveAll(cacheDir)
}

func TestGetLatestFile(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "test_cache_TestGetLatestFile")
	maxSize := int64(1024 * 1024)
	cache, err := NewDiskCache(maxSize, cacheDir)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	filename := "test_file"
	buildID1 := 1
	testFilePath1 := filepath.Join(t.TempDir(), "test_file_content1_TestGetLatestFile")
	createTestFile(t, testFilePath1, "test content")
	err = cache.AddFile(filename, buildID1, testFilePath1)
	if err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	buildID2 := 2
	testFilePath2 := filepath.Join(t.TempDir(), "test_file_content2_TestGetLatestFile")
	createTestFile(t, testFilePath2, "different content")
	err = cache.AddFile(filename, buildID2, testFilePath2)
	if err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	filePath, exists := cache.GetLatestFile(filename)
	if !exists {
		t.Fatalf("GetLatestFile: expected file to exist")
	}
	expectedPath := filepath.Join(cacheDir, fmt.Sprintf("%d__%s", buildID2, filename))
	if filePath != expectedPath {
		t.Errorf("GetLatestFile: expected path %s, got %s", expectedPath, filePath)
	}

	_, exists = cache.accessTimes[filename][buildID2]
	if !exists {
		t.Errorf("GetLatestFile: expected access time to exist")
	}

	// Clean up test directory
	os.RemoveAll(cacheDir)
}

func TestGetLatestFileNotExists(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "test_cache_TestGetLatestFileNotExists")
	maxSize := int64(1024 * 1024)
	cache, err := NewDiskCache(maxSize, cacheDir)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	filename := "test_file"
	filePath, exists := cache.GetLatestFile(filename)
	if exists {
		t.Errorf("GetLatestFile: expected file to not exist, got %s", filePath)
	}

	// Clean up test directory
	os.RemoveAll(cacheDir)
}
