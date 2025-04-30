// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cache

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name     string
		inputURL url.URL
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "valid url",
			inputURL: url.URL{Scheme: "http://", Host: "localhost:8080"},
			wantErr:  false,
		},
		{
			name:     "missing host",
			inputURL: url.URL{Scheme: "http://"},
			wantErr:  true,
			errMsg:   "missing host",
		},
		{
			name:     "missing scheme",
			inputURL: url.URL{Host: "localhost:8080"},
			wantErr:  true,
			errMsg:   "missing scheme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if client, err := NewClient(tt.inputURL); (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			} else if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("NewClient() error = %q, want error containing %q", err.Error(), tt.errMsg)
			} else if !tt.wantErr && client.cacheServerURL != tt.inputURL {
				t.Errorf("NewClient() returned client with url %+v, want %+v", client.cacheServerURL.Host, tt.inputURL)
			}

		})
	}
}

func TestDownloadABArtifact(t *testing.T) {
	buildID := "12345"
	lunchTarget := "test-target"
	artifactName := "my_artifact.zip"
	expectedPath := fmt.Sprintf("/download/android-build/builds/%s/%s/attempts/latest/artifacts/%s", buildID, lunchTarget, artifactName)
	expectedContent := "this is the artifact content"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET request, got %s", r.Method)
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != expectedPath {
			t.Errorf("Expected request path %q, got %q", expectedPath, r.URL.Path)
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		fmt.Fprint(w, expectedContent)
	}))
	defer server.Close()

	serverURL, _ := url.Parse(server.URL)
	client, err := NewClient(*serverURL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	filePath, _, err := client.DownloadABArtifact(buildID, lunchTarget, artifactName)
	if err != nil {
		t.Fatalf("DownloadABArtifact failed: %v", err)
	}
	defer os.Remove(filePath) // Clean up the temp file

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read downloaded file %q: %v", filePath, err)
	}
	if string(content) != expectedContent {
		t.Errorf("Downloaded content mismatch: got %q, want %q", string(content), expectedContent)
	}
}

func TestDownloadABArtifact_ServerError(t *testing.T) {
	buildID := "12345"
	lunchTarget := "test-target"
	artifactName := "my_artifact.zip"
	expectedPath := fmt.Sprintf("/download/android-build/builds/%s/%s/attempts/latest/artifacts/%s", buildID, lunchTarget, artifactName)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == expectedPath {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		} else {
			http.Error(w, "Not Found", http.StatusNotFound)
		}
	}))
	defer server.Close()

	serverURL, _ := url.Parse(server.URL)
	client, err := NewClient(*serverURL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	_, _, err = client.DownloadABArtifact(buildID, lunchTarget, artifactName)
	if err == nil {
		t.Fatal("DownloadABArtifact expected an error, but got nil")
	}
	if !strings.Contains(err.Error(), "error 500 Internal Server Error") {
		t.Errorf("Expected error containing '500 Internal Server Error', got: %v", err)
	}
}

func TestDownloadABArtifact_ClientError(t *testing.T) {
	// Use a non-existent server URL to simulate client error
	invalidURL, _ := url.Parse("http://invalid-host-that-does-not-exist:12345")
	client, err := NewClient(*invalidURL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	buildID := "12345"
	lunchTarget := "test-target"
	artifactName := "my_artifact.zip"

	_, _, err = client.DownloadABArtifact(buildID, lunchTarget, artifactName)
	if err == nil {
		t.Fatal("DownloadABArtifact expected an error, but got nil")
	}
	if !strings.Contains(err.Error(), "failed sending GET request") {
		t.Errorf("Expected error containing 'failed sending GET request', got: %v", err)
	}
}
