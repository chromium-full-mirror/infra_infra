// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	"go.chromium.org/chromiumos/config/go/test/api"
	server "go.chromium.org/chromiumos/test/ctpv2/common/server_template"
	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/hardcoded/chromeinfra"
)

const (
	binName                      = "al-provision-filter"
	androidBuildInternalScope    = "https://www.googleapis.com/auth/androidbuild.internal"
	cloudPlatformScope           = "https://www.googleapis.com/auth/cloud-platform"
	androidBuildInternalEndpoint = "https://androidbuildinternal.googleapis.com/android/internal/build/v3/buildIds/"
	gitMainAlDev                 = "git_main-al-dev/"
)

// GetHTTPClient returns a HTTP client ready to interact with androidbuildinternal apis
func GetHTTPClient() (*http.Client, error) {
	opts := chromeinfra.DefaultAuthOptions()
	// Introduce gerrit scopes for accessing the config files
	gerritScopes := []string{
		androidBuildInternalScope,
		cloudPlatformScope,
	}
	opts.Scopes = append(opts.Scopes, gerritScopes...)

	authenticator := auth.NewAuthenticator(context.Background(), auth.SilentLogin, opts)
	httpClient, err := authenticator.Client()
	if err != nil {
		return nil, err
	}
	return httpClient, err
}

// formURL forms a URL with the given base URL and query parameters
func formURL(baseURL string, branch string, params map[string]string) (string, error) {
	// Parse the base URL
	parsedURL, err := url.Parse(baseURL + branch)
	if err != nil {
		return "", err
	}
	query := parsedURL.Query()

	// Add query params
	for key, value := range params {
		query.Set(key, value)
	}

	parsedURL.RawQuery = query.Encode()

	return parsedURL.String(), nil
}

// MakeGetCall makes a GET call to the API endpoint.
func MakeGetCall(client *http.Client) {
	params := map[string]string{
		"buildType": "submitted",
	}

	requestURL, err := formURL(androidBuildInternalEndpoint, gitMainAlDev, params)
	if err != nil {
		log.Fatalf("Error forming URL: %v", err)
	}
	log.Printf("Request URL for AndroidBuildInternal: %s", requestURL)
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		log.Fatalf("Error creating request: %v", err)
	}
	// req.Header.Set("x-goog-user-project", "chromeos-bot")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Error making request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error reading response body: %v", err)
	}

	log.Println("Response Status:", resp.Status)
	log.Println("Response Body:", string(body))
}

// FetchBuildsNumber returns the build number for the required target
func FetchBuildsNumber() {
	client, err := GetHTTPClient()
	if err != nil {
		log.Fatalf("Error getting HTTP client: %v", err)
	}
	MakeGetCall(client)

}

func executor(req *api.InternalTestplan, log *log.Logger) (*api.InternalTestplan, error) {
	FetchBuildsNumber()
	return req, nil
}

func main() {
	err := server.Server(executor, binName)
	if err != nil {
		os.Exit(2)
	}

	os.Exit(0)
}
