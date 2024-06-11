// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package internal

import (
	"context"
	"encoding/json"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/cenkalti/backoff/v4"
	"google.golang.org/api/option"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"infra/cros/cmd/common_lib/common"
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
		host = common.DefaultDockerHost
	}
	if project == "" {
		project = common.DefaultDockerProject
	}
	return &ContainerInfoItem{
		TimeRecord:         time.Now().UTC(),
		RepositoryHostname: host,
		RepositoryProject:  project,
		Digest:             digest,
	}
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

// fetchContainerInfoFromFirestore grabs the data from the collection
// and Unmarshals it into a list of ContainerInfoItems.
func fetchContainerInfoFromFirestore(ctx context.Context, collection *firestore.CollectionRef, docName string) []*ContainerInfoItem {
	items := []*ContainerInfoItem{}

	doc, err := collection.Doc(docName).Get(ctx)
	if err != nil {
		return items
	}
	data := doc.Data()
	jsonStr, ok := data["info"].(string)
	if !ok {
		return items
	}
	err = json.Unmarshal([]byte(jsonStr), &items)
	if err != nil {
		return items
	}

	return items
}

// buildContainerInfosForFirestore converts the map of ContainerInfoItems
// into the record stored within Firestore.
func buildContainerInfosForFirestore(infos map[string][]*ContainerInfoItem) []*common.FirestoreItem {
	items := []*common.FirestoreItem{}

	for containerName, info := range infos {
		jsonBytes, err := json.Marshal(info)
		if err != nil {
			continue
		}
		firestoreItem := &common.FirestoreItem{
			DocName: containerName,
			Datum: map[string]string{
				"info": string(jsonBytes),
			},
		}

		items = append(items, firestoreItem)
	}

	return items
}

// filterContainerInfos filters out stale records.
func filterContainerInfos(infos map[string][]*ContainerInfoItem) map[string][]*ContainerInfoItem {
	filtered := map[string][]*ContainerInfoItem{}
	now := time.Now().UTC()

	for containerName, info := range infos {
		filteredInfo := []*ContainerInfoItem{}
		for i, infoItem := range info {
			// Keep at least 5 entries, filter out the rest.
			if i >= 5 {
				compareTime := infoItem.TimeRecord.AddDate(0, 0, 21)
				// If 21 days since time record, filter out.
				if compareTime.Compare(now) == -1 {
					continue
				}
			}

			filteredInfo = append(filteredInfo, infoItem)
		}
		filtered[containerName] = filteredInfo
	}

	return filtered
}

// pushContainerInfoToFirestore packages the map of ContainerInfoItems
// and pushes them into to Firestore collection.
func pushContainerInfoToFirestore(ctx context.Context, client *firestore.Client, collectionName string, infos map[string][]*ContainerInfoItem) (err error) {
	collection := client.Collection(collectionName)

	filteredInfos := filterContainerInfos(infos)
	items := buildContainerInfosForFirestore(filteredInfos)

	writeJobResults, err := common.BatchSet(ctx, collection, client, items)
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
