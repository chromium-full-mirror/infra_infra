// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"encoding/base64"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/api/gitiles"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/hardcoded/chromeinfra"
)

// GerritAuthOptsOnBot provides auth opts to authorize to Gerrit from a bot env.
var GerritAuthOptsOnBot = chromeinfra.SetDefaultAuthOptions(auth.Options{
	Method: auth.AutoSelectMethod,
	Scopes: []string{auth.OAuthScopeEmail, gitiles.OAuthScope},
})

type clientThatSendsRequests interface {
	Do(*http.Request) (resp *http.Response, err error)
}

// AnyStringInGerritList checks for any overlap between the given list of
// strings, and the list at the given Gerrit URL.
func AnyStringInGerritList(ctx context.Context, c clientThatSendsRequests, list []string, listURL string) (bool, error) {
	fileText, err := fetchFileFromURL(ctx, c, listURL)
	if err != nil {
		return false, err
	}
	listFromURL := strings.Split(string(fileText), ",")
	mapFromURL := map[string]bool{}
	for _, str := range listFromURL {
		mapFromURL[str] = true
	}
	for _, str := range list {
		if mapFromURL[str] {
			return true, nil
		}
	}
	return false, nil
}

// fetchFileFromURL retrieves text from the given URL, using LUCI auth.
func fetchFileFromURL(ctx context.Context, c clientThatSendsRequests, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Annotate(err, "fetch file %q: create request", url).Err()
	}
	resp, err := sendHTTPRequestWithRetries(c, req, true)
	if err != nil {
		return nil, errors.Annotate(err, "fetch file %q", url).Err()
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Reason("fetch file %q: response code: %d", url, resp.StatusCode).Err()
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Annotate(err, "fetch file %q: read body", url).Err()
	}
	bs, err := base64.StdEncoding.DecodeString(string(data))
	return bs, errors.Annotate(err, "fetch file %q: decode data", url).Err()
}

// sendHTTPRequestWithRetries sends the given request with the given HTTP
// client, retrying if any HTTP errors are returned with optional backoff. Retry
// count is controlled by maxHTTPRetries.
func sendHTTPRequestWithRetries(c clientThatSendsRequests, req *http.Request, backoff bool) (*http.Response, error) {
	var (
		retries int
		resp    *http.Response
		err     error
	)
	for retries < maxHTTPRetries {
		resp, err = c.Do(req)
		// Only retry if request was sent successfully and status was not 200.
		if err != nil || resp.StatusCode == http.StatusOK {
			break
		}
		retries += 1
		if backoff {
			time.Sleep(time.Duration(math.Pow(2, float64(retries))) * time.Second)
		}
	}
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// GerritClient initializes an HTTP client with auth opts to read from Gerrit.
func GerritClient(ctx context.Context, authOpts auth.Options) (*http.Client, error) {
	ga := auth.NewAuthenticator(ctx, auth.SilentLogin, authOpts)
	c, err := ga.Client()
	if err != nil {
		return nil, errors.Annotate(err, "initializing HTTP client for Gerrit calls").Err()
	}
	return c, nil
}
