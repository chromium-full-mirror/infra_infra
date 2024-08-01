// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"io"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
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
	return r, err
}

func (c *gsObject) NewRangeReader(ctx context.Context, offset, length int64) (io.ReadCloser, error) {
	return c.object.NewRangeReader(ctx, offset, length)
}

func newRealClient(ctx context.Context, creds string) (downloadClient, error) {
	client, err := storage.NewClient(ctx, option.WithCredentialsFile(creds))
	if err != nil {
		return nil, err
	}
	return &realDownloadClient{gsClient: client}, nil
}

type realDownloadClient struct {
	gsClient *storage.Client
}

func (c *realDownloadClient) getObject(name *storageObjectName) storageObject {
	return &gsObject{c.gsClient.Bucket(name.bucket).Object(name.path)}
}

func (c *realDownloadClient) close() error {
	return c.gsClient.Close()
}

// storageObjectName contains fields used to identify storage object.
type storageObjectName struct {
	bucket string
	path   string
}
