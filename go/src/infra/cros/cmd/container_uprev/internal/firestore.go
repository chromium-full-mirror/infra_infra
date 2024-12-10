// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package internal

import (
	"context"
	"encoding/json"
	"time"

	"cloud.google.com/go/firestore"

	"go.chromium.org/luci/common/errors"

	"infra/cros/cmd/common_lib/common"
)

// buildContainerInfosForFirestore converts the map of ContainerInfoItems
// into the record stored within Firestore.
func buildContainerInfosForFirestore(infos map[string][]*common.ContainerInfoItem) []*common.FirestoreItem {
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
func filterContainerInfos(infos map[string][]*common.ContainerInfoItem) map[string][]*common.ContainerInfoItem {
	filtered := map[string][]*common.ContainerInfoItem{}
	now := time.Now().UTC()

	for containerName, info := range infos {
		filteredInfo := []*common.ContainerInfoItem{}
		for i, infoItem := range info {
			// Clean out bad entries.
			if infoItem == nil {
				continue
			}
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
func pushContainerInfoToFirestore(ctx context.Context, client *firestore.Client, collectionName string, infos map[string][]*common.ContainerInfoItem) (err error) {
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
