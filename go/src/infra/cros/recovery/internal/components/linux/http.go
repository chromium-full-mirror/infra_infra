// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package linux

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

// CurlURL runs the curl command with the provided downloadURL, headers,
// and extraCurlArgs. Returns the output of the curl command.
//
// If the curl command returns a non-nil error, the HTTP response code (parsed
// from the error) and the command error is returned along with the output of
// curl.
func CurlURL(ctx context.Context, run components.Runner, timeout time.Duration, downloadURL string, headers map[string]string, extraCurlArgs ...string) (curlOutput string, HTTPErrorResponseCode int, err error) {
	curlArgs := []string{downloadURL, "--fail"}
	for key, value := range headers {
		curlArgs = append(curlArgs, "-H", fmt.Sprintf("%s:%s", key, value))
	}
	if len(extraCurlArgs) != 0 {
		curlArgs = append(curlArgs, extraCurlArgs...)
	}
	combinedArgs := strings.Join(curlArgs, " ")
	log.Debugf(ctx, "Running 'curl %s'", combinedArgs)
	curlOutput, err = run(ctx, timeout, "curl", curlArgs...)
	if err != nil {
		HTTPErrorResponseCode = extractHTTPResponseCodeFromCurlErr(err)
		log.Debugf(ctx, "Failed run 'curl %q' with HTTPErrorResponseCode %d: %s", combinedArgs, HTTPErrorResponseCode, curlOutput)
		return curlOutput, HTTPErrorResponseCode, errors.Annotate(err, "failed to run 'curl %s' with HTTPErrorResponseCode %d: %s", combinedArgs, HTTPErrorResponseCode, curlOutput).Err()
	}
	log.Debugf(ctx, "Successful run of 'curl %s'", combinedArgs)
	return curlOutput, 0, nil
}

// extractHTTPResponseCodeFromCurlErr extracts the HTTP Response Code from a
// curl error object.
func extractHTTPResponseCodeFromCurlErr(err error) int {
	var httpResponseCode int
	stdErrStr, ok := components.StdErrTag.Value(err)
	if !ok {
		return 0
	}
	re := regexp.MustCompile("(returned error: )([0-9]*)")
	matchParts := re.FindAllStringSubmatch(stdErrStr, -1)
	if len(matchParts) == 1 {
		httpResponseCode, _ = strconv.Atoi(matchParts[0][2])
	}
	return httpResponseCode
}
