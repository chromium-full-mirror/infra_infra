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
}

func NewContainerInfoItem(host, project, digest string) *ContainerInfoItem {
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
	}
}

// FetchFiltersFromFirestore grabs every filter stored within the
// the firestore database.
func FetchFiltersFromFirestore(ctx context.Context, creds, tag string) (filters []*api.CTPFilter, err error) {
	firestoreClient, err := EstablishFirestoreConnection(ctx, creds)
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

	collectionName := GetFirestoreCollection(tag)
	collection := firestoreClient.Collection(collectionName)
	filters, err = fetchFiltersFromFirestoreCollection(ctx, collection)

	return
}

func FetchFilterFromFirestore(ctx context.Context, creds, tag, name string) (filter *api.CTPFilter, err error) {
	firestoreClient, err := EstablishFirestoreConnection(ctx, creds)
	if err != nil {
		err = errors.Annotate(err, "failed to initialize firestore client").Err()
		return
	}
	defer func() {
		closeErr := firestoreClient.Close()
		if closeErr != nil {
			logging.Infof(ctx, "failed to close firestore client, %w", &closeErr)
		}
	}()

	collectionName := GetFirestoreCollection(tag)
	collection := firestoreClient.Collection(collectionName)
	filter, err = fetchFilterFromFirestoreCollection(ctx, collection, name)

	return
}

// FetchContainerInfoFromFirestoreDoc grabs the ContainerInfoItems
// from the provided firestore document reference.
func FetchContainerInfoFromFirestoreDoc(ctx context.Context, docRef *firestore.DocumentRef) []*ContainerInfoItem {
	items := []*ContainerInfoItem{}

	doc, err := docRef.Get(ctx)
	if err != nil {
		logging.Infof(ctx, "failed to get document %s, %w", docRef.ID, err)
		return items
	}
	data := doc.Data()
	jsonStr, ok := data["info"].(string)
	if !ok {
		return items
	}
	err = json.Unmarshal([]byte(jsonStr), &items)
	if err != nil {
		logging.Infof(ctx, "failed to unmarshal %s, %w", docRef.ID, err)
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

// fetchFiltersFromFirestoreCollection grabs every filter from the
// provided firestored collection reference.
func fetchFiltersFromFirestoreCollection(ctx context.Context, collection *firestore.CollectionRef) (filters []*api.CTPFilter, err error) {
	filters = []*api.CTPFilter{}

	documentRefs, err := collection.DocumentRefs(ctx).GetAll()
	if err != nil {
		err = errors.Annotate(err, "failed to get document refs from filter's firestore").Err()
		return
	}

	for _, documentRef := range documentRefs {
		filter, innerErr := buildCTPFilterFromDocumentRef(ctx, documentRef)
		if innerErr != nil {
			err = errors.Annotate(innerErr, "%s failed", documentRef.ID).Err()
			return
		}

		filters = append(filters, filter)
	}

	return
}

func buildCTPFilterFromDocumentRef(ctx context.Context, documentRef *firestore.DocumentRef) (filter *api.CTPFilter, err error) {
	containerInfos := FetchContainerInfoFromFirestoreDoc(ctx, documentRef)
	if len(containerInfos) == 0 {
		err = fmt.Errorf("%s is missing container info", documentRef.ID)
		return
	}
	// Grab most recent.
	// Convert to CTPFilter.
	containerInfo := containerInfos[0]
	filter = &api.CTPFilter{
		ContainerInfo: &api.ContainerInfo{
			Container: &buildapi.ContainerImageInfo{
				Name:   documentRef.ID,
				Digest: containerInfo.Digest,
				Repository: &buildapi.GcrRepository{
					Hostname: containerInfo.RepositoryHostname,
					Project:  containerInfo.RepositoryProject,
				},
			},
		},
	}
	return
}

// fetchFilterFromFirestoreCollection grabs a single filter
// from the provided firestore collection reference.
func fetchFilterFromFirestoreCollection(ctx context.Context, collection *firestore.CollectionRef, filterName string) (filter *api.CTPFilter, err error) {
	documentRef := collection.Doc(filterName)
	filter, err = buildCTPFilterFromDocumentRef(ctx, documentRef)
	return
}
