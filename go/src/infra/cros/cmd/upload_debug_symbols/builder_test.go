// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
//go:build !windows
// +build !windows

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"debug/elf"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.chromium.org/infra/cros/internal/gs"
)

type mockClient struct {
	expectedResponses map[string]string
}

func (c *mockClient) Do(req *http.Request) (*http.Response, error) {
	// if the request matches a string in the expected responses then generate a
	// response and return it. If it isn't apart of the map then return an error
	// and a nil response.
	if val, ok := c.expectedResponses[req.URL.String()]; ok {
		response := http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(val))}
		return &response, nil
	} else {
		return nil, fmt.Errorf("error: %s is not an expected url", req.URL.String())
	}
}

func initCrashConnectionMock(mockURL, mockKey string, responseMap map[string]string) crashConnectionInfo {
	return crashConnectionInfo{
		url:    mockURL,
		key:    mockKey,
		client: &mockClient{expectedResponses: responseMap},
	}
}

func buildFakeELFWithNote(buildID string) []byte {
	var buf bytes.Buffer

	decoded, err := hex.DecodeString(buildID)
	if err != nil {
		return nil
	}

	idLen := len(decoded)
	sectionHeaderOffset := 52 + 1 + 10 + 19 + 16 + idLen

	buf.Grow(sectionHeaderOffset + 2*40)

	h := elf.Header32{
		Ident:     [16]byte{0x7F, 'E', 'L', 'F', 0x1, 0x1, 0x1},
		Type:      1,
		Machine:   3,
		Version:   1,
		Shoff:     uint32(sectionHeaderOffset),
		Ehsize:    0x34,
		Shentsize: 0x28,
		Shnum:     3,
		Shstrndx:  1,
	}
	binary.Write(&buf, binary.LittleEndian, h)

	buf.Write([]byte{0})
	buf.Write([]byte(".shstrtab\x00"))
	buf.Write([]byte(".note.gnu.build-id\x00"))

	noteStart := buf.Len()
	binary.Write(&buf, binary.LittleEndian, []uint32{0x4, uint32(idLen), 3})
	buf.Write([]byte("GNU\x00"))
	buf.Write(decoded)
	noteEnd := buf.Len()

	binary.Write(&buf, binary.LittleEndian, elf.Section32{Name: 0})

	binary.Write(&buf, binary.LittleEndian, elf.Section32{
		Name:      1,
		Type:      uint32(elf.SHT_STRTAB),
		Off:       52,
		Size:      30,
		Addralign: 0x1,
	})

	binary.Write(&buf, binary.LittleEndian, elf.Section32{
		Name:      0xb,
		Type:      uint32(elf.SHT_NOTE),
		Flags:     uint32(elf.SHF_ALLOC),
		Addr:      uint32(noteStart),
		Off:       uint32(noteStart),
		Addralign: 0x4,
		Size:      uint32(noteEnd - noteStart),
	})

	return buf.Bytes()
}

// TestGetBuildId ensures that we can extract a build ID from a splitdebug
// file.
func TestGetBuildId(t *testing.T) {
	const expected = "BFCF6FA6CCBDEF00501810DE869C8A2F40ABC321"

	testDir, err := os.MkdirTemp("", "getBuildIdTest")
	if err != nil {
		t.Error("error: " + err.Error())
	}
	defer os.RemoveAll(testDir)

	mockPath := filepath.Join(testDir, "fake.debug")
	err = os.WriteFile(mockPath, buildFakeELFWithNote(expected), 0644)
	if err != nil {
		t.Error("error: " + err.Error())
	}

	actual, err := getBuildId(mockPath)
	if err != nil {
		t.Error("error: " + err.Error())
	}

	if actual != expected {
		t.Error(actual + " doesn't match expected ID " + expected)
	}
}

// TestDownloadZippedSymbols ensures that we are fetching from the correct service and
// handling the response appropriately.
func TestDownloadZippedSymbols(t *testing.T) {
	gsPath := "gs://some-debug-symbol/degbug.tgz"

	expectedDownloads := map[string][]byte{
		gsPath: []byte("hello world"),
	}
	fakeClient := &gs.FakeClient{
		T:                 t,
		ExpectedDownloads: expectedDownloads,
	}

	tarballDir, err := os.MkdirTemp("", "tarball")
	if err != nil {
		t.Error("error: " + err.Error())
	}
	defer os.RemoveAll(tarballDir)

	tgzPath, err := downloadZippedSymbols(fakeClient, gsPath, tarballDir)

	if err != nil {
		t.Error("error: " + err.Error())
	}

	if _, err := os.Stat(tgzPath); os.IsNotExist(err) {
		t.Error("error: " + err.Error())
	}
}

// TestUnzipTgz confirms that we can properly unzip a given tgz file.
func TestUnzipSymbols(t *testing.T) {

	targetString := "gzip test"

	// Create temp dir to work in.
	testDir, err := os.MkdirTemp("", "tarballTest")
	if err != nil {
		t.Error("error: " + err.Error())
	}
	defer os.RemoveAll(testDir)

	// Generate file information.
	outputFilePath := filepath.Join(testDir, "test.tar")
	inputFilePath := filepath.Join(testDir, "test.tgz")
	inputFile, err := os.Create(inputFilePath)
	if err != nil {
		t.Error("error: " + err.Error())
	}
	defer inputFile.Close()

	// Create a mock .tgz to test unzipping.
	zipWriter := gzip.NewWriter(inputFile)
	zipWriter.Name = "test.tgz"
	zipWriter.Comment = "hello world"

	_, err = zipWriter.Write([]byte(targetString))
	if err != nil {
		t.Error("error: " + err.Error())
	}

	err = zipWriter.Close()
	if err != nil {
		t.Error("error: " + err.Error())
	}

	err = unzipSymbols(inputFilePath, outputFilePath)
	if err != nil {
		t.Error("error: " + err.Error())
	}

	// Check if the output file was created.
	if _, err := os.Stat(outputFilePath); os.IsNotExist(err) {
		t.Error("error: " + err.Error())
	}

	outputFile, err := os.Open(outputFilePath)
	if err != nil {
		t.Error("error: " + err.Error())
	}
	defer outputFile.Close()

	output, err := io.ReadAll(outputFile)
	if err != nil {
		t.Error("error: " + err.Error())
	}

	// Verify contents unzipped correctly.
	if string(output) != targetString {
		t.Errorf("error: expected %s got %s", targetString, string(output))
	}

}

// TestUnpackTarball confirms that we can properly unpack a given tarball and
// return filepaths to it's contents. Basic testing pulled from
// https://pkg.go.dev/archive/tar#pkg-overview.
func TestUnpackTarball(t *testing.T) {
	// Create working directory and tarball.
	testDir, err := os.MkdirTemp("", "tarballTest")
	if err != nil {
		t.Error("error: " + err.Error())
	}
	debugSymbolsDir, err := os.MkdirTemp(testDir, "symbols")
	if err != nil {
		t.Error("error: " + err.Error())
	}
	defer os.RemoveAll(testDir)

	// Generate file information.
	tarPath := filepath.Join(testDir, "test.tar")
	inputFile, err := os.Create(tarPath)
	if err != nil {
		t.Error("error: " + err.Error())
	}

	tarWriter := tar.NewWriter(inputFile)
	// Struct for file info
	type file struct {
		name, body string
		modeType   fs.FileMode
	}

	// Create an array holding some basic info to build headers. Contains regular
	// files and directories.
	files := []file{
		{"/test1.so.sym", "MODULE Linux arm F4F6FA6CCBDEF455039C8DE869C8A2F40 blkid", 0600},
		{"./test2.so.sym", "MODULE Linux arm F4F6FA6CCBDEF455039C8DE869C8A2F41 blkid", 0600},
		{"b/c", "", fs.ModeDir},
		// Different Debug ID as other test1.so.sym so should be included.
		{"b/c/test1.so.sym", "MODULE Linux arm F4F6FA6CCBDEF455039C8DE869C8A2F42 blkid", 0600},
		{"../test3.so.sym", "MODULE Linux arm F4F6FA6CCBDEF455039C8DE869C8A2F43 blkid", 0600},
		{"a/b/c/d/", "", fs.ModeDir},
		{"./test4.so.sym", "MODULE Linux arm F4F6FA6CCBDEF455039C8DE869C8A2F44 blkid", 0600},
		// Same Debug ID as previous file so should not be included.
		{"a/b/c/d/test4.so.sym", "MODULE Linux arm F4F6FA6CCBDEF455039C8DE869C8A2F44 blkid", 0600},
		{"a/shouldntadd.txt", "not a symbol file", 0600},
	}

	// List of files we expect to see return after the test call.
	expectedSymbolFiles := map[string]bool{
		debugSymbolsDir + "/test1.so.sym":   false,
		debugSymbolsDir + "/test1.so.sym-1": false,
		debugSymbolsDir + "/test2.so.sym":   false,
		debugSymbolsDir + "/test3.so.sym":   false,
		debugSymbolsDir + "/test4.so.sym":   false,
	}

	// Write the mock files to the tarball.
	for _, file := range files {
		hdr := &tar.Header{
			Name: file.name,
			Mode: int64(file.modeType),
			Size: int64(len(file.body)),
		}
		if err := tarWriter.WriteHeader(hdr); err != nil {
			t.Error("error: " + err.Error())
		}

		if file.modeType == 0600 {
			if _, err := tarWriter.Write([]byte(file.body)); err != nil {
				t.Error("error: " + err.Error())
			}
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Error("error: " + err.Error())
	}

	// Call the Function
	symbolPaths, err := unpackTarball(tarPath, debugSymbolsDir)
	if err != nil {
		t.Error("error: " + err.Error())
	}
	if len(symbolPaths) == 0 {
		t.Error("error: Empty list of paths returned")
	}
	// Verify that we received a list pointing to all the expected files and no
	// others.
	for _, path := range symbolPaths {
		if val, ok := expectedSymbolFiles[path]; ok {
			if val {
				t.Error("error: symbol file appeared multiple times in function return")
			}
			expectedSymbolFiles[path] = true
		} else {
			t.Errorf("error: unexpected symbol file returned %s", path)
		}
	}
}

// TestUnpackSplitdebugTarballs confirms that we can properly unpack a given tarball and
// return filepaths to it's contents. Basic testing pulled from
// https://pkg.go.dev/archive/tar#pkg-overview.
func TestUnpackSplitdebugTarballs(t *testing.T) {
	// Create working directory and tarball.
	testDir, err := os.MkdirTemp("", "splitdebugTarballTest")
	if err != nil {
		t.Error("error: " + err.Error())
	}
	debugSymbolsDir, err := os.MkdirTemp(testDir, "symbols")
	if err != nil {
		t.Error("error: " + err.Error())
	}
	defer os.RemoveAll(testDir)

	// Struct for symbolFile info
	type symbolFile struct {
		name, body string
		modeType   fs.FileMode
	}

	// The debug Id is a byte swapped and truncated version of the build Id.
	const buildId1 = "F4F6FA6CCBDEF455039C8DE869C8A2F4AB"
	const debugId1 = "6CFAF6F4DECB55F4039C8DE869C8A2F40"
	const buildId1Dup = "F4F6FA6CCBDEF455039C8DE869C8A2F6AB"
	const debugId1Dup = "6CFAF6F4DECB55F4039C8DE869C8A2F60"
	const buildId2 = "F4F6FA6CCBDEF455039C8DE869C8A2F5AC"
	const debugId2 = "6CFAF6F4DECB55F4039C8DE869C8A2F50"
	const buildId3 = "F4F6FA6CCBDEF455039C8DE869C8A2F7AD"
	const debugId3 = "6CFAF6F4DECB55F4039C8DE869C8A2F70"
	const buildId4 = "F4F6FA6CCBDEF455039C8DE869C8A2F8AE"
	const debugId4 = "6CFAF6F4DECB55F4039C8DE869C8A2F80"

	// Create an array holding some basic info to build headers. Contains regular
	// files and directories.
	symbolFiles := []symbolFile{
		{"/test1.so.sym", "MODULE Linux arm " + debugId1 + " test1.so\nINFO CODE_ID " + buildId1, 0600},
		{"./test2.so.sym", "MODULE Linux arm " + debugId2 + " test2.so\nINFO CODE_ID " + buildId2, 0600},
		{"b/c", "", fs.ModeDir},
		// Different Debug ID as other test1.so.sym so should be included.
		{"b/c/test1.so.sym", "MODULE Linux arm " + debugId1Dup + " test1.so\nINFO CODE_ID " + buildId1Dup, 0600},
		{"../test3.so.1.sym", "MODULE Linux arm " + debugId3 + " test3.so.1\nINFO CODE_ID " + buildId3, 0600},
		{"a/b/c/d/", "", fs.ModeDir},
		{"./test4.so.sym", "MODULE Linux arm " + debugId4 + " test4.so\nINFO CODE_ID " + buildId4, 0600},
		// Same Debug ID as previous file so should not be included.
		{"a/b/c/d/test4.so.sym", "MODULE Linux arm " + debugId4 + " test4.so\nINFO CODE_ID " + buildId4, 0600},
		{"a/shouldntadd.txt", "not a symbol file", 0600},
	}

	// List of files we expect to see return after the test call.
	expectedSymbolFiles := map[string]bool{
		debugSymbolsDir + "/" + debugId1 + "/test1.so.sym":    false,
		debugSymbolsDir + "/" + debugId1Dup + "/test1.so.sym": false,
		debugSymbolsDir + "/" + debugId2 + "/test2.so.sym":    false,
		debugSymbolsDir + "/" + debugId3 + "/test3.so.1.sym":  false,
		debugSymbolsDir + "/" + debugId4 + "/test4.so.sym":    false,
	}

	// Struct for file info
	type splitdebugFile struct {
		name     string
		body     []byte
		modeType fs.FileMode
	}
	// Create an array holding some basic info to build headers. Contains regular
	// files and directories.
	splitdebugFiles := []splitdebugFile{
		{"/test1.so.debug", buildFakeELFWithNote(buildId1), 0600},
		{"./test2.so.debug", buildFakeELFWithNote(buildId2), 0600},
		{"b/c", nil, fs.ModeDir},
		// Different Debug ID as other test1.so.debug so should be included.
		{"b/c/test1.so.debug", buildFakeELFWithNote(buildId1Dup), 0600},
		// Different name as breakpad symbol (test3.so.1.sym) should be included
		{"../test3.so.1.0.5.debug", buildFakeELFWithNote(buildId3), 0600},
		{"a/b/c/d/", nil, fs.ModeDir},
		{"./test4.so.debug", buildFakeELFWithNote(buildId4), 0600},
		// Same Debug ID as previous file so should not be included.
		{"a/b/c/d/test4.so.debug", buildFakeELFWithNote(buildId4), 0600},
		{"a/shouldntadd.txt", []byte("not a symbol file\x00"), 0600},
	}

	// Generate file information.
	splitdebugTarPath := filepath.Join(testDir, "test-debug.tar")
	splitdebugInputFile, err := os.Create(splitdebugTarPath)
	if err != nil {
		t.Error("error: " + err.Error())
	}

	splitdebugTarWriter := tar.NewWriter(splitdebugInputFile)

	// Write the mock files to the tarball.
	for _, file := range splitdebugFiles {
		hdr := &tar.Header{
			Name: file.name,
			Mode: int64(file.modeType),
			Size: int64(len(file.body)),
		}
		if err := splitdebugTarWriter.WriteHeader(hdr); err != nil {
			t.Error("error: " + err.Error())
		}

		if file.modeType == 0600 {
			if _, err := splitdebugTarWriter.Write(file.body); err != nil {
				t.Error("error: " + err.Error())
			}
		}
	}
	if err := splitdebugTarWriter.Close(); err != nil {
		t.Error("error: " + err.Error())
	}

	symbolTarPath := filepath.Join(testDir, "test-symbols.tar")
	symbolInputFile, err := os.Create(symbolTarPath)
	if err != nil {
		t.Error("error: " + err.Error())
	}

	symbolTarWriter := tar.NewWriter(symbolInputFile)

	// Write the mock files to the tarball.
	for _, file := range symbolFiles {
		hdr := &tar.Header{
			Name: file.name,
			Mode: int64(file.modeType),
			Size: int64(len(file.body)),
		}
		if err := symbolTarWriter.WriteHeader(hdr); err != nil {
			t.Error("error: " + err.Error())
		}

		if file.modeType == 0600 {
			if _, err := symbolTarWriter.Write([]byte(file.body)); err != nil {
				t.Error("error: " + err.Error())
			}
		}
	}
	if err := symbolTarWriter.Close(); err != nil {
		t.Error("error: " + err.Error())
	}

	// Call the function under test.
	breakpadPaths, err := unpackSplitdebugTarballs(splitdebugTarPath, symbolTarPath, debugSymbolsDir)
	if err != nil {
		t.Error("error: " + err.Error())
	}
	if len(breakpadPaths) == 0 {
		t.Error("error: Empty list of breakpad paths returned")
	}
	// Verify that we received a list pointing to all the expected files and no
	// others.
	for _, path := range breakpadPaths {
		if val, ok := expectedSymbolFiles[path]; ok {
			if val {
				t.Errorf("error: breakpad file %q appeared multiple times in function return", path)
			}
			// Verify that the breakpad file has an associated splitdebug file.
			base, _ := strings.CutSuffix(path, ".sym")
			debugFile := base
			if _, err := os.Stat(debugFile); err != nil {
				t.Errorf("error: breakpad file %q missing splitdebug file %q", path, debugFile)
			}

			expectedSymbolFiles[path] = true
		} else {
			t.Errorf("error: unexpected breakpad file returned %q", path)
		}
	}
	for path, ok := range expectedSymbolFiles {
		if !ok {
			t.Errorf("error: breakpad file %q not found", path)
		}
	}
}

// TestGenerateConfigs validates that proper task configs are generated when a
// list of filepaths are given.
func TestGenerateConfigs(t *testing.T) {
	// Init the mock files and verifying structures.
	expectedTasks := map[taskConfig]bool{}
	type responseInfo struct {
		filename string
		symbol   string
		status   string
		// Local path to write the file to. Used for dupe debug symbols
		// (mimicking behavior in unpackTarball)
		localPath string
	}
	mockResponses := []*responseInfo{
		{
			filename: "test1.so.sym",
			symbol:   "F4F6FA6CCBDEF455039C8DE869C8A2F40",
			status:   "FOUND",
		}, {
			filename:  "test1.so.sym",
			symbol:    "F4F6FA6CCBDEF455039C8DE869C8A2F41",
			status:    "MISSING",
			localPath: "test1.so.sym-1",
		}, {
			filename: "test2.so.sym",
			symbol:   "F4F6FA6CCBDEF455039C8DE869C8A2F40",
			status:   "FOUND",
		}, {
			filename: "test3.so.sym",
			symbol:   "F4F6FA6CCBDEF455039C8DE869C8A2F40",
			status:   "Missing",
		},
		{
			filename: "test4.so.sym",
			symbol:   "F4F6FA6CCBDEF455039C8DE869C8A2F40",
			status:   "MISSING",
		},
		{
			filename: "test5.so.sym",
			symbol:   "F4F6FA6CCBDEF455039C8DE869C8A2F40",
			status:   "STATUS_UNSPECIFIED",
		},
		{
			filename: "test6.so.sym",
			symbol:   "F4F6FA6CCBDEF455039C8DE869C8A2F40",
			status:   "STATUS_UNSPECIFIED",
		},
	}

	// Make the expected request body.
	responseBody := filterResponseBody{Pairs: []filterResponseStatusPair{}}

	// Test for all 3 cases found in http://google3/net/crash/symbolcollector/symbol_collector.proto?l=19
	for _, response := range mockResponses {
		symbol := filterSymbolFileInfo{response.filename, response.symbol}
		responseBody.Pairs = append(responseBody.Pairs, filterResponseStatusPair{SymbolId: symbol, Status: response.status})
	}
	mockResponseBody, err := json.Marshal(responseBody)
	if err != nil {
		t.Error("error: " + err.Error())
	}

	// Mock the symbol files locally.
	testDir, err := os.MkdirTemp("", "configGenTest")
	if err != nil {
		t.Error("error: " + err.Error())
	}
	defer os.RemoveAll(testDir)

	mockPaths := []string{}
	for _, response := range mockResponses {
		localPath := response.filename
		if response.localPath != "" {
			localPath = response.localPath
		}
		mockPath := filepath.Join(testDir, localPath)
		err = os.WriteFile(mockPath, []byte(fmt.Sprintf("MODULE Linux arm %s %s", response.symbol, filepath.Base(response.filename))), 0644)
		if err != nil {
			t.Error("error: " + err.Error())
		}
		task := taskConfig{mockPath, "BREAKPAD", response.filename, response.symbol, false, false}

		if response.status != "FOUND" {
			expectedTasks[task] = false
		}
		mockPaths = append(mockPaths, mockPath)
	}

	// Init global variables and
	mockCrash := initCrashConnectionMock("google.com", "1234", map[string]string{"google.com/symbols:checkStatuses?key=1234": string(mockResponseBody)})

	tasks, err := generateConfigs(context.Background(), mockPaths, 0, false, mockCrash)
	if err != nil {
		t.Error("error: " + err.Error())
	}
	// Check that returns aren't nil.
	if tasks == nil {
		t.Error("error: recieved tasks when nil was expected")
	}

	// Verify that we received a list pointing to all the expected files and no
	// others.
	for _, task := range tasks {
		if val, ok := expectedTasks[task]; ok {
			if val {
				t.Errorf("error: task %v appeared multiple times in function return", task)
			}
			expectedTasks[task] = true
		} else {
			t.Errorf("error: unexpected task returned %+v", task)
		}
	}

	for task, value := range expectedTasks {
		if value == false {
			t.Errorf("error: task for file %s never seen", task.debugFile)
		}
	}
}

// TestGenerateSplitdebugConfigs validates that proper task configs are generated when a
// list of filepaths are given.
func TestGenerateSplitdebugConfigs(t *testing.T) {
	// Init the mock files and verifying structures.
	expectedTasks := map[taskConfig]bool{}
	type responseInfo struct {
		filename       string
		symbol         string
		status         string
		breakpadPath   string
		splitdebugPath string
	}
	mockResponses := []*responseInfo{
		{
			filename:       "test1.so",
			symbol:         "F4F6FA6CCBDEF455039C8DE869C8A2F40",
			status:         "FOUND",
			breakpadPath:   "test1.so.sym",
			splitdebugPath: "test1.so",
		}, {
			filename:       "test2.so",
			symbol:         "F4F6FA6CCBDEF455039C8DE869C8A2F40",
			status:         "FOUND",
			breakpadPath:   "test2.so.sym",
			splitdebugPath: "test2.so",
		}, {
			filename:       "test3.so",
			symbol:         "F4F6FA6CCBDEF455039C8DE869C8A2F40",
			status:         "Missing",
			breakpadPath:   "test3.so.sym",
			splitdebugPath: "test3.so",
		},
		{
			filename:       "test4.so.1",
			symbol:         "F4F6FA6CCBDEF455039C8DE869C8A2F40",
			status:         "MISSING",
			breakpadPath:   "test4.so.1.sym",
			splitdebugPath: "test4.so.1",
		},
		{
			filename:       "test5.so",
			symbol:         "F4F6FA6CCBDEF455039C8DE869C8A2F40",
			status:         "STATUS_UNSPECIFIED",
			breakpadPath:   "test5.so.sym",
			splitdebugPath: "test5.so",
		},
		{
			filename:       "test6.so",
			symbol:         "F4F6FA6CCBDEF455039C8DE869C8A2F40",
			status:         "STATUS_UNSPECIFIED",
			breakpadPath:   "test6.so.sym",
			splitdebugPath: "test6.so",
		},
		{
			filename:     "test7.so",
			symbol:       "F4F6FA6CCBDEF455039C8DE869C8A2F40",
			status:       "STATUS_UNSPECIFIED",
			breakpadPath: "test7.so.sym",
			// No splitdebugPath, no expected splitdebug upload task
		},
	}

	// Make the expected request body.
	responseBody := filterResponseBody{Pairs: []filterResponseStatusPair{}}

	// Test for all 3 cases found in http://google3/net/crash/symbolcollector/symbol_collector.proto?l=19
	for _, response := range mockResponses {
		symbol := filterSymbolFileInfo{response.filename, response.symbol}
		responseBody.Pairs = append(responseBody.Pairs, filterResponseStatusPair{SymbolId: symbol, Status: response.status})
	}
	mockResponseBody, err := json.Marshal(responseBody)
	if err != nil {
		t.Error("error: " + err.Error())
	}

	// Mock the symbol files locally.
	testDir, err := os.MkdirTemp("", "configGenTest")
	if err != nil {
		t.Error("error: " + err.Error())
	}
	defer os.RemoveAll(testDir)

	mockPaths := []string{}
	for _, response := range mockResponses {
		// Make a mock directory with the debug Id.
		localDir := filepath.Join(testDir, response.symbol)
		err := os.Mkdir(localDir, 0750)
		if err != nil && !os.IsExist(err) {
			t.Error("error: " + err.Error())
		}

		mockSplitdebugPath := filepath.Join(localDir, response.splitdebugPath)
		// Create a mock splitdebug file.
		if response.splitdebugPath != "" {
			err = os.WriteFile(mockSplitdebugPath, buildFakeELFWithNote(response.symbol), 0644)
			if err != nil {
				t.Error("error: " + err.Error())
			}
		}
		// Create a mock breakpad file.
		mockPath := filepath.Join(localDir, response.breakpadPath)
		err = os.WriteFile(mockPath, []byte(fmt.Sprintf("MODULE Linux arm %s %s", response.symbol, response.filename)), 0644)
		if err != nil {
			t.Error("error: " + err.Error())
		}
		// Generate two tasks.
		task := taskConfig{mockPath, "BREAKPAD", response.filename, response.symbol, false, false}
		splitdebugTask := taskConfig{mockSplitdebugPath, "ELF", response.filename, response.symbol, false, false}

		if response.status != "FOUND" {
			expectedTasks[task] = false
			if response.splitdebugPath != "" {
				expectedTasks[splitdebugTask] = false
			}
		}
		mockPaths = append(mockPaths, mockPath)
	}

	// Init global variables and
	mockCrash := initCrashConnectionMock("google.com", "1234", map[string]string{"google.com/symbols:checkStatuses?key=1234": string(mockResponseBody)})

	tasks, err := generateSplitdebugConfigs(context.Background(), mockPaths, 0, false, mockCrash)
	if err != nil {
		t.Error("error: " + err.Error())
	}
	// Check that returns aren't nil.
	if tasks == nil {
		t.Error("error: recieved tasks when nil was expected")
	}

	// Verify that we received a list pointing to all the expected files and no
	// others.
	for _, task := range tasks {
		if val, ok := expectedTasks[task]; ok {
			if val {
				t.Errorf("error: task %v appeared multiple times in function return", task)
			}
			expectedTasks[task] = true
		} else {
			t.Errorf("error: unexpected task returned %+v", task)
		}
	}

	for task, value := range expectedTasks {
		if value == false {
			t.Errorf("error: task %+v never seen", task)
		}
	}
}

// TestUploadSymbols affirms that the worker design and retry model are valid.
func TestUploadSymbols(t *testing.T) {
	// Create tasks and expected returns.
	tasks := []taskConfig{
		{"", "BREAKPAD", "test1.so.sym", "", false, false},
		{"", "BREAKPAD", "test2.so.sym", "", false, false},
		{"", "BREAKPAD", "test3.so.sym", "", false, false},
		{"", "BREAKPAD", "test4.so.sym", "", false, false},
	}

	// Mock the symbol files locally.
	testDir, err := os.MkdirTemp("", "uploadSymbolsTest")
	if err != nil {
		t.Error("error: " + err.Error())
	}
	defer os.RemoveAll(testDir)

	// Write mock files locally.
	for index, task := range tasks {
		mockPath := filepath.Join(testDir, task.debugFile)
		err = os.WriteFile(mockPath, []byte("MODULE Linux arm F4F6FA6CCBDEF455039C8DE869C8A2F40 blkid"), 0644)
		if err != nil {
			t.Error("error: " + err.Error())
		}
		tasks[index].symbolPath = mockPath
	}

	// unique URL and key
	mockURLKeyPair := crashUploadInformation{UploadUrl: "crashupload.com", UploadKey: "abc"}
	mockCompleteResponse := crashSubmitResposne{Result: "OK"}
	mockURLKeyPairJSON, err := json.Marshal(mockURLKeyPair)
	if err != nil {
		t.Error("error: could not marshal response")
	}
	mockCompleteResponseJSON, err := json.Marshal(mockCompleteResponse)
	if err != nil {
		t.Error("error: could not marshal response")
	}

	expectedResponses := map[string]string{
		"google.com/uploads:create?key=1234":       string(mockURLKeyPairJSON),
		"crashupload.com":                          "",
		"google.com/uploads/abc:complete?key=1234": string(mockCompleteResponseJSON),
	}

	crashMock := initCrashConnectionMock("google.com", "1234", expectedResponses)
	retcode, err := uploadSymbols(tasks, 64, 2, false, crashMock)

	if err != nil {
		t.Error("error: " + err.Error())
	}

	if retcode != 0 {
		t.Errorf("error: recieved non-zero retcode %d", retcode)
	}
}

// TestCleanErrorMessage tests to make sure we won't accidentally expose the api
// key.
func TestCleanErrorMessage(t *testing.T) {
	mockKey := "abcde12345!@#$"
	mockErr := fmt.Errorf("could not connect to www.google.com/secret?key=%s&v1=1&v2=test", mockKey)

	cleanedErrMessage := cleanErrorMessage(mockErr.Error(), mockKey)

	if strings.Contains(cleanedErrMessage, mockKey) {
		t.Error("error: secret key found in error response")
	}

	if !strings.Contains(cleanedErrMessage, "-HIDDEN-KEY-") {
		t.Error("error: replaced key not found")
	}

	mockErr = fmt.Errorf("nil http repsonse was recieved")
	cleanedErrMessage = cleanErrorMessage(mockErr.Error(), mockKey)

	if strings.Contains(cleanedErrMessage, "-HIDDEN-KEY-") {
		t.Error("error: non-existant key was fouind in error")
	}
}
