// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package androidapi

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
)

const (
	buildsEndpoint = "https://androidbuildinternal.googleapis.com/android/internal/build/v3/builds"
	targetType     = "-trunk_staging-userdebug"
)

// BuildGetRequest defines get request params for builds endpoint
type BuildGetRequest struct {
	BuildType   string
	Branch      string
	MaxResults  string
	SortingType string
	Successful  string
	Board       string
}

// formURL forms a URL with the given base URL and query parameters
func formURLForBuildAPI(req BuildGetRequest) (string, error) {
	// Parse the base URL
	parsedURL, err := url.Parse(buildsEndpoint)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}
	query := parsedURL.Query()

	queryParams := map[string]string{
		"buildType":   req.BuildType,
		"branch":      req.Branch,
		"maxResults":  req.MaxResults,
		"sortingType": req.SortingType,
		"successful":  req.Successful,
		"target":      req.Board + targetType,
	}

	// Add query params
	for key, value := range queryParams {
		query.Set(key, value)
	}

	parsedURL.RawQuery = query.Encode()

	return parsedURL.String(), nil
}

// getResponseFromAndroidBuildAPI makes a GET call to the Android API endpoint.
func getResponseFromAndroidBuildAPI(buildsReq BuildGetRequest, client *http.Client, rt RunType) (string, error) {
	requestURL, err := formURLForBuildAPI(buildsReq)
	if err != nil {
		return "", fmt.Errorf("error forming URL: %w", err)
	}
	log.Printf("Request URL: %s", requestURL)
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return "", fmt.Errorf("error creating GET request: %w", err)
	}
	// required for local run only
	if rt == Local {
		req.Header.Set("x-goog-user-project", "chromeos-bot")

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
		return "", fmt.Errorf("error reading response body: %w", err)
	}

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
	if !ok || buildIDStr == "" {
		return 0, fmt.Errorf("unable to find or parse 'buildId' or 'buildId' is empty")
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
func GetLatestGreenBuildNumber(rt RunType, buildsReq BuildGetRequest) (int, error) {
	httpClient, err := GetAndroidOnePlatformClient(rt)
	if err != nil {
		return 0, fmt.Errorf("failed to create HTTP client: %w", err)
	}
	resp, err := getResponseFromAndroidBuildAPI(buildsReq, httpClient, rt)
	if err != nil {
		return 0, fmt.Errorf("failed to get response from api: %w", err)
	}
	buildNumber, err := extractBuildNumberFromResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("failed to extract build number from response: %w", err)
	}
	return buildNumber, nil

}
