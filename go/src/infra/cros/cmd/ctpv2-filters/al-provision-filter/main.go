// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"strconv"
	"time"

	"cloud.google.com/go/compute/metadata"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"go.chromium.org/chromiumos/config/go/test/api"
	server "go.chromium.org/chromiumos/test/ctpv2/common/server_template"
)

const (
	binName                            = "al-provision-filter"
	androidBuildInternalScope          = "https://www.googleapis.com/auth/androidbuild.internal"
	cloudPlatformScope                 = "https://www.googleapis.com/auth/cloud-platform"
	androidBuildInternalBuildsEndpoint = "https://androidbuildinternal.googleapis.com/android/internal/build/v3/builds"
	gceServiceAccountJSONPath          = "/creds/service_accounts/service-account-chromeos.json"
)

func guessUnixHomeDir() string {
	if v := os.Getenv("HOME"); v != "" {
		return v
	}
	// Else, fall back to user.Current:
	if u, err := user.Current(); err == nil {
		return u.HomeDir
	}
	return ""
}

// fetchCredentials fetched the credentials required to access Andoif one platform apis
func fetchCredentials() (*google.Credentials, error) {
	// Read the service account JSON key file
	var serviceAccountJSONPath string
	if metadata.OnGCE() {
		serviceAccountJSONPath = gceServiceAccountJSONPath
	} else {
		serviceAccountJSONPath = guessUnixHomeDir() + ".config/gcloud/application_default_credentials.json"
	}
	jsonData, err := os.ReadFile(serviceAccountJSONPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}
	creds, err := google.CredentialsFromJSON(context.Background(), jsonData, cloudPlatformScope, androidBuildInternalScope)
	if err != nil {
		return nil, err
		// log.Printf("Error loading credentials: %s\n", err)
	}
	return creds, nil
}

// getAuthorizedHTTP creates an authorized HTTP client with OAuth2 credentials.
func getAuthorizedHTTP(credentials *oauth2.TokenSource, timeout time.Duration) (*http.Client, error) {
	client := &http.Client{
		Timeout: timeout,
		Transport: &oauth2.Transport{
			Source: *credentials,
			Base:   &http.Transport{},
		},
	}
	return client, nil
}

// formURL forms a URL with the given base URL and query parameters
func formURLForBuildAPI(board string) (string, error) {
	// Parse the base URL
	parsedURL, err := url.Parse(androidBuildInternalBuildsEndpoint)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}
	query := parsedURL.Query()

	queryParams := map[string]string{
		"buildType":   "submitted",
		"branch":      "git_main-al-dev",
		"maxResults":  "1",
		"sortingType": "creationTimestamp",
		"successful":  "true",
		"target":      board + "-trunk_staging-userdebug",
	}

	// Add query params
	for key, value := range queryParams {
		query.Set(key, value)
	}

	parsedURL.RawQuery = query.Encode()

	return parsedURL.String(), nil
}

// getResponseFromAndroidBuildAPI makes a GET call to the Android API endpoint.
func getResponseFromAndroidBuildAPI(board string, client *http.Client, log *log.Logger) (string, error) {
	requestURL, err := formURLForBuildAPI(board)
	if err != nil {
		return "", fmt.Errorf("error forming URL: %w", err)
	}
	log.Printf("Request URL: %s", requestURL)
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return "", fmt.Errorf("error creating GET request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making GET request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Error closing response body: %v", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
	}

	log.Println("Response Status:", resp.Status)
	log.Println("Response Body:", string(body))
	return string(body), nil
}

// extractBuildNumberFromResponse extracts the build number from the JSON response.
func extractBuildNumberFromResponse(resp string) (int, error) {
	var result map[string]interface{}

	// Parse the JSON string into the map
	err := json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return 0, fmt.Errorf("error parsing JSON: %w", err)
	}

	// Check if the "builds" field exists and is an array
	builds, ok := result["builds"].([]interface{})
	if !ok || len(builds) == 0 {
		return 0, fmt.Errorf("'builds' field is missing or empty")
	}

	// Fetch the first build and cast it to a map
	buildInfo, ok := builds[0].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("unable to parse build information")
	}

	// Extract the build number and assert it as a float64
	buildIDStr, ok := buildInfo["buildId"].(string)
	if !ok {
		return 0, fmt.Errorf("unable to find or parse 'buildId'")
	}

	// Convert the buildId from string to int
	buildID, err := strconv.Atoi(buildIDStr)
	if err != nil {
		return 0, fmt.Errorf("error converting 'buildId' to int: %w", err)
	}

	// Return the buildId as an int
	return buildID, nil
}

// GetLatestGreenBuildNumber calls the android one platform api to get latest build version
func GetLatestGreenBuildNumber(board string, log *log.Logger) (int, error) {
	creds, err := fetchCredentials()
	if err != nil {
		return 0, fmt.Errorf("error fetching credentials: %w", err)
	}
	httpClient, err := getAuthorizedHTTP(&creds.TokenSource, 100*time.Second)
	if err != nil {
		return 0, fmt.Errorf("failed to create authorized HTTP client: %w", err)
	}
	resp, err := getResponseFromAndroidBuildAPI(board, httpClient, log)
	if err != nil {
		return 0, fmt.Errorf("failed to get response from api: %w", err)
	}
	buildNumber, err := extractBuildNumberFromResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("failed to extract build number from response: %w", err)
	}
	return buildNumber, nil

}

func executor(req *api.InternalTestplan, log *log.Logger) (*api.InternalTestplan, error) {

	// TODO: update internal test plan accordingly and pass in board value
	latestGreenBuild, err := GetLatestGreenBuildNumber("", log)
	if err != nil {
		log.Printf("Error getting latest green build number: %v", err)
		return req, nil
	}
	log.Printf("Latest green build number: %d\n", latestGreenBuild)

	return req, nil

}

func main() {
	err := server.Server(executor, binName)
	if err != nil {
		os.Exit(2)
	}

	os.Exit(0)
}
