// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMkfsHandlerBadRequests(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name   string
		method string
		url    string
	}{
		{"bad method", "POST", "/mkfs-ext2"},
		{"no fs", "GET", "/mkfs-"},
		{"unsupported fs", "GET", "/mkfs-ext2-fake"},
		{"no params", "GET", "/mkfs-ext2/"},
		{"extra params", "GET", "/mkfs-ext2/foo="},
		{"extra params2", "GET", "/mkfs-erofs/file=abc&foo="},
		{"empty source", "GET", "/mkfs-squashfs/file=abc&file="},
		{"duplicated source", "GET", "/mkfs-ext4/file=abc&file=xyz&file=abc"},
	}
	gsa := &archiveServer{}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(tc.method, tc.url, nil)
			w := httptest.NewRecorder()
			gsa.mkfsHandler(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("mkfsHandler(%q %q) codes %d, want %d", tc.method, tc.url, w.Code, http.StatusBadRequest)
			}
		})
	}
}
