// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

// Mock the http client for testing
type mockHTTPClient struct {
	responses []*http.Response
	requests  []*http.Request
	lock      sync.Mutex
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.requests = append(m.requests, req)
	if resp := m.responses[0]; resp != nil {
		m.responses = m.responses[1:]
		return resp, nil
	}
	return nil, fmt.Errorf("no more responses defined")
}

func (m *mockHTTPClient) addResponse(url string, resp *http.Response) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.responses = append(m.responses, resp)
}

func (m *mockHTTPClient) getRequests() []*http.Request {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.requests
}

func (m *mockHTTPClient) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.Do(req)
}

func createMockClient() *mockHTTPClient {
	return &mockHTTPClient{
		responses: make([]*http.Response, 0),
		requests:  make([]*http.Request, 0),
		lock:      sync.Mutex{},
	}
}

// Helper function to create a dummy zip file for testing
func createDummyZip(t *testing.T, zipPath string) {
	t.Helper()
	_, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Failed to create dummy zip file: %v", err)
	}
}

func TestDownloadChunk(t *testing.T) {
	// Create a mock http client
	mockClient := createMockClient()

	// Create a test file
	testFile, err := os.CreateTemp("", "test_file")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(testFile.Name())
	defer testFile.Close()

	// Create a test response
	testData := []byte("This is a test chunk")
	testResp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(testData)),
	}
	mockClient.addResponse("http://test.com/file", testResp)

	// Inject the mock client
	downloadChunkClient = func() *http.Client {
		return &http.Client{Transport: mockClient}
	}

	// Call downloadChunk
	err = downloadChunk("http://test.com/file", testFile, 0, int64(len(testData)-1), 0)
	if err != nil {
		t.Fatalf("downloadChunk failed: %v", err)
	}

	// Verify that the file content is correct
	fileContent, err := os.ReadFile(testFile.Name())
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if !bytes.Equal(fileContent, testData) {
		t.Errorf("File content is not correct: got %s, want %s", string(fileContent), string(testData))
	}
}

// TODO: Fix and re-enable
func TestDownloadURLParallel(t *testing.T) {
	// Create a mock http client
	mockClient := createMockClient()

	// Create a test file
	testFile, err := os.CreateTemp("", "test_file")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(testFile.Name())
	defer testFile.Close()

	// Set up mock responses for multiple chunks
	chunkData := []byte("This is the data chunk")
	fullData := append(chunkData, chunkData...)

	mockClient.addResponse("http://test.com/file", &http.Response{
		StatusCode: 206,
		Body:       io.NopCloser(bytes.NewReader(chunkData)),
		Header:     http.Header{"Content-Length": {fmt.Sprint(len(chunkData))}},
	})

	mockClient.addResponse("http://test.com/file", &http.Response{
		StatusCode: 206,
		Body:       io.NopCloser(bytes.NewReader(chunkData)),
		Header:     http.Header{"Content-Length": {fmt.Sprint(len(chunkData))}},
	})

	downloadChunkClient = func() *http.Client {
		return &http.Client{Transport: mockClient}
	}

	// Call downloadURLParallel
	err = downloadURLParallel("http://test.com/file", testFile, int64(len(fullData)), 2)
	if err != nil {
		t.Fatalf("downloadURLParallel failed: %v", err)
	}

	// Verify that the file content is correct
	fileContent, err := os.ReadFile(testFile.Name())
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if !bytes.Equal(fileContent, fullData) {
		t.Errorf("File content is not correct: got %s, want %s", string(fileContent), string(fullData))
	}

	// Verify the number of requests
	requests := mockClient.getRequests()
	if len(requests) != 2 {
		t.Errorf("Expected 2 requests, but made %d", len(requests))
	}
}

func TestDownloadURLParallelError(t *testing.T) {
	// Create a mock http client
	mockClient := createMockClient()

	// Create a test file
	testFile, err := os.CreateTemp("", "test_file")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(testFile.Name())
	defer testFile.Close()

	// Set up mock responses for multiple chunks
	chunk1Data := []byte("This is the first chunk")
	mockClient.addResponse("http://test.com/file", &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(chunk1Data)),
		Header:     http.Header{"Content-Length": {fmt.Sprint(len(chunk1Data))}},
	})
	mockClient.addResponse("http://test.com/file", &http.Response{
		StatusCode: 500,
		Body:       io.NopCloser(bytes.NewReader([]byte("Internal Server Error"))),
	})

	downloadChunkClient = func() *http.Client {
		return &http.Client{Transport: mockClient}
	}

	// Call downloadURLParallel, should return an error
	err = downloadURLParallel("http://test.com/file", testFile, int64(len(chunk1Data)), 2)
	if err == nil {
		t.Fatalf("downloadURLParallel should have failed, but did not")
	}
}

func TestDownloadURLParallelInvalidNumChunks(t *testing.T) {
	mockClient := createMockClient()

	testFile, err := os.CreateTemp("", "test_file")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(testFile.Name())
	defer testFile.Close()

	mockClient.addResponse("http://test.com/file", &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader([]byte("test"))),
		Header:     http.Header{"Content-Length": {"10"}},
	})

	downloadChunkClient = func() *http.Client {
		return &http.Client{Transport: mockClient}
	}

	err = downloadURLParallel("http://test.com/file", testFile, 10, 0)
	if err == nil {
		t.Fatalf("downloadURLParallel should have failed with invalid number of chunks, but did not")
	}
}

func TestMountZip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping os tests in Windows")
	}
	// Create a dummy zip file
	zipFile := filepath.Join(t.TempDir(), "test.zip")
	createDummyZip(t, zipFile)

	mountDir := filepath.Join(t.TempDir(), "mounted_zip")

	// Mock exec.Command for testing
	// Store the original execCommand
	originalExecCommand := execCommand
	// Restore the original exec.Command after the test
	defer func() { execCommand = originalExecCommand }()

	// Replace execCommand with a mock
	execCommand = func(name string, args ...string) *exec.Cmd {
		// Check if the command is fuse-zip
		if name == "fuse-zip" {
			// Return a mock command that always succeeds
			return exec.Command("true")
		} else {
			// Return the original command for other commands
			return originalExecCommand(name, args...)
		}
	}

	originalExecLookPath := execLookPath
	defer func() { execLookPath = originalExecLookPath }()
	execLookPath = func(file string) (string, error) { return "", nil }

	err := mountZip(zipFile, mountDir)
	if err != nil {
		t.Fatalf("Failed to mount zip file: %v", err)
	}

	// Check if the mount directory was created
	_, err = os.Stat(mountDir)
	if os.IsNotExist(err) {
		t.Errorf("Mount directory not created")
	} else if err != nil {
		t.Errorf("Error checking mount directory: %v", err)
	}

}

func TestUnmountZip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping os tests in Windows")
	}
	mountDir := filepath.Join(t.TempDir(), "mounted_zip")
	err := os.Mkdir(mountDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create mount directory: %v", err)
	}

	// Mock exec.Command for testing
	// Store the original exec.Command
	originalExecCommand := execCommand
	// Restore the original exec.Command after the test
	defer func() { execCommand = originalExecCommand }()

	// Replace exec.Command with a mock
	execCommand = func(name string, args ...string) *exec.Cmd {
		// Check if the command is fusermount
		if name == "fusermount" {
			// Return a mock command that always succeeds
			return exec.Command("true")
		} else {
			// Return the original command for other commands
			return originalExecCommand(name, args...)
		}
	}

	err = unmountZip(mountDir, true)
	if err != nil {
		t.Fatalf("Failed to unmount zip file: %v", err)
	}

	// Check if the mount directory was removed
	_, err = os.Stat(mountDir)
	if !os.IsNotExist(err) {
		t.Errorf("Mount directory not removed")
	}

}

func TestMountAndUnmountZip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping os tests in Windows")
	}
	// Create a dummy zip file
	zipFile := filepath.Join(t.TempDir(), "test.zip")
	createDummyZip(t, zipFile)

	mountDir := filepath.Join(t.TempDir(), "mounted_zip")

	// Mock exec.Command for testing
	// Store the original exec.Command
	originalExecCommand := execCommand
	// Restore the original exec.Command after the test
	defer func() { execCommand = originalExecCommand }()

	// Replace exec.Command with a mock
	execCommand = func(name string, args ...string) *exec.Cmd {
		// Check if the command is fuse-zip
		if name == "fuse-zip" {
			// Return a mock command that always succeeds
			return exec.Command("true")
		} else if name == "fusermount" {
			// Return a mock command that always succeeds
			return exec.Command("true")
		} else {
			// Return the original command for other commands
			return originalExecCommand(name, args...)
		}
	}

	originalExecLookPath := execLookPath
	defer func() { execLookPath = originalExecLookPath }()
	execLookPath = func(file string) (string, error) { return "", nil }

	err := mountZip(zipFile, mountDir)
	if err != nil {
		t.Fatalf("Failed to mount zip file: %v", err)
	}

	defer func() {
		err := unmountZip(mountDir, true)
		if err != nil {
			t.Fatalf("Failed to unmount zip file: %v", err)
		}
	}()

	// Check if the mount directory was created
	_, err = os.Stat(mountDir)
	if os.IsNotExist(err) {
		t.Errorf("Mount directory not created")
	} else if err != nil {
		t.Errorf("Error checking mount directory: %v", err)
	}

	//Wait for deferred function to complete
	//Check if the mount directory was removed
}
