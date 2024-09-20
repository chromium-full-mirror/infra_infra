// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package androidapi provides library functions for interating with Android One platform APIs go/ants-api#one-platform
package androidapi

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/user"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	androidBuildInternalScope = "https://www.googleapis.com/auth/androidbuild.internal"
	cloudPlatformScope        = "https://www.googleapis.com/auth/cloud-platform"
	gceServiceAccountJSONPath = "/creds/service_accounts/service-account-chromeos.json"
)

// RunType is enum defining where would client be executed
type RunType int

// Define constants for RunType
const (
	Local RunType = iota
	Container
	ServiceAccount
)

// String method to get a string representation of RunType
func (rt RunType) String() string {
	switch rt {
	case Local:
		return "local"
	case Container:
		return "container"
	case ServiceAccount:
		return "serviceAccount"
	default:
		return "unknown"
	}
}

// GetAndroidOnePlatformClient returns an HTTP client for making REST calls to Android One platform APIs.
// The rt parameter specifies the environment, determining whether the client is used on a bot with an attached Service Account (SA),
// within a container, or in a local environment.
func GetAndroidOnePlatformClient(rt RunType) (*http.Client, error) {
	creds, err := FetchCredentials(rt)
	if err != nil {
		return nil, fmt.Errorf("error fetching credentials: %w", err)
	}
	httpClient, err := getAuthorizedHTTP(&creds.TokenSource, 100*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to create authorized HTTP client: %w", err)
	}
	return httpClient, nil
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

// FetchCredentials fetches creds for authentication.
func FetchCredentials(rt RunType) (*google.Credentials, error) {
	switch rt {
	case Local:
		return fetchCredentialsFromJSON(true)
	case Container:
		return fetchCredentialsFromJSON(false)
	case ServiceAccount:
		return fetchDefaultCredentialsFromSA()
	default:
		return nil, fmt.Errorf("unknown run type")
	}
}

// fetchDefaultCredentialsFromSA retrieves credentials if running on a bot with a Service Account (SA) attached.
// Application Default Credentials (ADC) or GOOGLE_DEFAULT_CREDENTIALS environment variable is not set on the bot,
// credentials are obtained from the metadata server. Note that this method will not work if executed inside a container.
func fetchDefaultCredentialsFromSA() (*google.Credentials, error) {
	creds, err := google.FindDefaultCredentials(context.Background(), cloudPlatformScope, androidBuildInternalScope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading credentials: %v\n", err)
		os.Exit(1)
	}
	return creds, nil
}

// fetchCredentialsFromJSON retrieves credentials from a specified Service Account (SA) keyfile in JSON format.
// This function is designed to work within containers, where credentials are typically mounted by Drone inside the container.
// When running locally, the function will use the local gcloud Service Account keyfile.
// NOTE: If the local run fails, ensure that you have logged in with gcloud and have the necessary scopes.
func fetchCredentialsFromJSON(localRun bool) (*google.Credentials, error) {
	// Read the service account JSON key file
	var serviceAccountJSONPath string
	if localRun {
		serviceAccountJSONPath = guessUnixHomeDir() + "/.config/gcloud/application_default_credentials.json"
	} else {
		serviceAccountJSONPath = gceServiceAccountJSONPath
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

// fetches home directory
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
