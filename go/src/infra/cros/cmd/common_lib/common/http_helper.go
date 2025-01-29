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
	luciauth "go.chromium.org/luci/server/auth"
)

// GerritAuthScopes provides auth scopes to authorize to Gerrit.
var GerritAuthScopes = []string{auth.OAuthScopeEmail, gitiles.OAuthScope}

type clientThatSendsRequests interface {
	Do(*http.Request) (resp *http.Response, err error)
}

// GetPoolsFromURL fetches a gerrit url having csv pool values and returns the array of pools
func GetPoolsFromURL(ctx context.Context, c clientThatSendsRequests, listURL string) ([]string, error) {
	fileText, err := fetchFileFromURL(ctx, c, listURL)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(fileText), ","), nil
}

// AnyStringInGerritList checks for any overlap between the given list of
// strings, and the list at the given Gerrit URL.
func AnyStringInGerritList(ctx context.Context, c clientThatSendsRequests, list []string, listURL string, pools []string) (bool, error) {
	if len(pools) == 0 {
		fetchedPools, err := GetPoolsFromURL(ctx, c, listURL)
		if err != nil {
			return false, err
		}
		pools = fetchedPools
	}
	mapFromURL := map[string]bool{}
	for _, str := range pools {
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

// withIDTokenAudience adds the given audience to the given auth options.
func withIDTokenAudience(audience string, authOpts auth.Options) auth.Options {
	authOpts.Audience = audience
	authOpts.UseIDTokens = true
	return authOpts
}

// SilentLoginHTTPClient initializes a silent-login HTTP client with the given
// auth options. It is not compatible with code running in an App Engine
// environment.
func SilentLoginHTTPClient(ctx context.Context, authOpts auth.Options) (*http.Client, error) {
	ga := auth.NewAuthenticator(ctx, auth.SilentLogin, authOpts)
	c, err := ga.Client()
	if err != nil {
		return nil, errors.Annotate(err, "initializing silent-login HTTP client").Err()
	}
	return c, nil
}

// GCPHTTPClient initializes an HTTP client for use in a GCP environment.
func GCPHTTPClient(ctx context.Context, rpcOpts ...luciauth.RPCOption) (*http.Client, error) {
	t, err := luciauth.GetRPCTransport(ctx, luciauth.AsSelf, rpcOpts...)
	if err != nil {
		return nil, errors.Annotate(err, "creating transport for GAE HTTP client").Err()
	}
	return &http.Client{Transport: t}, nil
}
