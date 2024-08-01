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

// gsClient specifies the APIs between archive-server and storage client.
// gsClient interface is used mainly for testing purpose,
// since storage pkg does not provide test pkg.
type gsClient interface {
	// getObject returns object handle given the gs object name.
	getObject(name *gsObjectName) gsObject
	// close closes the client.
	close() error
}

// gsObject specifies the APIs between archive-server and storage object.
type gsObject interface {
	// https://pkg.go.dev/cloud.google.com/go/storage#ObjectHandle.Attrs
	// storage.ErrObjectNotExist will be returned if the object is not found.
	Attrs(context.Context) (*storage.ObjectAttrs, error)
	// https://pkg.go.dev/cloud.google.com/go/storage#ObjectHandle.NewReader
	NewReader(context.Context) (io.ReadCloser, error)
	// https://pkg.go.dev/cloud.google.com/go/storage#ObjectHandle.NewRangeReader
	NewRangeReader(context.Context, int64, int64) (io.ReadCloser, error)
}

type realGSObject struct {
	gsObject *storage.ObjectHandle
}

func (c *realGSObject) Attrs(ctx context.Context) (*storage.ObjectAttrs, error) {
	return c.gsObject.Attrs(ctx)
}

func (c *realGSObject) NewReader(ctx context.Context) (io.ReadCloser, error) {
	r, err := c.gsObject.NewReader(ctx)
	return r, err
}

func (c *realGSObject) NewRangeReader(ctx context.Context, offset, length int64) (io.ReadCloser, error) {
	return c.gsObject.NewRangeReader(ctx, offset, length)
}

func newRealClient(ctx context.Context, creds string) (gsClient, error) {
	client, err := storage.NewClient(ctx, option.WithCredentialsFile(creds))
	if err != nil {
		return nil, err
	}
	return &realGSClient{gsClient: client}, nil
}

type realGSClient struct {
	gsClient *storage.Client
}

func (c *realGSClient) getObject(name *gsObjectName) gsObject {
	return &realGSObject{c.gsClient.Bucket(name.bucket).Object(name.path)}
}

func (c *realGSClient) close() error {
	return c.gsClient.Close()
}

// gsObjectName contains fields used to identify google storage object.
type gsObjectName struct {
	bucket string
	path   string
}
