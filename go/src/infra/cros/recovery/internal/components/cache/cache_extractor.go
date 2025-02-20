// Copyright (c) 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cache

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/components/linux"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/logger/metrics"
	"go.chromium.org/infra/cros/recovery/scopes"
)

// ExtractRequest holds all data required to extract file from file on cache service.
type ExtractRequest struct {
	// URL to download the file from cache service.
	CacheFileURL string
	// Name of the file we wantt o extract from file.
	ExtractFileName string
	// Filepath of destination file.
	DestintionFilePath string
	// Download timeout.
	Timeout time.Duration
	// Number of times the download can be re-attempted.
	DownloadImageReattemptCount int
	// Wait before the download is re-attempted.
	DownloadImageReattemptWait time.Duration
}

// Extract extract file from cache service by modifying URL to download the file.
func Extract(ctx context.Context, req *ExtractRequest, run components.Runner) error {
	// Path provided by TLS cannot be used for downloading and/or extracting the image file.
	// But we can utilize the address of caching service and apply some string manipulation to construct the URL that can be used for this.
	// Example: `http://Addr:8082/extract/chromeos-image-archive/board-release/R99-XXXXX.XX.0/chromiumos_test_image.tar.xz?file=chromiumos_test_image.bin`
	extractPath := strings.Replace(req.CacheFileURL, "/download/", "/extract/", 1)
	sourcePath := fmt.Sprintf("%s?file=%s", extractPath, req.ExtractFileName)
	// We need to count the original download as well as any re-attempts.
	remainingDownloadAttempts := req.DownloadImageReattemptCount + 1
	for {
		if httpResponseCode, err := CurlFile(ctx, run, sourcePath, req.DestintionFilePath, req.Timeout); err != nil {
			log.Debugf(ctx, "Extract: HTTP Response Code is :%d", httpResponseCode)
			if httpResponseCode/100 == 5 && remainingDownloadAttempts > 1 {
				remainingDownloadAttempts -= 1
				time.Sleep(req.DownloadImageReattemptWait)
				continue
			}
			return errors.Annotate(err, "extract from cache").Err()
		} else {
			break
		}
	}
	return nil
}

// CurlFile downloads file by using curl util.
func CurlFile(ctx context.Context, run components.Runner, cacheURL, destinationPath string, timeout time.Duration) (int, error) {
	_, HTTPErrorCode, err := curlCacheURL(ctx, run, timeout, cacheURL, "--output", destinationPath)
	if err != nil {
		return HTTPErrorCode, errors.Annotate(err, "failed to curl file %q to %q", cacheURL, destinationPath).Err()
	}
	return 0, nil
}

// CurlFileContents reads a file by using curl util.
func CurlFileContents(ctx context.Context, run components.Runner, cacheURL string, timeout time.Duration) (string, int, error) {
	fileContents, HTTPErrorCode, err := curlCacheURL(ctx, run, timeout, cacheURL)
	if err != nil {
		return "", HTTPErrorCode, errors.Annotate(err, "failed to curl contents of file %q", cacheURL).Err()
	}
	return fileContents, 0, nil
}

// curlCacheURL runs the curl command with the provided cacheURL and
// extraCurlArgs. Additional curl arguments are added to include HTTP request
// headers that need to be added to cache HTTP requests. Returns the output
// of the curl command.
//
// If the curl command returns a non-nil error, the HTTP response code (parsed
// from the error) and the command error is returned along with the output of
// curl.
func curlCacheURL(ctx context.Context, run components.Runner, timeout time.Duration, cacheURL string, extraCurlArgs ...string) (curlOutput string, HTTPErrorResponseCode int, err error) {
	curlOutput, HTTPErrorResponseCode, err = linux.CurlURL(ctx, run, timeout, cacheURL, HTTPRequestHeaders(ctx), extraCurlArgs...)
	if err != nil {
		RecordCacheAccessFailure(ctx, cacheURL, HTTPErrorResponseCode)
	}
	return curlOutput, HTTPErrorResponseCode, err
}

// HTTPRequestHeaders returns a map of header keys to values of HTTP headers
// that should be included in HTTP requests to the cache service. The values
// are retrieved from scopes specific to the provided context.
func HTTPRequestHeaders(ctx context.Context) map[string]string {
	headers := make(map[string]string)
	if v, ok := scopes.GetParam(ctx, scopes.ParamKeySwarmingTaskID); ok {
		headers["X-SWARMING-TASK-ID"] = v.(string)
	}
	if v, ok := scopes.GetParam(ctx, scopes.ParamKeyBuildbucketID); ok {
		headers["X-BBID"] = v.(string)
	}
	return headers
}

// RecordCacheAccessFailure records non-500 HTTP response errors of an access
// attempt of a path as an observation metric.
//
// We are only interested in 500 errors coming from the caching service at the
// moment, so the observation is only recorded if the code is >= 500. Non-500
// errors are recorded by the caching service.
func RecordCacheAccessFailure(ctx context.Context, sourcePath string, failedHTTPResponseCode int) {
	if failedHTTPResponseCode >= 500 {
		if execMetric := metrics.GetDefaultAction(ctx); execMetric != nil {
			execMetric.Observations = append(execMetric.Observations,
				metrics.NewInt64Observation("cache_failed_response_code", int64(failedHTTPResponseCode)),
				metrics.NewStringObservation("cache_failed_source_path", sourcePath),
			)
		}
	}
}
