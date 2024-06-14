// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

const saProject = "chromeos-bot"
const saFile = "/creds/service_accounts/service-account-chromeos.json"

func queryForBuilds(log *log.Logger) (*bigquery.RowIterator, error) {
	ctx := context.Background()

	c, err := bigquery.NewClient(ctx, saProject,
		option.WithCredentialsFile(saFile))
	defer c.Close()

	if err != nil {
		return nil, fmt.Errorf("unable to make bq client %s", err)
	}

	b, err := os.ReadFile("/usr/bin/q.txt")
	if err != nil {
		b, err = os.ReadFile("q.txt")
		if err != nil {
			return nil, fmt.Errorf("unable to read file: %s", err)

		}
	}

	cmd := string(b)
	bqQ := c.Query(cmd)

	iter, err := bqQ.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to make Bigquery call: %s", err)
	}

	return iter, nil
}

type resSchema struct {
	Board     string
	Milestone int64
	Platform  string
}

func iterThroughData(bqIter *bigquery.RowIterator, log *log.Logger) []resSchema {
	var data []resSchema
	for {

		var resp resSchema
		err := bqIter.Next(&resp)
		if err == iterator.Done { // from "google.golang.org/api/iterator"
			break
		}
		if err != nil {
			// Intentionally do not rause this error, just log it for now.
			// Sometimes something silly can happen and we get `NULL` from the query for some items,
			// this is where its going to be raised. For now, lets log && continue to not break every result.
			log.Println("INFORMATIONAL ERR - could not decode error on loop: \n", err)
			continue
		}
		data = append(data, resp)
	}

	return data
}
