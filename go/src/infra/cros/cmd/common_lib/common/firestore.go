// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package firestore implements an interface for all firestore API actions.
package common

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/cenkalti/backoff/v4"
	"google.golang.org/api/option"

	"go.chromium.org/luci/common/logging"
)

// InitClient returns a firestore client set to access the given project and
// database. A databaseID is required as the default database (default) requires
// a separate process.
func InitClient(ctx context.Context, projectID, databaseID string, opts ...option.ClientOption) (*firestore.Client, error) {
	client, err := firestore.NewClientWithDatabase(ctx, projectID, databaseID, opts...)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// EstablishFirestoreConnection uses provided credentials
// to establish a connection to the test platform firestore database.
func EstablishFirestoreConnection(ctx context.Context, firestoreDatabaseName, creds string) (client *firestore.Client, err error) {
	projectID := TestPlatformDataProjectID

	clientOpts := []option.ClientOption{}

	if creds != "" {
		clientOpts = append(clientOpts, option.WithCredentialsFile(creds))
	}

	retryFunc := func() (*firestore.Client, error) {
		return InitClient(ctx, projectID, firestoreDatabaseName, clientOpts...)
	}
	notifyFunc := func(e error, t time.Duration) {
		logging.Infof(ctx, "failed to initialize client after %s with error: %s", t, e)
	}
	backer := backoff.NewExponentialBackOff(
		backoff.WithInitialInterval(time.Second*2),
		backoff.WithMaxInterval(time.Second*16),
		backoff.WithMaxElapsedTime(time.Minute),
	)
	return backoff.RetryNotifyWithData(retryFunc, backer, notifyFunc)
}

// FirestoreItem wraps the interface item with it's intended document name.
type FirestoreItem struct {
	DocName string
	Datum   interface{}
}

func BatchSet(ctx context.Context, collection *firestore.CollectionRef, client *firestore.Client, items []*FirestoreItem, opts ...firestore.SetOption) ([]*firestore.BulkWriterJob, error) {
	bulkWriter := client.BulkWriter(ctx)
	// Flush and close the bulkWriter when done.
	defer bulkWriter.End()

	// Store the job receives in a list to be processed by the caller.
	jobs := []*firestore.BulkWriterJob{}

	// Add items to the collection provided.
	for _, item := range items {
		// Create a new Doc reference keyed with the item.DocName
		doc := collection.Doc(item.DocName)

		// Add a set action to the bulkWriter. This action will sit in the queue
		// until either 50 items are enqueue'd or the end() call is made.
		//
		// NOTE: Set creates or overwrites the document in the collection.
		job, err := bulkWriter.Set(doc, item.Datum, opts...)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}
