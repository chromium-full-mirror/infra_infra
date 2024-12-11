// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
)

// downloadClient specifies the APIs between archive-server and storage client.
// downloadClient interface is used mainly for testing purpose,
// since storage pkg does not provide test pkg.
type downloadClient interface {
	// getObject returns object handle given the storage object name.
	getObject(name *storageObjectName) storageObject
	// close closes the client.
	close() error
}

// storageObject specifies the APIs between archive-server and storage object.
type storageObject interface {
	// https://pkg.go.dev/cloud.google.com/go/storage#ObjectHandle.Attrs
	// storage.ErrObjectNotExist will be returned if the object is not found.
	Attrs(context.Context) (*storage.ObjectAttrs, error)
	// https://pkg.go.dev/cloud.google.com/go/storage#ObjectHandle.NewReader
	NewReader(context.Context) (io.ReadCloser, error)
	// https://pkg.go.dev/cloud.google.com/go/storage#ObjectHandle.NewRangeReader
	NewRangeReader(context.Context, int64, int64) (io.ReadCloser, error)
}

type gsObject struct {
	object *storage.ObjectHandle
}

func (c *gsObject) Attrs(ctx context.Context) (*storage.ObjectAttrs, error) {
	return c.object.Attrs(ctx)
}

func (c *gsObject) NewReader(ctx context.Context) (io.ReadCloser, error) {
	r, err := c.object.NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("new GS reader: %w", err)
	}
	return r, nil
}

func (c *gsObject) NewRangeReader(ctx context.Context, offset, length int64) (io.ReadCloser, error) {
	return c.object.NewRangeReader(ctx, offset, length)
}

func newRealClient(ctx context.Context, androidServer string, opts ...option.ClientOption) (downloadClient, error) {
	gs, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("new client for GS: %w", err)
	}
	hc, _, err := transport.NewHTTPClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("new client for android: %w", err)
	}
	return &realDownloadClient{gsClient: gs, androidClient: &androidClient{hc: hc, server: androidServer}}, nil
}

type realDownloadClient struct {
	gsClient      *storage.Client
	androidClient *androidClient
}

// We use "android-build" as a magic string to indicate the object is from a
// general GS bucket or from android build API server.
func (c *realDownloadClient) getObject(name *storageObjectName) storageObject {
	if name.bucket == "android-build" {
		return &androidObject{client: c.androidClient, path: name.path}
	}
	return &gsObject{c.gsClient.Bucket(name.bucket).Object(name.path)}
}

func (c *realDownloadClient) close() error {
	// There's no need to close androidClient as it's a basic http client.
	return c.gsClient.Close()
}

// storageObjectName contains fields used to identify storage object.
type storageObjectName struct {
	bucket string
	path   string
}

type androidClient struct {
	hc     *http.Client
	server string
}

// request requests to the Android build API server.
func (c *androidClient) request(ctx context.Context, path string) (*http.Response, error) {
	url := fmt.Sprintf("%s/%s", c.server, path)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("android client request: %w", err)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("android client request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		// The response from Android build API server is a JSON encoded
		// googleapi.Error.
		content, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		e := struct {
			Error googleapi.Error `json:"error"`
		}{}
		err = json.Unmarshal(content, &e)
		if err != nil {
			return nil, &googleapi.Error{Code: resp.StatusCode, Message: string(content)}
		}
		return nil, &e.Error
	}

	return resp, nil
}

type androidObject struct {
	client *androidClient
	path   string
}

func (c *androidObject) Attrs(ctx context.Context) (*storage.ObjectAttrs, error) {
	resp, err := c.client.request(ctx, c.metadataPath())
	if err != nil {
		return nil, fmt.Errorf("attrs: %w", err)
	}
	content, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	// The response is json encoded metadata, and we need to parse them and
	// convert to the target type.
	data := struct {
		Name        string `json:"name"`
		Size        int64  `json:"size,string"`
		ContentType string `json:"contentType"`
	}{}
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, fmt.Errorf("attrs: json %q: %w", content, err)
	}
	return &storage.ObjectAttrs{Name: data.Name, Size: data.Size, ContentType: data.ContentType}, nil
}

func (c *androidObject) NewReader(ctx context.Context) (io.ReadCloser, error) {
	resp, err := c.client.request(ctx, c.rawContentPath())
	if err != nil {
		return nil, fmt.Errorf("new android reader: %w", err)
	}
	return resp.Body, nil
}

func (c *androidObject) NewRangeReader(context.Context, int64, int64) (io.ReadCloser, error) {
	return nil, fmt.Errorf("android build API server doesn't support range requests")
}

func (c *androidObject) metadataPath() string {
	// Android build API server returns the file metadata in json format when
	// 'alt=media'.
	return c.escapeArtifactPath() + "?alt=json"
}

func (c *androidObject) rawContentPath() string {
	// Android build API server returns the whole file content when 'alt=media'.
	return c.escapeArtifactPath() + "?alt=media"
}

// escapeArtifactPath escapes the "/" in the artifact name.
//
// For an artifact name including "/", we need to replace it with "%2F" before
// we request the Android build API server.
func (c *androidObject) escapeArtifactPath() string {
	// The path is in format of
	// {rpc}/{bid}/{target}/attempts/{attemptsid}/artifacts/{artifactid}
	// The {artifactid} part (7th) needs to be escaped.
	parts := strings.SplitN(c.path, "/", 7)
	if len(parts) < 7 {
		// The path is in wrong format. Do nothing here and pass it to the caller
		// to handle it.
		return c.path
	}
	parts[6] = url.QueryEscape(parts[6])
	return strings.Join(parts, "/")
}
