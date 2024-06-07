// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package internal

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/cenkalti/backoff/v4"
	"google.golang.org/api/option"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/common"
)

// UpdateShaStorage connects the the firestore and uploads the SHAs produced
// during the uprev service.
func UpdateShaStorage(ctx context.Context, containerSHAs map[string]string, creds, tag string) (err error) {
	step, ctx := build.StartStep(ctx, "Update SHAs")
	defer func() { step.End(err) }()

	firestoreClient, err := establishFirestoreConnection(ctx, creds)
	if err != nil {
		err = errors.Annotate(err, "failed to initialize firestore client").Err()
		return
	}
	defer func() {
		closeErr := firestoreClient.Close()
		if closeErr != nil {
			logging.Infof(ctx, "failed to close firestore client, %w", closeErr)
		}
	}()

	collectionName := getFirestoreCollection(tag)
	firestoreItems := convertNewShasToFirestoreItems(ctx, firestoreClient, collectionName, containerSHAs)
	err = uploadContainerShas(ctx, firestoreClient, collectionName, firestoreItems)
	if err != nil {
		err = errors.Annotate(err, "failed to upload container SHAs").Err()
		return
	}

	return
}

// RevertShas swaps the previous sha with the current sha
// and updates the firestore.
func RevertShas(ctx context.Context, containerNames []string, creds, tag string) (err error) {
	firestoreClient, err := establishFirestoreConnection(ctx, creds)
	if err != nil {
		err = errors.Annotate(err, "failed to initialize firestore client").Err()
		return
	}
	defer func() {
		closeErr := firestoreClient.Close()
		if closeErr != nil {
			logging.Infof(ctx, "failed to close firestore client, %w", closeErr)
		}
	}()

	collectionName := getFirestoreCollection(tag)
	containersCollection := firestoreClient.Collection(collectionName)

	firestoreItems := []*common.FirestoreItem{}
	for _, containerName := range containerNames {
		sha, prevSha := common.FetchDigestFromFirestore(ctx, containersCollection, containerName)
		if prevSha == "" {
			logging.Infof(ctx, "no previous digest found for %s", containerName)
			continue
		}
		logging.Infof(ctx, "reverting %s: %s -> %s", containerName, sha, prevSha)
		firestoreItem := &common.FirestoreItem{
			DocName: containerName,
			Datum: map[string]string{
				"digest": prevSha,
				// In case we need to revert the revert, maintain the sha.
				"prevDigest": sha,
			},
		}

		firestoreItems = append(firestoreItems, firestoreItem)
	}

	err = uploadContainerShas(ctx, firestoreClient, collectionName, firestoreItems)
	if err != nil {
		err = errors.Annotate(err, "failed to upload container SHAs").Err()
		return
	}

	return
}

// establishFirestoreConnection connects to the firestore with
// the option to provide in a credentials file.
func establishFirestoreConnection(ctx context.Context, creds string) (client *firestore.Client, err error) {
	projectID := common.TestPlatformDataProjectID
	firestoreDatabaseName := common.TestPlatformFireStore

	clientOpts := []option.ClientOption{}

	if creds != "" {
		clientOpts = append(clientOpts, option.WithCredentialsFile(creds))
	}

	retryFunc := func() (*firestore.Client, error) {
		return common.InitClient(ctx, projectID, firestoreDatabaseName, clientOpts...)
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

// getFirestoreCollection returns the collection name
// based on whether its the prod or staging environment.
func getFirestoreCollection(tag string) string {
	if tag == common.LabelProd {
		return common.FireStoreContainersProdCollection
	}
	return common.FireStoreContainersStagingCollection
}

// convertNewShasToFirestoreItems converts the container sha map
// to a valid firestore upload item.
func convertNewShasToFirestoreItems(ctx context.Context, firestoreClient *firestore.Client, collectionName string, containerSHAs map[string]string) []*common.FirestoreItem {
	items := []*common.FirestoreItem{}
	containersCollection := firestoreClient.Collection(collectionName)

	for containerName, sha := range containerSHAs {
		currentSha, _ := common.FetchDigestFromFirestore(ctx, containersCollection, containerName)
		if sha == currentSha {
			logging.Infof(ctx, "sha did not change for %s", containerName)
			continue
		}
		firestoreItem := &common.FirestoreItem{
			DocName: containerName,
			Datum: map[string]string{
				"digest":     sha,
				"prevDigest": currentSha,
			},
		}

		items = append(items, firestoreItem)
	}

	return items
}

// uploadContainerShas writes the sha updates to the firestore.
func uploadContainerShas(ctx context.Context, firestoreClient *firestore.Client, collectionName string, items []*common.FirestoreItem) (err error) {
	containersCollection := firestoreClient.Collection(collectionName)

	writeJobResults, err := common.BatchSet(ctx, containersCollection, firestoreClient, items)
	if err != nil {
		err = errors.Annotate(err, "failed to upload container SHAs").Err()
		return
	}

	for _, job := range writeJobResults {
		if _, jobErr := job.Results(); jobErr != nil {
			err = errors.Append(err, jobErr)
		}
	}

	return
}
