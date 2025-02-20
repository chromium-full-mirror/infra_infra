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

	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

type ContainerInfosMap = map[string]*common.ContainerInfoItem

// UpdateShaStorage connects the the firestore and uploads the SHAs produced
// during the uprev service.
func UpdateShaStorage(ctx context.Context, firestoreDatabaseName string, containerInfo ContainerInfosMap, creds, tag string) (err error) {
	step, ctx := build.StartStep(ctx, "Update SHAs")
	defer func() { step.End(err) }()

	firestoreClient, err := common.EstablishFirestoreConnection(ctx, firestoreDatabaseName, creds)
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

	collectionName := common.GetFirestoreCollection(tag)
	err = addContainerInfoToStorage(ctx, firestoreClient, collectionName, containerInfo)
	if err != nil {
		err = errors.Annotate(err, "failed to upload container info").Err()
		return
	}

	return
}

// RevertShas swaps the previous sha with the current sha
// and updates the firestore.
func RevertShas(ctx context.Context, containerNames []string, firestoreDatabaseName, creds, tag string) (err error) {
	firestoreClient, err := common.EstablishFirestoreConnection(ctx, firestoreDatabaseName, creds)
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

	collectionName := common.GetFirestoreCollection(tag)
	containersCollection := firestoreClient.Collection(collectionName)

	infosMap := map[string][]*common.ContainerInfoItem{}
	for _, containerName := range containerNames {
		currentInfos := common.FetchContainerInfoFromFirestoreDoc(ctx, containersCollection.Doc(containerName))
		// Can't revert the only record.
		if len(currentInfos) <= 1 {
			continue
		}
		infosMap[containerName] = currentInfos[1:]
	}

	err = pushContainerInfoToFirestore(ctx, firestoreClient, collectionName, infosMap)
	return
}

func addContainerInfoToStorage(ctx context.Context, firestoreClient *firestore.Client, collectionName string, containerInfos map[string]*common.ContainerInfoItem) (err error) {
	containersCollection := firestoreClient.Collection(collectionName)

	infosMap := map[string][]*common.ContainerInfoItem{}
	// Add new container info to storage record.
	for containerName, containerInfo := range containerInfos {
		currentInfos := common.FetchContainerInfoFromFirestoreDoc(ctx, containersCollection.Doc(containerName))
		infos := append([]*common.ContainerInfoItem{containerInfo}, currentInfos...)
		infosMap[containerName] = infos
	}

	err = pushContainerInfoToFirestore(ctx, firestoreClient, collectionName, infosMap)
	return
}
