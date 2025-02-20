// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package karte

import (
	"context"
	"net/http"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"

	kartepb "go.chromium.org/infra/cros/karte/api"
	kclient "go.chromium.org/infra/cros/karte/client"
	"go.chromium.org/infra/cros/recovery/logger/metrics"
)

// Client is a wrapped Karte client that exposes only the metrics.Metrics interface.
type client struct {
	impl kartepb.KarteClient
}

// NewMetrics creates a new metrics client.
func NewMetrics(ctx context.Context, c *kclient.Config, o ...kclient.Option) (metrics.Metrics, error) {
	innerClient, err := kclient.NewClient(ctx, c, o...)
	if err != nil {
		return nil, errors.Annotate(err, "wrap karte client").Err()
	}
	return &client{impl: innerClient}, nil
}

// NewMetricsWithHttp creates a new Karte client interface.
//
// It's used for creating karte interface with a http client,
// not for local command line tool.
func NewMetricsWithHttp(ctx context.Context, hc *http.Client, hostname string, opt *prpc.Options) (metrics.Metrics, error) {
	if hc == nil {
		return nil, errors.Reason("new karte client: hc cannot be nil").Err()
	}
	if hostname == "" {
		return nil, errors.Reason("new karte client: hostname cannot be empty").Err()
	}
	return &client{impl: kartepb.NewKartePRPCClient(&prpc.Client{
		C:       hc,
		Host:    hostname,
		Options: opt,
	})}, nil
}

// Create creates a new action in Karte using the following process.
//
// 1. Create action in Karte.
// 2. Record name on action argument by mutating it.
// 3. Walking the observations and creating them as well.
func (c *client) Create(ctx context.Context, action *metrics.Action) error {
	if c == nil {
		return errors.Reason("create: client cannot be nil").Err()
	}
	karteResp, err := c.impl.CreateAction(
		ctx,
		&kartepb.CreateActionRequest{
			Action: convertActionToKarteAction(action),
		},
	)
	if err != nil {
		return errors.Annotate(err, "create").Err()
	}
	action.Name = karteResp.GetName()
	err = c.createObservations(ctx, action.Name, action.Observations...)
	return errors.Annotate(err, "create").Err()
}

// createObservations creates observations in karte.
func (c *client) createObservations(ctx context.Context, actionName string, observations ...*metrics.Observation) error {
	if actionName == "" {
		return errors.Reason("actionName cannot be empty").Err()
	}
	for _, observation := range observations {
		obs, err := makeKarteObservation(actionName, observation)
		if err != nil {
			return errors.Annotate(err, "create observation").Err()
		}
		if _, err := c.impl.CreateObservation(
			ctx,
			&kartepb.CreateObservationRequest{
				Observation: obs,
			},
		); err != nil {
			return errors.Annotate(err, "create observation").Err()
		}
	}
	return nil
}

// defaultResultSetSize is the number of records to return by default from Karte.
const defaultResultSetSize = 1000

// Search takes a query struct and produces a resultset.
func (c *client) Search(ctx context.Context, q *metrics.Query) (*metrics.QueryResult, error) {
	filter, lErr := q.Lower()
	if lErr != nil {
		return nil, errors.Annotate(lErr, "karte search").Err()
	}
	pageSize := q.Limit
	if pageSize <= 0 {
		pageSize = defaultResultSetSize
	}
	karteResp, kErr := c.impl.ListActions(
		ctx,
		&kartepb.ListActionsRequest{
			PageSize: defaultResultSetSize,
			// We explicitly do not set a page token so that we get
			// the most recent results first.
			PageToken: "",
			Filter:    filter,
		},
	)
	if kErr != nil {
		return nil, errors.Annotate(kErr, "karte search").Err()
	}

	var actions []*metrics.Action
	for _, a := range karteResp.GetActions() {
		action := convertKarteActionToAction(a)
		if action != nil {
			actions = append(actions, action)
		}
	}

	return &metrics.QueryResult{
		PageToken: karteResp.GetNextPageToken(),
		Actions:   actions,
	}, nil
}
