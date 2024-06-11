// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package internal

import (
	"context"

	"cloud.google.com/go/firestore"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"
)

// UpdateShaStorage connects the the firestore and uploads the SHAs produced
// during the uprev service.
func UpdateShaStorage(ctx context.Context, containerInfo map[string]*ContainerInfoItem, creds, tag string) (err error) {
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
	err = addContainerInfoToStorage(ctx, firestoreClient, collectionName, containerInfo)
	if err != nil {
		err = errors.Annotate(err, "failed to upload container info").Err()
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

	infosMap := map[string][]*ContainerInfoItem{}
	for _, containerName := range containerNames {
		currentInfos := fetchContainerInfoFromFirestore(ctx, containersCollection, containerName)
		// Can't revert the only record.
		if len(currentInfos) <= 1 {
			continue
		}
		infosMap[containerName] = currentInfos[1:]
	}

	err = pushContainerInfoToFirestore(ctx, firestoreClient, collectionName, infosMap)
	return
}

func addContainerInfoToStorage(ctx context.Context, firestoreClient *firestore.Client, collectionName string, containerInfos map[string]*ContainerInfoItem) (err error) {
	containersCollection := firestoreClient.Collection(collectionName)

	infosMap := map[string][]*ContainerInfoItem{}
	// Add new container info to storage record.
	for containerName, containerInfo := range containerInfos {
		currentInfos := fetchContainerInfoFromFirestore(ctx, containersCollection, containerName)
		infos := append([]*ContainerInfoItem{containerInfo}, currentInfos...)
		infosMap[containerName] = infos
	}

	err = pushContainerInfoToFirestore(ctx, firestoreClient, collectionName, infosMap)
	return
}
