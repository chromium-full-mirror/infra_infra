// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package firestore implements an interface for all firestore API actions.
package common

import (
	"context"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/option"
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
