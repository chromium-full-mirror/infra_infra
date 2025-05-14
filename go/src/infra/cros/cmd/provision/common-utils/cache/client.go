// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cache enables interaction with the cache server.
package cache

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"

	conf "go.chromium.org/chromiumos/config/go"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

type Client struct {
	cacheServerURL url.URL
}

// Client constructs a client to query the cache server.
func NewClient(serverURL url.URL) (*Client, error) {
	if serverURL.Host == "" {
		return &Client{}, fmt.Errorf("cache server url missing host: %+v", serverURL)
	}
	if serverURL.Scheme == "" {
		return &Client{}, fmt.Errorf("cache server url missing scheme: %+v", serverURL)
	}
	return &Client{cacheServerURL: serverURL}, nil
}

// DownloadABArtifact returns an Android Build artifact from the cache server.
func (client *Client) DownloadABArtifactByStoragePath(sp *conf.StoragePath) (string, error) {
	if sp.GetHostType() != conf.StoragePath_ANDROID_BUILD {
		return "", fmt.Errorf("storage path %+v had unexpected host type %v", sp, sp.GetHostType())
	}
	buildID, buildTarget, artifactName, err := common.ParseAndroidPath(sp.GetPath())
	if err != nil {
		return "", fmt.Errorf("parsing android storage path: %w", err)
	}
	localPath, _, err := client.DownloadABArtifact(buildID, buildTarget, artifactName)
	if err != nil {
		return "", fmt.Errorf("downloading android build artifact %+v: %w", sp, err)
	}
	return localPath, nil
}

// DownloadABArtifact returns an Android Build artifact from the cache server.
func (client *Client) DownloadABArtifact(buildID, buildTarget, artifactName string) (localPath, errorReason string, err error) {
	downloadURL := client.cacheServerURL
	downloadURL.Path = path.Join("download", "android-build", "builds", buildID, buildTarget, "attempts", "latest", "artifacts", artifactName)
	log.Println("Downloading Android Build artifact from cache server: ", downloadURL.String())

	httpClient := http.Client{}
	resp, err := httpClient.Get(downloadURL.String())
	if err != nil {
		return "", "FLEET: failed to pull from cache server", fmt.Errorf("failed sending GET request to download Android artifact from cache server: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Sprintf("FLEET: error code while downloading: %s", resp.Status), fmt.Errorf("error %s while downloading Android artifact from %s", resp.Status, downloadURL.String())
	}
	out, err := os.CreateTemp(os.TempDir(), "cached-*")
	if err != nil {
		return "", "INFRA: unable to create tempfile on host", fmt.Errorf("failed to create a temp file on host: %w", err)
	}
	defer out.Close()
	log.Printf("Created tempfile %s to store the pulled file", out.Name())
	if _, err = io.Copy(out, resp.Body); err != nil {
		return "", "INFRA: unable to copy cache to tempfile", fmt.Errorf("failed to copy request data to tempfile: %w", err)
	}
	return out.Name(), "", nil
}
