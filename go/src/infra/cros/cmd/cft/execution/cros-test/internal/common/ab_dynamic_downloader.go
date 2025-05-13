// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	androidBuildInternalScope     = "https://www.googleapis.com/auth/androidbuild.internal"
	buildsEndpoint                = "https://androidbuildinternal.googleapis.com/android/internal/build/v3/builds"
	cloudPlatformScope            = "https://www.googleapis.com/auth/cloud-platform"
	getBuildQuery                 = buildsEndpoint + "?buildType=submitted&branch=%s&target=%s&maxResults=1&buildAttemptStatus=complete&successful=True"
	getArtifactUrlQuery           = buildsEndpoint + "/%s/%s/attempts/latest/artifacts/%s/url?redirect=false"
	localCacheDir                 = "/tmp/tf_local_cache"
	localCacheMaxSize             = 100 * 1024 * 1024 * 1024 // 100GB
	satlabServiceAccountJSONPath  = "/creds/service_accounts/skylab-drone.json"
	defaultParallelDownloadChunks = 10
	xtsSuiteMountDir              = "/tmp/xts_suite_mount"
	xtsResultsDir                 = "/tmp/xts_results"
)

// Use default (localCacheMaxSize) unless set externally.
var DiskCacheSize = int64(localCacheMaxSize)

// Alias the exec command for testing.
var execCommand = exec.Command
var execLookPath = exec.LookPath

func FetchXtsSuite(testType string, branch string, target string, buildId int) (string, error) {
	cache, err := NewDiskCache(DiskCacheSize, localCacheDir)
	if err != nil {
		return "", fmt.Errorf("failed initializing disk cache: %w", err)
	}

	zipName := fmt.Sprintf("android-%s.zip", testType)
	zipPath, exists := cache.GetFile(zipName, buildId)
	if !exists {
		// ZIP is not in local disk cache, need to download.
		signedUrl, err := getArtifactDownloadURL(buildId, target, zipName)
		if err != nil {
			return "", fmt.Errorf("failed to fetch xTS suite ZIP: %w", err)
		}

		contentLength, err := getUrlContentLength(signedUrl)
		if err != nil {
			return "", fmt.Errorf("failed downloading xTS suite ZIP: %w", err)
		}

		file, err := cache.CreateFile(zipName, buildId, contentLength, false)
		if err != nil {
			return "", fmt.Errorf("error creaing local xTS suite file: %w", err)
		}
		defer file.Close()

		err = downloadURLParallel(signedUrl, file, contentLength, defaultParallelDownloadChunks)
		if err != nil {
			return "", fmt.Errorf("failed downloading xTS suite ZIP: %w", err)
		}

		zipPath = file.Name()
	}

	// Unmounting any potential previous mountings.
	unmountZip(xtsSuiteMountDir, false)
	// Mounting the downloaded ZIP.
	err = mountZip(zipPath, xtsSuiteMountDir)
	if err != nil {
		return "", fmt.Errorf("failed mounting xTS suite ZIP: %w", err)
	}
	err = linkXtsSuiteJar(testType, xtsSuiteMountDir)
	if err != nil {
		return "", fmt.Errorf("failed linking xTS suite JAR: %w", err)
	}

	// Link the results and logs folders in.
	// Needed because these folders are hardcoded into CompatibilityBuildHelper.java
	err = linkXtsResultDir(testType, xtsSuiteMountDir)
	if err != nil {
		return "", fmt.Errorf("failed linking xTS results directory: %w", err)
	}
	err = linkXtsLogsDir(testType, xtsSuiteMountDir)
	if err != nil {
		return "", fmt.Errorf("failed linking xTS logs directory: %w", err)
	}

	//Add xTS specific launcher to the path
	launcherDir := filepath.Join(xtsSuiteMountDir, fmt.Sprintf("android-%s", testType), "tools")
	os.Setenv("PATH", fmt.Sprintf("%s:%s", launcherDir, os.Getenv("PATH")))
	os.Setenv("USE_ATS", "false")
	return xtsSuiteMountDir, nil
}

// getAndroidOnePlatformClient returns an HTTP client for making REST calls to Android One platform APIs.
func getAndroidOnePlatformClient() (*http.Client, error) {
	// Read the service account JSON key file
	jsonData, err := os.ReadFile(satlabServiceAccountJSONPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}
	creds, err := google.CredentialsFromJSON(context.Background(), jsonData, cloudPlatformScope, androidBuildInternalScope)
	if err != nil {
		return nil, fmt.Errorf("error fetching credentials: %w", err)
	}

	httpClient := &http.Client{
		Timeout: 100 * time.Second,
		Transport: &oauth2.Transport{
			Source: creds.TokenSource,
			Base:   &http.Transport{},
		},
	}
	return httpClient, nil
}

func getLatestGreenBuildNumber(branch string, target string) (int, error) {
	httpClient, err := getAndroidOnePlatformClient()
	if err != nil {
		return 0, fmt.Errorf("failed to create HTTP client: %w", err)
	}
	requestURL := fmt.Sprintf(getBuildQuery, branch, target)
	resp, err := getResponseFromAndroidBuildAPI(requestURL, httpClient)
	if err != nil {
		return 0, fmt.Errorf("failed to get response from api: %w", err)
	}
	buildNumber, err := extractBuildNumberFromResponse(resp)
	if err != nil {
		return 0, fmt.Errorf("failed to extract build number from response: %w", err)
	}
	return buildNumber, nil
}

func getArtifactDownloadURL(buildId int, target string, resourceId string) (string, error) {
	httpClient, err := getAndroidOnePlatformClient()
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP client: %w", err)
	}
	requestURL := fmt.Sprintf(getArtifactUrlQuery, strconv.Itoa(buildId), target, resourceId)
	resp, err := getResponseFromAndroidBuildAPI(requestURL, httpClient)
	if err != nil {
		return "", fmt.Errorf("failed to get response from api: %w", err)
	}
	artifactUrl, err := extractArtifactUrlFromResponse(resp)
	if err != nil {
		return "", fmt.Errorf("failed to extract artifact URL from response: %w", err)
	}
	return artifactUrl, nil
}

// getResponseFromAndroidBuildAPI makes a GET call to the Android API endpoint.
func getResponseFromAndroidBuildAPI(requestURL string, client *http.Client) (string, error) {
	log.Printf("Request URL: %s", requestURL)
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return "", fmt.Errorf("error creating GET request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
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
	var result map[string]any

	// Parse the JSON string into the map
	err := json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return 0, fmt.Errorf("error parsing JSON: %w", err)
	}

	// Check if the "builds" field exists and is an array
	builds, ok := result["builds"].([]any)
	if !ok || len(builds) == 0 {
		return 0, fmt.Errorf("'builds' field is missing or empty")
	}

	// Fetch the first build and cast it to a map
	buildInfo, ok := builds[0].(map[string]any)
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

func extractArtifactUrlFromResponse(resp string) (string, error) {
	var result map[string]any

	// Parse the JSON string into the map
	err := json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return "", fmt.Errorf("error parsing JSON: %w, response: %s", err, resp)
	}

	signedUrl, ok := result["signedUrl"].(string)
	if !ok || signedUrl == "" {
		return "", fmt.Errorf("unable to find or parse 'signedUrl' from response: %s", resp)
	}

	return signedUrl, nil
}

func getUrlContentLength(url string) (int64, error) {
	// Get the content length (= file size) from the header request.
	resp, err := http.Head(url)
	if err != nil {
		return -1, fmt.Errorf("error downloading xTS suite from URL: %s, %w", url, err)
	}
	if resp.StatusCode != 200 {
		return -1, fmt.Errorf("unexpected status code: %d fetching xTS suite", resp.StatusCode)
	}
	headerMap := resp.Header
	length, err := strconv.ParseInt(headerMap["Content-Length"][0], 10, 64)
	if err != nil {
		return -1, fmt.Errorf("error getting size of xTS suite artifact from URL: %s, %w", url, err)
	}
	return length, nil
}

var downloadChunkClient = func() *http.Client {
	return &http.Client{}
}

// downloadChunk downloads a chunk of the URL and writes it to the file.
func downloadChunk(url string, file *os.File, start, end int64, chunk int) error {
	client := downloadChunkClient()
	fmt.Printf("Downloading chunk %d from %d to %d\n", chunk, start, end)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	// Set the Range header to download a specific chunk
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download chunk %d: %w", chunk, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("received non-2xx status code: %d while downloading chunk %d", resp.StatusCode, chunk)
	}

	// Copy the content of response's body to the correct position
	// in the output file in a thread-safe way.
	defChunkSize := int64(100 * 1024 * 1024) // 100KB default buffer size.
	currPos := start
	for currPos < end {
		nextChunkSize := end - currPos + 1
		if nextChunkSize > defChunkSize {
			nextChunkSize = defChunkSize
		}
		buf := make([]byte, nextChunkSize)
		readBytes, err := resp.Body.Read(buf)
		if err != nil {
			return fmt.Errorf("failed to read chunk %d: readbytes=%d: %w", chunk, readBytes, err)
		}
		_, err = file.WriteAt(buf[0:readBytes], currPos)
		if err != nil {
			return fmt.Errorf("failed to write chunk %d to file: %w", chunk, err)
		}
		currPos += int64(readBytes)
	}

	return nil
}

func downloadURLParallel(url string, file *os.File, fileSize int64, numChunks int) error {
	var chunkSize int64
	if numChunks <= 0 {
		return fmt.Errorf("number of chunks must be greater than 0")
	}
	chunkSize = fileSize / int64(numChunks)

	if fileSize%int64(numChunks) != 0 {
		chunkSize++
	}

	var wg sync.WaitGroup
	var downloadErr error

	for start := int64(0); start < fileSize; start += chunkSize {
		end := start + chunkSize - 1
		if end >= fileSize {
			end = fileSize - 1
		}

		wg.Add(1)
		go func(start, end int64, chunk int) {
			defer wg.Done()
			err := downloadChunk(url, file, start, end, chunk)
			if err != nil {
				downloadErr = fmt.Errorf("failed to download chunk %d: %w", chunk, err)
			}

		}(start, end, int(start/chunkSize))
	}

	wg.Wait()

	if downloadErr != nil {
		return downloadErr
	}
	return nil
}

func mountZip(zipPath string, mountPoint string) error {
	// Check if fuse-zip is installed
	_, err := execLookPath("fuse-zip")
	if err != nil {
		return fmt.Errorf("fuse-zip not found: %w", err)
	}

	// Create the mount point if it doesn't exist
	err = os.MkdirAll(mountPoint, 0755)
	if err != nil {
		return fmt.Errorf("failed to create mount point: %w", err)
	}

	// Mount the zip file using fuse-zip
	cmd := execCommand("fuse-zip", zipPath, mountPoint) // , "-r", "-o", "allow_other"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to mount zip file: %w", err)
	}

	return nil
}

func unmountZip(mountPoint string, deleteMountPoint bool) error {
	// Unmount the zip file
	cmd := execCommand("fusermount", "-u", mountPoint)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to unmount zip file: %w", err)
	}
	// Remove the mount point directory
	if deleteMountPoint {
		err = os.Remove(mountPoint)
		if err != nil {
			return fmt.Errorf("failed to remove mount point: %w", err)
		}
	}
	return nil
}

func linkXtsSuiteJar(testType string, mountPoint string) error {
	jarPath := filepath.Join(mountPoint, fmt.Sprintf("android-%s", testType), "tools", fmt.Sprintf("%s-tradefed.jar", testType))
	return checkAndLink(jarPath, filepath.Join("/tradefed", filepath.Base(jarPath)), false)
}

func linkXtsResultDir(testType string, mountPoint string) error {
	targetDir := filepath.Join(mountPoint, fmt.Sprintf("android-%s", testType), "results")
	return checkAndLink(xtsResultsDir, targetDir, true)
}

func linkXtsLogsDir(testType string, mountPoint string) error {
	targetDir := filepath.Join(mountPoint, fmt.Sprintf("android-%s", testType), "logs")
	// Logs are also mapped to the same result directory.
	return checkAndLink(xtsResultsDir, targetDir, true)
}

func checkAndLink(source string, mountPoint string, isDirectory bool) error {
	fmt.Printf("Linking %s to %s\n", source, mountPoint)

	// Check if the link already exists.
	if _, err := os.Stat(mountPoint); err == nil || !os.IsNotExist(err) {
		return nil
	}

	if isDirectory {
		// Create the target dir if it doesn't exist.
		err := os.MkdirAll(source, 0755)
		if err != nil {
			return fmt.Errorf("failed to create results dir: %w", err)
		}
	}

	// Create the link.
	cmd := execCommand("ln", "-s", source, mountPoint)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to link results dir: %w", err)
	}
	return nil
}
