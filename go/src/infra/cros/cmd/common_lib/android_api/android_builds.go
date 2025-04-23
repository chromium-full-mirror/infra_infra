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

// AndroidBuildClient is an interface for android builds API
type AndroidBuildClient interface {
	GetLatestGreenBuildNumber(rt RunType, buildsReq BuildGetRequest) (int, error)
	GetBranchFromBuildID(rt RunType, buildsReq BuildGetRequest) (string, error)
}

// DefaultAndroidBuildClient is a concrete implementation of AndroidBuilds
type DefaultAndroidBuildClient struct{}

// Define a function that returns the interface
var AndroidBuildFactory = func() AndroidBuildClient {
	return &DefaultAndroidBuildClient{}
}

// BuildGetRequest defines get request params for builds endpoint
type BuildGetRequest struct {
	BuildID            string
	BuildType          string
	Branch             string
	MaxResults         string
	SortingType        string
	Successful         string
	Board              string
	BuildAttemptStatus string
}

// formBuildAPIURL forms a URL with the given base URL and query parameters
func formBuildAPIURL(req BuildGetRequest) (string, error) {
	// Parse the base URL
	parsedURL, err := url.Parse(buildsEndpoint)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}
	query := parsedURL.Query()

	query.Set("buildType", req.BuildType)
	query.Set("maxResults", req.MaxResults)
	query.Set("sortingType", req.SortingType)
	query.Set("successful", req.Successful)
	query.Set("target", req.Board+targetType)
	query.Set("buildAttemptStatus", req.BuildAttemptStatus)

	// Make sure the request params values are non-empty, otherwise server throws error.
	if req.BuildID != "" {
		query.Set("buildId", req.BuildID)
	}
	if req.Branch != "" {
		query.Set("branch", req.Branch)
	}

	parsedURL.RawQuery = query.Encode()

	return parsedURL.String(), nil
}

// getResponseFromAndroidBuildAPI makes a GET call to the Android API endpoint.
func getResponseFromAndroidBuildAPI(buildsReq BuildGetRequest, client *http.Client, rt RunType) (string, error) {
	requestURL, err := formBuildAPIURL(buildsReq)
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

// extractBuildInfo extracts information about the first build from the JSON response.
func extractBuildInfo(resp string) (map[string]interface{}, error) {
	var result map[string]interface{}

	// Parse the JSON string into the map
	err := json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return nil, fmt.Errorf("error parsing JSON: %w", err)
	}

	// Check if the "builds" field exists and is a non-empty array
	builds, ok := result["builds"].([]interface{})
	if !ok || len(builds) == 0 {
		return nil, fmt.Errorf("'builds' field is missing or empty")
	}

	// Fetch the first build and cast it to a map
	buildInfo, ok := builds[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unable to parse build information")
	}

	return buildInfo, nil
}

// extractBuildNumber extracts the build number from the JSON response.
func extractBuildNumber(resp string) (int, error) {
	buildInfo, err := extractBuildInfo(resp)
	if err != nil {
		return 0, err
	}

	buildIDStr, ok := buildInfo["buildId"].(string)
	if !ok || buildIDStr == "" {
		return 0, fmt.Errorf("unable to find or parse 'buildId' or 'buildId' is empty")
	}

	// Convert the buildId from string to int
	if buildID, err := strconv.Atoi(buildIDStr); err != nil {
		return 0, fmt.Errorf("error converting 'buildId' to int: %w", err)
	} else {
		return buildID, nil
	}
}

// extractBuildBranch extracts the build branch from the JSON response.
func extractBuildBranch(resp string) (string, error) {
	buildInfo, err := extractBuildInfo(resp)
	if err != nil {
		return "", err
	}

	branchStr, ok := buildInfo["branch"].(string)
	if !ok || branchStr == "" {
		return "", fmt.Errorf("unable to find or parse 'branch' or 'branch' is empty")
	}

	return branchStr, nil
}

// fetchBuildInfo calls the underlying API to get the response.
func fetchBuildInfo(rt RunType, buildsReq BuildGetRequest) (string, error) {
	httpClient, err := GetAndroidOnePlatformClient(rt)
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP client: %w", err)
	}

	return getResponseFromAndroidBuildAPI(buildsReq, httpClient, rt)
}

// GetLatestGreenBuildNumber calls the android one platform api to get latest build version
func (a *DefaultAndroidBuildClient) GetLatestGreenBuildNumber(rt RunType, buildsReq BuildGetRequest) (int, error) {
	resp, err := fetchBuildInfo(rt, buildsReq)
	if err != nil {
		return 0, fmt.Errorf("failed to get response from api: %w", err)
	}

	buildNumber, err := extractBuildNumber(resp)
	if err != nil {
		return 0, fmt.Errorf("failed to extract build number from response: %w", err)
	}

	return buildNumber, nil
}

// GetLatestGreenBuildNumber calls the android one platform api to get branch for given build Id.
func (a *DefaultAndroidBuildClient) GetBranchFromBuildID(rt RunType, buildsReq BuildGetRequest) (string, error) {
	resp, err := fetchBuildInfo(rt, buildsReq)
	if err != nil {
		return "", fmt.Errorf("failed to get response from api: %w", err)
	}

	branch, err := extractBuildBranch(resp)
	if err != nil {
		return "", fmt.Errorf("failed to extract branch from response: %w", err)
	}

	return branch, nil
}
