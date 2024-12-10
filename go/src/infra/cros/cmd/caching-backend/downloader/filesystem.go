// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"infra/libs/otil"
)

// mkfsHandler handles the series RPCs of mkfs-<fs_type>.
func (c *archiveServer) mkfsHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otil.FuncSpan(r.Context())
	defer func() { otil.EndSpan(span, nil) }()

	id := generateTraceID(r)
	maker, sources, err := parseMkfsRequest(id, r)
	if err != nil {
		errStr := fmt.Sprintf("make file sytem: %s", err)
		http.Error(w, errStr, http.StatusBadRequest)
		log.Print(errStr)
		return
	}

	// TODO b/369899216 - Normalize the query strings so difference order of them
	// will go to the same cache.

	_ = maker(ctx, w, sources...)
}

// parseMkfsRequest parses the request and returns the proper file system maker
// function for the request.
func parseMkfsRequest(id string, r *http.Request) (mkfsFunc, []string, error) {
	if r.Method != http.MethodGet {
		return nil, nil, fmt.Errorf("%s unsupport method %q, only GET is supported", id, r.Method)
	}

	// Extract the file system wanted from the RPC.
	// urlParts is like ["", RPC, remainings].
	urlParts := strings.SplitN(r.URL.Path, "/", 3)
	rpcParts := strings.SplitN(urlParts[1], "-", 2)
	if len(rpcParts) < 2 {
		return nil, nil, fmt.Errorf("%s no file system specified in %q", id, r.URL.Path)
	}
	fs := rpcParts[1]

	makerFunc, ok := filesystemMakers[fs]
	if !ok {
		return nil, nil, fmt.Errorf("%s unsupported file system: %s", id, fs)
	}

	qs, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return nil, nil, fmt.Errorf("%s invalid query: %w", id, err)
	}
	// User must and must only specify the `file` parameter.
	if l := len(qs); l == 0 || l > 1 {
		return nil, nil, fmt.Errorf("%s must specify %q parameter and no other parameters are allowed", id, mkfsParamKey)
	}

	// The source must be non-empty and identical.
	sources := qs[mkfsParamKey]
	set := map[string]struct{}{}
	for _, s := range sources {
		if s == "" {
			return nil, nil, fmt.Errorf("%s empty source", id)
		}
		if _, ok := set[s]; ok {
			return nil, nil, fmt.Errorf("%s duplicated source: %q", id, s)
		}
		set[s] = struct{}{}
	}
	return makerFunc, sources, nil
}

// The parameter (aka query string) for the mkfs call, e.g.
// GET/mkfs-foo/?file=file1.
const mkfsParamKey = "file"

var filesystemMakers = map[string]mkfsFunc{
	"ext2":     mkfsExt2,
	"ext3":     mkfsExt3,
	"ext4":     mkfsExt4,
	"squashfs": mkfsSquarshfs,
	"erofs":    mkfsErofs,
}

type mkfsFunc func(ctx context.Context, w http.ResponseWriter, sources ...string) error

func mkfsExt2(ctx context.Context, w http.ResponseWriter, sources ...string) error {
	return nil
}
func mkfsExt3(ctx context.Context, w http.ResponseWriter, sources ...string) error {
	return nil
}
func mkfsExt4(ctx context.Context, w http.ResponseWriter, sources ...string) error {
	return nil
}
func mkfsSquarshfs(ctx context.Context, w http.ResponseWriter, sources ...string) error {
	return nil
}
func mkfsErofs(ctx context.Context, w http.ResponseWriter, sources ...string) error {
	return nil
}
