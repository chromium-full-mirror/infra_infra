// Copyright 2022 The ChromiumOS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"time"

	cloudBQ "cloud.google.com/go/bigquery"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	"go.chromium.org/infra/cros/karte/internal/identifiers"
)

// actionRangePersistOptions is a structure that can be used to manage an attempt to persist a range of actions.
type actionRangePersistOptions struct {
	// startID is a structural representation of earliest Karte ID to persist to BigQuery.
	startID time.Time
	// stopID is a structural representation of the latest Karte ID to persist to BigQuery.
	stopID time.Time
	// bq is the client that we use to add ValueSavers to BigQuery tables.
	bq bqPersister
}

// run gathers up all the observations and actions and persists them.
func persistActionRangeImpl(ctx context.Context, a *actionRangePersistOptions) (int, error) {
	q, err := makeQuery(a)
	if err != nil {
		return 0, errors.Annotate(err, "run").Err()
	}
	_, tally, err := persistActions(ctx, a, q.Query)
	if err != nil {
		return 0, errors.Annotate(err, "run").Err()
	}
	if err := persistObservations(ctx, a); err != nil {
		return 0, errors.Annotate(err, "run").Err()
	}
	return tally, nil
}

// makeQuery makes a query that queries a range of actions.
func makeQuery(a *actionRangePersistOptions) (*ActionEntitiesQuery, error) {
	switch {
	case a.startID.Before(a.stopID):
		// Do nothing. makeQuery was called correctly, the stop time is strictly after the start time.
	case a.startID.Equal(a.stopID):
		return nil, errors.Reason("make query: rejecting likely erroneous call: start time and stop time are the same %q", a.startID.String()).Err()
	default:
		return nil, errors.Reason("make query: invalid range %q to %q", a.startID.String(), a.stopID.String()).Err()
	}
	q, err := newActionNameRangeQuery(a.startID, a.stopID)
	if err != nil {
		return nil, errors.Annotate(err, "make query").Err()
	}
	return q, nil
}

// insertBatch inserts a batch of actions into BigQuery.
func insertBatch(ctx context.Context, a *actionRangePersistOptions, ents []*ActionEntity) error {
	if len(ents) == 0 {
		return nil
	}
	valueSavers := make([]cloudBQ.ValueSaver, 0, len(ents))
	// This conversion right here, in a perfect world, would not be necessary.
	// "Next" should just return an array of valuesavers, but that is a problem for another day.
	for _, ent := range ents {
		valueSavers = append(valueSavers, ent.ConvertToValueSaver())
	}
	f := a.bq.getInserter("entities", "actions")
	return errors.WrapIf(f(ctx, valueSavers), "insert batch")
}

// insertObservationBatch inserts a batch of observations into BigQuery.
func insertObservationBatch(ctx context.Context, a *actionRangePersistOptions, ents []*ObservationEntity) error {
	if len(ents) == 0 {
		return nil
	}
	valueSavers := make([]cloudBQ.ValueSaver, 0, len(ents))
	for _, ent := range ents {
		valueSavers = append(valueSavers, ent.ConvertToValueSaver())
	}
	f := a.bq.getInserter("entities", "observations")
	return errors.WrapIf(f(ctx, valueSavers), "insert batch")
}

// persistActions persists all the actions corresponding to our attached query to bigquery.
func persistActions(ctx context.Context, a *actionRangePersistOptions, q *datastore.Query) (*ActionQueryAncillaryData, int, error) {
	out := &ActionQueryAncillaryData{}
	tally := 0
	logging.Infof(ctx, "Persist actions: beginning offload attempt")
	var hopper []*ActionEntity
	err := datastore.Run(ctx, q, func(actionEntity *ActionEntity) error {
		if tally <= 20 || tally%1000 == 0 {
			logging.Infof(ctx, "Persist actions: offloaded %d records so far", tally)
		}
		tally++

		hopper = append(hopper, actionEntity)

		out.updateWith(&ActionQueryAncillaryData{
			BiggestID:       actionEntity.ID,
			SmallestID:      actionEntity.ID,
			BiggestVersion:  identifiers.GetIDVersion(actionEntity.ID),
			SmallestVersion: identifiers.GetIDVersion(actionEntity.ID),
		})

		if len(hopper) >= defaultBatchSize {
			insertBatch(ctx, a, hopper)
		}
		if err := insertBatch(ctx, a, hopper); err != nil {
			return errors.Annotate(err, "persist actions").Err()
		}
		hopper = nil
		return nil
	})
	if err != nil {
		return nil, 0, errors.Annotate(err, "persist actions").Err()
	}
	if err := insertBatch(ctx, a, hopper); err != nil {
		return nil, 0, errors.Annotate(err, "persist actions").Err()
	}
	logging.Infof(ctx, "Persist actions: offloaded %d records in total", tally)
	return out, tally, nil
}

// persistObservations persists all of our observations associated with the actions found in `persistActions` to bigquery.
func persistObservations(ctx context.Context, a *actionRangePersistOptions) error {
	biggestID, err := identifiers.MakeRawID(a.startID, 0)
	if err != nil {
		return errors.Annotate(err, "persist actions").Err()
	}
	smallestID, err := identifiers.MakeRawID(a.stopID, 0)
	if err != nil {
		return errors.Annotate(err, "persist actions").Err()
	}
	var hopper []*ObservationEntity
	query := datastore.NewQuery(ObservationKind).
		Gte("action_id", smallestID).
		Lte("action_id", biggestID)
	tally := 0
	rErr := datastore.Run(ctx, query, func(o *ObservationEntity) error {
		if tally <= 20 || tally%1000 == 0 {
			logging.Infof(ctx, "Persist actions: finished processing %d observations so far", tally)
		}
		tally++
		hopper = append(hopper, o)
		if len(hopper) >= defaultBatchSize {
			if err := insertObservationBatch(ctx, a, hopper); err != nil {
				return errors.Annotate(err, "offloading records").Err()
			}
			hopper = nil
		}
		return nil
	})
	logging.Errorf(ctx, "exactly %d observations processed for range [%q, %q].", tally, smallestID, biggestID)
	if rErr != nil {
		return errors.Annotate(rErr, "persisting observations").Err()
	}
	return insertObservationBatch(ctx, a, hopper)
}
