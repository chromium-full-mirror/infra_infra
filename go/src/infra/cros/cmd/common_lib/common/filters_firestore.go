// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/option"

	buildapi "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
)

// ContainerInfoItem represents an Uprev record for
// containers built during this program's execution.
type ContainerInfoItem struct {
	TimeRecord         time.Time
	RepositoryHostname string
	RepositoryProject  string
	Digest             string
	ContainerName      string
	BinaryName         string
}

func NewContainerInfoItem(host, project, digest, name, binary string) *ContainerInfoItem {
	if host == "" {
		host = DefaultDockerHost
	}
	if project == "" {
		project = DefaultDockerProject
	}
	return &ContainerInfoItem{
		TimeRecord:         time.Now().UTC(),
		RepositoryHostname: host,
		RepositoryProject:  project,
		Digest:             digest,
		ContainerName:      name,
		BinaryName:         binary,
	}
}

func FetchContainerInfoFromFirestore(ctx context.Context, firestoreDatabaseName, tag, name string, clientOpts ...option.ClientOption) (containerInfo *api.ContainerInfo, err error) {
	firestoreClient, err := EstablishFirestoreConnection(ctx, firestoreDatabaseName, clientOpts...)
	if err != nil {
		err = errors.Annotate(err, "failed to initialize firestore client").Err()
		return
	}
	defer func() {
		if closeErr := firestoreClient.Close(); closeErr != nil {
			logging.Infof(ctx, "failed to close firestore client, %s", &closeErr)
		}
	}()

	collectionName := GetFirestoreCollection(tag)
	collection := firestoreClient.Collection(collectionName)
	containerInfo, err = fetchContainerInfoFromFirestoreCollection(ctx, collection, name)

	return
}

func FetchFilterFromFirestore(ctx context.Context, firestoreDatabaseName, tag, name string, clientOpts ...option.ClientOption) (filter *api.CTPFilter, err error) {
	containerInfo, err := FetchContainerInfoFromFirestore(ctx, firestoreDatabaseName, tag, name, clientOpts...)
	filter = &api.CTPFilter{
		ContainerInfo: containerInfo,
	}

	return
}

// FetchContainerInfoFromFirestoreDoc grabs the ContainerInfoItems
// from the provided firestore document reference.
func FetchContainerInfoFromFirestoreDoc(ctx context.Context, docRef *firestore.DocumentRef) []*ContainerInfoItem {
	items := []*ContainerInfoItem{}

	doc, err := docRef.Get(ctx)
	if err != nil {
		logging.Infof(ctx, "failed to get document %s, %s", docRef.ID, err)
		return items
	}
	data := doc.Data()
	jsonStr, ok := data["info"].(string)
	if !ok {
		return items
	}
	err = json.Unmarshal([]byte(jsonStr), &items)
	if err != nil {
		logging.Infof(ctx, "failed to unmarshal %s, %s", docRef.ID, err)
		return items
	}

	return items
}

// GetFirestoreCollection returns the collection name
// based on whether its the prod or staging environment.
func GetFirestoreCollection(tag string) string {
	if tag == LabelProd {
		return FireStoreContainersProdCollection
	}
	return FireStoreContainersStagingCollection
}

func buildContainerInfoFromDocumentRef(ctx context.Context, documentRef *firestore.DocumentRef) (containerInfo *api.ContainerInfo, err error) {
	containerInfos := FetchContainerInfoFromFirestoreDoc(ctx, documentRef)
	if len(containerInfos) == 0 {
		err = fmt.Errorf("%s is missing container info", documentRef.ID)
		return
	}
	// Grab most recent.
	// Convert to CTPFilter.
	containerInfoItem := containerInfos[0]
	containerName := containerInfoItem.ContainerName
	if containerName == "" {
		containerName = documentRef.ID
	}
	containerInfo = &api.ContainerInfo{
		Container: &buildapi.ContainerImageInfo{
			Name:   containerName,
			Digest: containerInfoItem.Digest,
			Repository: &buildapi.GcrRepository{
				Hostname: containerInfoItem.RepositoryHostname,
				Project:  containerInfoItem.RepositoryProject,
			},
		},
		BinaryName: containerInfoItem.BinaryName,
	}
	return
}

// fetchContainerInfoFromFirestoreCollection grabs a single filter
// from the provided firestore collection reference.
func fetchContainerInfoFromFirestoreCollection(ctx context.Context, collection *firestore.CollectionRef, containerName string) (containerInfo *api.ContainerInfo, err error) {
	documentRef := collection.Doc(containerName)
	containerInfo, err = buildContainerInfoFromDocumentRef(ctx, documentRef)
	defer func() {
		if err != nil && collection != nil {
			// Print the list of available containers if we fail.
			iter := collection.DocumentRefs(ctx)
			if docs, allErr := iter.GetAll(); allErr != nil {
				logging.Infof(ctx, "Failed to list available containers due to error: %s", allErr)
			} else {
				logging.Infof(ctx, "Available containers under the connection:")
				for _, doc := range docs {
					logging.Infof(ctx, "Container: %s", doc.ID)
				}
			}
		}
	}()
	return
}
