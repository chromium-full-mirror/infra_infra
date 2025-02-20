// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestHTTPStatusCodeChecker(t *testing.T) {
	t.Parallel()
	code := 200
	resp := &http.Response{StatusCode: code}
	chk := httpStatusChecker(code)
	if err := chk(resp); err != nil {
		t.Errorf("httpStatusChecker(%d) failed unexpectedly with HTTP 200 response; err=%v", code, err)
	}

	resp = &http.Response{StatusCode: 404}
	if err := chk(resp); err == nil {
		t.Errorf("httpStatusChecker(%d) succeeded with HTTP 404 response; want error", code)
	}
}

func TestHTTPContentChecker(t *testing.T) {
	t.Parallel()
	content := "hello world"
	contentMD5 := "5eb63bbbe01eeed093cb22bb8f5acdc3"
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(content))}

	chk := httpContentChecker(contentMD5)
	if err := chk(resp); err != nil {
		t.Errorf("httpContentChecker(%q) failed unexpectedly; err=%v", content, err)
	}
	newContent := "bang!"
	resp = &http.Response{Body: io.NopCloser(strings.NewReader(newContent))}
	if err := chk(resp); err == nil {
		t.Errorf("httpContentChecker(%q) succeeded with input %q; want error", content, newContent)
	}
}

func TestCreateRequests(t *testing.T) {
	cases := []struct {
		name       string
		endpoints  []string
		uri        string
		headers    string
		wantURLs   []string
		wantHeader http.Header
	}{
		{
			name:       "endpoints with URI",
			endpoints:  []string{"http://1.1.1.1", "http://1.1.1.2"},
			uri:        "/health",
			headers:    "",
			wantURLs:   []string{"http://1.1.1.1/health", "http://1.1.1.2/health"},
			wantHeader: http.Header{},
		},
		{
			name:       "endpoints with URI and header",
			endpoints:  []string{"http://1.1.1.1", "http://1.1.1.2"},
			uri:        "/",
			headers:    "x-header1:1,x-header2:2",
			wantURLs:   []string{"http://1.1.1.1/", "http://1.1.1.2/"},
			wantHeader: http.Header{"X-Header1": {"1"}, "X-Header2": {"2"}},
		},
	}
	ctx := context.Background()
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			reqs, err := createRequests(ctx, c.endpoints, c.uri, c.headers)
			if err != nil {
				t.Errorf("createRequests(%v, %q, %q) err=%s, want nil", c.endpoints, c.uri, c.headers, err)
			}
			var gotURLs []string
			for _, r := range reqs {
				gotURLs = append(gotURLs, r.URL.String())
				if diff := cmp.Diff(c.wantHeader, r.Header); diff != "" {
					t.Errorf("createRequest(%v, %q, %q) returned unexpected headers (-want +got):\n%s", c.endpoints, c.uri, c.headers, diff)
				}
			}
		})
	}
}

func TestCreateRequestsErrors(t *testing.T) {
	cases := []string{"x-header", "x-header:", "x-header:1,x-foo", "x-header:1,x-foo:"}
	for _, c := range cases {
		c := c
		t.Run("bad headers", func(t *testing.T) {
			t.Parallel()
			if _, err := createRequests(context.Background(), []string{"http://1.1.1.1"}, "/", c); err == nil {
				t.Errorf("createRequests(%q) succeeded, want error", c)
			}
		})
	}

	t.Run("bad endpoints", func(t *testing.T) {
		t.Parallel()
		if _, err := createRequests(context.Background(), []string{""}, "", ""); err == nil {
			t.Errorf("createRequests(endpoints={''}) succeeded, want error")
		}
	})
}
