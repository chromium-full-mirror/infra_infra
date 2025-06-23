// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/grpcutil"
	"go.chromium.org/luci/server/cron"

	kartepb "go.chromium.org/infra/cros/karte/api"
	"go.chromium.org/infra/cros/karte/internal/scalars"
)

// karteFrontend is the implementation of kartepb.KarteServer
// used in the application.
type karteFrontend struct {
	projectID string
}

// KarteFrontend is a combination of the Karte RPCs and the cron RPCs.
// In the future, any other services exposes by Karte should also be added here.
type KarteFrontend interface {
	kartepb.KarteServer
	kartepb.KarteCronServer
	GetProjectID() string
}

// NewKarteFrontend produces a new Karte frontend.
func NewKarteFrontend(projectID string) KarteFrontend {
	return &karteFrontend{
		projectID: projectID,
	}
}

// GetProjectID gets the project ID for the Karte frontend.
func (k *karteFrontend) GetProjectID() string {
	return k.projectID
}

// ListActions lists the actions that Karte knows about.
func (k *karteFrontend) ListActions(ctx context.Context, req *kartepb.ListActionsRequest) (*kartepb.ListActionsResponse, error) {
	q, err := newActionEntitiesQuery(req.GetPageToken(), req.GetFilter())
	if err != nil {
		return nil, errors.Annotate(err, "list actions").Err()
	}

	es, _, err := q.Next(ctx, req.GetPageSize())
	if err != nil {
		return nil, errors.Annotate(err, "list actions (page size: %d)", req.GetPageSize()).Err()
	}
	var actions []*kartepb.Action
	for _, e := range es {
		actions = append(actions, e.ConvertToAction())
	}
	return &kartepb.ListActionsResponse{
		Actions:       actions,
		NextPageToken: q.Token,
	}, nil
}

// ListObservations lists the observations that Karte knows about.
func (k *karteFrontend) ListObservations(ctx context.Context, req *kartepb.ListObservationsRequest) (*kartepb.ListObservationsResponse, error) {
	q, err := newObservationEntitiesQuery(req.GetPageToken(), req.GetFilter())
	if err != nil {
		return nil, errors.Annotate(err, "list observations").Err()
	}
	es, err := q.Next(ctx, req.GetPageSize())
	if err != nil {
		return nil, errors.Annotate(err, "list observations (page size: %d)", req.GetPageSize()).Err()
	}
	var observations []*kartepb.Observation
	for _, e := range es {
		observations = append(observations, e.ConvertToObservation())
	}
	return &kartepb.ListObservationsResponse{
		Observations:  observations,
		NextPageToken: q.Token,
	}, nil
}

// PersistToBigquery persists all Karte-tracked records in a given time range to BigQuery.
// This is a cron method and part of the cron group of API calls.
// It is intentionally EXACTLY equivalent to calling the non-cron API persist-action-range.
//
// By default, we take the current time, truncate it to the nearest hour, and set that to the END time.
// We then take one hour before the end time and make that the start time.
func (k *karteFrontend) PersistToBigquery(ctx context.Context, req *kartepb.PersistToBigqueryRequest) (*kartepb.PersistToBigqueryResponse, error) {
	intervalStart, intervalEnd, err := makeAlignedIntervalStrictlyInPast(time.Now().UTC(), 10*time.Minute)
	if err != nil {
		return nil, errors.Annotate(err, "persist to bigquery").Err()
	}
	resp, err := k.PersistActionRange(
		ctx,
		&kartepb.PersistActionRangeRequest{
			StartTime: scalars.ConvertTimeToTimestampPtr(intervalStart),
			StopTime:  scalars.ConvertTimeToTimestampPtr(intervalEnd),
		},
	)
	if err != nil {
		return nil, status.Errorf(codes.Aborted, "persist to bigquery failed: %s", err)
	}
	return &kartepb.PersistToBigqueryResponse{
		Succeeded:      true,
		CreatedActions: resp.GetCreatedRecords(),
	}, nil
}

// InstallServices takes a Karte frontend and exposes it to a LUCI prpc.Server.
func InstallServices(k KarteFrontend, srv grpc.ServiceRegistrar) {
	kartepb.RegisterKarteServer(srv, k)
	kartepb.RegisterKarteCronServer(srv, k)
	cron.RegisterHandler(
		"persist-to-bq",
		func(ctx context.Context) error {
			_, err := k.PersistToBigquery(
				ctx,
				&kartepb.PersistToBigqueryRequest{},
			)
			err = grpcutil.WrapIfTransient(err)
			err = errors.WrapIf(err, "persist to bq cron")
			return err
		},
	)
}

// makeAlignedIntervalStrictlyInPast returns an interval of duration d
// that is strictly before the instant t.
func makeAlignedIntervalStrictlyInPast(t time.Time, d time.Duration) (time.Time, time.Time, error) {
	var zero time.Time
	if ok := t.Location() == time.UTC; !ok {
		return zero, zero, errors.Reason("aligned interval strictly in past: unexpected location %q", t.Location().String()).Err()
	}
	endTime := t.Truncate(d).UTC()
	startTime := endTime.Add(-d).UTC()
	return startTime, endTime, nil
}
