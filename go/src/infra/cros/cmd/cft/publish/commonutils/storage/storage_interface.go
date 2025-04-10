// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

var ErrBucketNotExist = errors.New("bucket does not exist")
var ErrObjectNotExist = errors.New("object does not exist")

// actionTimeout is the timeout that pertains to reading or writing
// a single file from/to GCS.
const actionTimeout = 60 * time.Second

// This file exists to provide a mockable interface to the google storage client
type StorageClientInterface interface {
	Close()
	Write(ctx context.Context, filePath string, gsObject GSObject) error
	Read(ctx context.Context, gsObject GSObject, destFilePath string) error
}

type StorageClient struct {
	client *storage.Client
}

func NewStorageClientWithCredsFile(ctx context.Context, credentialsFile string) (*StorageClient, error) {
	client, err := storage.NewClient(ctx, option.WithCredentialsFile(credentialsFile))
	if err != nil {
		return nil, err
	}
	return &StorageClient{
		client: client,
	}, nil
}

func NewStorageClientWithDefaultAccount(ctx context.Context) (*StorageClient, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return &StorageClient{
		client: client,
	}, nil
}

// Write a single file to GCS.
func (c *StorageClient) Write(ctx context.Context, filePath string, gsObject GSObject) error {
	// Open local file.
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("unable to open local files %s, %w", filePath, err)
	}
	defer f.Close()

	ctx, cancel := context.WithTimeout(ctx, actionTimeout)
	defer cancel()

	wc := c.client.Bucket(gsObject.Bucket).Object(gsObject.Object).NewWriter(ctx)
	if _, err := io.Copy(wc, f); err != nil {
		return fmt.Errorf("failed file copy for local file %s, %w", filePath, err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("failed to close writer: %w", err)
	}
	return nil
}

// Read downloads a file from GCS to the given local path. If the bucket does
// not exist in GCS, the method returns ErrBucketNotExist. If the object does
// not exist in GCS, the method returns ErrObjectNotExist.
func (c *StorageClient) Read(ctx context.Context, gsObject GSObject, destFilePath string) (retErr error) {
	ctx, cancel := context.WithTimeout(ctx, actionTimeout)
	defer cancel()

	reader, err := c.client.Bucket(gsObject.Bucket).Object(gsObject.Object).NewReader(ctx)
	if err == storage.ErrBucketNotExist {
		return ErrBucketNotExist
	}
	if err == storage.ErrObjectNotExist {
		return ErrObjectNotExist
	}
	if err != nil {
		return fmt.Errorf("opening reader: %w", err)
	}
	defer func() {
		if err := reader.Close(); err != nil && retErr == nil {
			retErr = err
		}
	}()

	dst, err := os.OpenFile(destFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("opening destination file %q: %w", destFilePath, err)
	}
	defer func() {
		if err := dst.Close(); err != nil && retErr == nil {
			retErr = err
		}
	}()

	if _, err := io.Copy(dst, reader); err != nil {
		return fmt.Errorf("copying to local file: %w", err)
	}
	return nil
}

func (c *StorageClient) Close() {
	c.client.Close()
}
