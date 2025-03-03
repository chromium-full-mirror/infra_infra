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
		name string
		url  string
	}{
		{"unsupported fs", "/mkfs-ext2-fake"},
		{"no params", "/mkfs-ext2/"},
		{"extra params", "/mkfs-ext2/?foo="},
		{"extra params2", "/mkfs-erofs/?file=abc&foo="},
		{"empty source", "/mkfs-squashfs/?file=abc&file="},
		{"duplicated source", "/mkfs-ext4/?file=abc&file=xyz&file=abc"},
	}
	gsa := &archiveServer{}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest("GET", tc.url, nil)
			w := httptest.NewRecorder()
			gsa.mkfsHandler(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("mkfsHandler(%q) codes %d, want %d", tc.url, w.Code, http.StatusBadRequest)
			}
		})
	}
}
