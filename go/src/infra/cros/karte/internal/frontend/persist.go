// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	cloudBQ "cloud.google.com/go/bigquery"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	kartepb "go.chromium.org/infra/cros/karte/api"
	"go.chromium.org/infra/cros/karte/internal/externalclients"
	"go.chromium.org/infra/cros/karte/internal/scalars"
)

// PersistAction persists a single action.
func (k *karteFrontend) PersistAction(ctx context.Context, req *kartepb.PersistActionRequest) (*kartepb.PersistActionResponse, error) {
	client := externalclients.GetBQ(ctx)
	id := req.GetActionId()
	if id == "" {
		return nil, status.Errorf(codes.InvalidArgument, "persist action: request ID cannot be empty")
	}
	ent := ActionEntity{}
	ent.ID = id
	if err := datastore.Get(ctx, &ent); err != nil {
		logging.Errorf(ctx, "Cannot retrieve action: %s", err)
		return nil, errors.Annotate(err, "persist action").Err()
	}
	valueSaver := ent.ConvertToValueSaver()
	logging.Infof(ctx, "beginning to insert record to bigquery")
	if err := client.Put(ctx, k.GetProjectID(), "entities", "actions", []cloudBQ.ValueSaver{valueSaver}); err != nil {
		logging.Errorf(ctx, "cannot insert action: %s", err)
		return nil, status.Errorf(codes.Aborted, "error persisting single record: %s", err)
	}

	return &kartepb.PersistActionResponse{
		Succeeded:     true,
		CreatedRecord: true,
	}, nil
}

// persistBqClient is a wrapper around the bigquery client that exposes only the interface necessary to persist to
// persist ranges of actions.
type persistBqClient struct {
	client *cloudBQ.Client
}

// bqInserter persists ranges of actions to BigQuery.
type bqInserter = func(context.Context, []cloudBQ.ValueSaver) error

// getInserter gets the inserter for a table in a dataset.
func (c persistBqClient) getInserter(dataset string, table string) bqInserter {
	return func(ctx context.Context, valueSavers []cloudBQ.ValueSaver) error {
		return c.client.Dataset(dataset).Table(table).Inserter().Put(ctx, valueSavers)
	}
}

// PersistActionRange persists a range of actions.
func (k *karteFrontend) PersistActionRange(ctx context.Context, req *kartepb.PersistActionRangeRequest) (*kartepb.PersistActionRangeResponse, error) {
	logging.Infof(ctx, "Persisting action range %v to bigquery.", req)
	client, err := cloudBQ.NewClient(ctx, cloudBQ.DetectProjectID)
	if err != nil {
		logging.Errorf(ctx, "Cannot create bigquery client: %s", err)
		return nil, status.Errorf(codes.Aborted, "persist action range: cannot create bigquery client: %s", err)
	}

	return k.persistActionRangeImpl(ctx, persistBqClient{client}, req)
}

// bqPersister is the subset of the BigQuery interface used by the implementation of persistAction.
type bqPersister interface {
	getInserter(dataset string, table string) bqInserter
}

// timeRanges is the number of splits that we divide a time range into before persisting it to bigquery.
const timeRanges = 10

// persistActionRangeImpl is the implementation of persist range action.
func (*karteFrontend) persistActionRangeImpl(ctx context.Context, client bqPersister, req *kartepb.PersistActionRangeRequest) (*kartepb.PersistActionRangeResponse, error) {
	if req.GetStartVersion() != "" && req.GetStartVersion() != "zzzz" {
		return nil, errors.Reason("unsupported version %q", req.GetStartVersion()).Err()
	}
	if req.GetStopVersion() != "" && req.GetStopVersion() != "zzzz" {
		return nil, errors.Reason("unsupported version %q", req.GetStopVersion()).Err()
	}

	start := req.GetStartTime()
	stop := req.GetStopTime()
	times, err := splitTimeRange(scalars.ConvertTimestampPtrToTime(start), scalars.ConvertTimestampPtrToTime(stop), timeRanges)
	if err != nil {
		return nil, errors.Annotate(err, "persist action range impl").Err()
	}

	var createdRecords = int32(0)
	var wg sync.WaitGroup
	var merr errors.MultiError
	var merrMutex sync.Mutex
	for _, t := range times {
		wg.Add(1)
		go func(t timeRangePair) {
			defer wg.Done()
			tally, err := persistActionRangeImpl(ctx, &actionRangePersistOptions{
				startID: t.start,
				stopID:  t.stop,
				bq:      client,
			})
			if err != nil {
				merrMutex.Lock()
				defer merrMutex.Unlock()
				merr = append(merr, errors.Annotate(err, "persist action range impl").Err())
				return
			}
			atomic.AddInt32(&createdRecords, int32(tally))
		}(t)
	}
	wg.Wait()
	if len(merr) != 0 {
		return nil, merr
	}

	return &kartepb.PersistActionRangeResponse{
		Succeeded:      true,
		CreatedRecords: createdRecords,
	}, nil
}

// timeRangePair consists of a starting point and an ending point.
type timeRangePair struct {
	start time.Time
	stop  time.Time
}

// splitTimeRange takes a time range and a number of subranges to split it into and splits it.
func splitTimeRange(start time.Time, stop time.Time, entries int) ([]timeRangePair, error) {
	var out []timeRangePair
	duration, err := validateSplitTimeRange(start, stop, entries)
	if err != nil {
		return nil, err
	}

	for i := range entries {
		curStart := start.Add(scaleNanoseconds(duration.Nanoseconds(), float64(i)/float64(entries)))
		curStop := start.Add(scaleNanoseconds(duration.Nanoseconds(), float64(i+1)/float64(entries)))
		if i == entries-1 {
			curStop = stop
		}
		out = append(out, timeRangePair{start: curStart, stop: curStop})
	}

	return out, nil
}

func scaleNanoseconds(nanoseconds int64, scale float64) time.Duration {
	return time.Duration(float64(nanoseconds) * scale)
}

func validateSplitTimeRange(start time.Time, stop time.Time, entries int) (time.Duration, error) {
	switch {
	case start.Location() != time.UTC:
		return 0, errors.Reason("split time range: start time must be UTC not %q", start.Location().String()).Err()
	case stop.Location() != time.UTC:
		return 0, errors.Reason("split time range: stop time must be UTC not %q", start.Location().String()).Err()
	case start.Equal(stop):
		return 0, errors.Reason("split time range: start time cannot equal stop time %q", start.String()).Err()
	case start.After(stop):
		return 0, errors.Reason("split time range: start time cannot occur after stop %q", stop.String()).Err()
	case entries <= 0:
		return 0, errors.Reason("split time range: invalid number of entries %d", entries).Err()
	}
	duration := stop.Sub(start)
	switch {
	case duration.Microseconds() < 1:
		return 0, errors.Reason("split time range: start %q and stop %q must differ by at least one microsecond", start.String(), stop.String()).Err()
	case duration.Hours() > 30*60:
		return 0, errors.Reason("split time range: start %q and stop %q must differ by at most 30 days", start.String(), stop.String()).Err()
	}
	return duration, nil
}
