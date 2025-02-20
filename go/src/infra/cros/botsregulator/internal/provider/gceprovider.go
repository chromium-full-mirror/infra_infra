// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package provider

import (
	"context"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	gcepAPI "go.chromium.org/luci/gce/api/config/v1"

	"go.chromium.org/infra/cros/botsregulator/internal/clients"
)

// gcepProvider is the GCE Provider implementation of the Provider interface.
type gcepProvider struct {
	gc clients.GCEPClient
}

// NewGCEPClient returns a new gcepClient instance.
func NewGCEPClient(ctx context.Context, host string) (*gcepProvider, error) {
	gc, err := clients.NewGCEPClient(ctx, host)
	if err != nil {
		return nil, err
	}
	return &gcepProvider{
		gc: gc,
	}, nil
}

// get gets GCE Provider specified config.
func (g *gcepProvider) get(ctx context.Context, configID string) (*gcepAPI.Config, error) {
	res, err := g.gc.Get(ctx, &gcepAPI.GetRequest{
		Id: configID,
	})
	if err != nil {
		return nil, errors.Annotate(err, "could not GET the config: %s", configID).Err()
	}
	return res, nil
}

// update updates GCE Provider specified config.
func (g *gcepProvider) update(ctx context.Context, cf *gcepAPI.Config, cfID string) error {
	_, err := g.gc.Update(ctx, &gcepAPI.UpdateRequest{
		Id:     cfID,
		Config: cf,
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: []string{"config.duts"},
		},
	})
	if err != nil {
		return errors.Annotate(err, "could not UPDATE the config: %s", cf).Err()
	}
	return nil
}

// UpdateConfig is called as BPI.UpdateConfig and
// is responsible for orchestrating the config update.
func (g *gcepProvider) UpdateConfig(ctx context.Context, hns []string, cfID string) error {
	logging.Infof(ctx, "updateConfig: starting GCEP flow for duts: %v", hns)
	cf, err := g.get(ctx, cfID)
	if err != nil {
		return err
	}
	logging.Infof(ctx, "updateConfig: retrieved Config: %v", cf)

	cf.Duts = newStringSet(hns)
	logging.Infof(ctx, "updateConfig: config.Duts pre-update: %v", cf.Duts)

	err = g.update(ctx, cf, cfID)
	if err != nil {
		return err
	}
	logging.Infof(ctx, "updateConfig: done for prefix: %s", cf.Prefix)
	return nil
}
