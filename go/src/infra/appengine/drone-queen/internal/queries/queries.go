// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package queries contains convenient datastore queries.
package queries

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	"go.chromium.org/infra/appengine/drone-queen/api"
	"go.chromium.org/infra/appengine/drone-queen/internal/config"
	"go.chromium.org/infra/appengine/drone-queen/internal/entities"
	"go.chromium.org/infra/libs/otil"
)

// Name used for OpenTelemetry tracers.
const tname = "go.chromium.org/infra/appengine/drone-queen/internal/queries"

// CreateNewDrone creates a new Drone datastore entity with a unique ID.
// This function cannot be called in a transaction.
func CreateNewDrone(ctx context.Context, now time.Time) (_ entities.DroneID, err error) {
	ctx, span := otil.FuncSpan(ctx)
	defer func() { otil.EndSpan(span, err) }()
	return createNewDrone(ctx, now, func() string { return uuid.New().String() })
}

// createNewDrone creates a new Drone datastore entity with a unique
// ID.  An ID generator function must be provided.  This function
// cannot be called in a transaction.
func createNewDrone(ctx context.Context, now time.Time, generator func() string) (entities.DroneID, error) {
	const maxAttempts = 10
	var id entities.DroneID
	retry := errors.New("retry")
	for i := 1; ; i++ {
		f := func(ctx context.Context) error {
			proposed := generator()
			key := datastore.MakeKey(ctx, entities.DroneKind, proposed)
			res, err := datastore.Exists(ctx, key)
			if err != nil {
				return errors.Fmt("check if drone %s exists: %w", id, err)
			}
			if res.Any() {
				if i == maxAttempts {
					return errors.New("max attempts finding unique ID")
				}
				return retry
			}
			id = entities.DroneID(proposed)
			drone := entities.Drone{
				ID:         id,
				Expiration: now.Add(config.AssignmentDuration(ctx)).UTC(),
			}
			if err := datastore.Put(ctx, &drone); err != nil {
				return err
			}
			return nil
		}
		if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
			if err == retry {
				retryUniqueUUID.Add(ctx, 1, config.Instance(ctx))
				continue
			}
			return "", errors.Fmt("create new drone: %w", err)
		}
		return id, nil
	}
}

// getDroneDUTs gets the DUTs assigned to a drone.  This does not have
// to be run in a transaction, but caveat emptor.
func getDroneDUTs(ctx context.Context, d entities.DroneID) ([]*entities.DUT, error) {
	q := datastore.NewQuery(entities.DUTKind)
	q = q.Eq(entities.AssignedDroneField, d)
	q = q.Ancestor(entities.DUTGroupKey(ctx))
	var duts []*entities.DUT
	if err := datastore.GetAll(ctx, q, &duts); err != nil {
		return nil, errors.Fmt("get drone %v DUTs:: %w", d, err)
	}
	return duts, nil
}

// getUnassignedDUTs gets at most the specified number of unassigned
// DUTs.  Draining DUTs are ignored.  This does not have to be run in
// a transaction, but caveat emptor.  If n is less than zero, return
// no DUTs. hive is the zone/hive the DUT/Drone belongs to.
func getUnassignedDUTs(ctx context.Context, n int32, hive string) (_ []*entities.DUT, err error) {
	ctx, span := otil.FuncSpan(ctx)
	defer func() { otil.EndSpan(span, err) }()
	otil.AddValues(span, n, hive)
	if n < 0 {
		return nil, nil
	}
	q := datastore.NewQuery(entities.DUTKind)
	q = q.Eq(entities.AssignedDroneField, "")
	q = q.Eq(entities.DrainingField, false)
	q = q.Eq(entities.HiveField, hive)
	q = q.Ancestor(entities.DUTGroupKey(ctx))
	q = q.Limit(n)
	var duts []*entities.DUT
	if err := datastore.GetAll(ctx, q, &duts); err != nil {
		return nil, errors.Fmt("get %v unassigned DUTs: %w", n, err)
	}
	return duts, nil
}

// AssignNewDUTs assigns new DUTs to the drone according to its load
// indicators and current DUTs.  Returns the list of all DUTs assigned
// to the drone. hive is the zone/hive the DUT/Drone belongs to.
//
// This function needs to be run within a datastore transaction.
func AssignNewDUTs(ctx context.Context, d entities.DroneID, li *api.ReportDroneRequest_LoadIndicators, hive string, version string) (_ []*entities.DUT, err error) {
	ctx, span := otil.FuncSpan(ctx)
	defer func() { otil.EndSpan(span, err) }()
	otil.AddValues(span, d)
	currentDUTs, err := getDroneDUTs(ctx, d)
	if err != nil {
		return nil, errors.Fmt("assign new DUTs to %v: %w", d, err)
	}
	dutsNeeded := uint32ToInt(li.GetDutCapacity()) - len(currentDUTs)

	newDUTs, err := getUnassignedDUTs(ctx, int32(dutsNeeded), hive)
	if err != nil {
		return nil, errors.Fmt("assign new DUTs to %v: %w", d, err)
	}
	logging.Infof(ctx, "Got unassigned DUTs to assign: %v", entities.FormatDUTs(newDUTs))
	for _, dut := range newDUTs {
		dut.AssignedDrone = d
	}
	currentDUTs = append(currentDUTs, newDUTs...)
	if err := datastore.Put(ctx, newDUTs); err != nil {
		return nil, errors.Fmt("assign new DUTs to %v: %w", d, err)
	}
	updateAgentLoad(d, agentLoad{hive: hive, version: version, totalCapacity: int(li.GetDutCapacity()), usedCapacity: len(currentDUTs)})
	return currentDUTs, nil
}

// FreeInvalidDUTs unassigns DUTs that are assigned to a missing or
// expired drone.  This function cannot be called in a transaction.
func FreeInvalidDUTs(ctx context.Context, now time.Time) (err error) {
	ctx, span := otil.FuncSpan(ctx)
	defer func() { otil.EndSpan(span, err) }()
	var d []entities.DUT
	q := datastore.NewQuery(entities.DUTKind)
	q = q.Ancestor(entities.DUTGroupKey(ctx))
	q = q.KeysOnly(true)
	if err := datastore.GetAll(ctx, q, &d); err != nil {
		return errors.Fmt("free invalid DUTs: get all DUTs: %w", err)
	}
	validDrones, err := getValidDrones(ctx, now)
	if err != nil {
		return errors.Fmt("free invalid DUTs: %w", err)
	}
	for _, d := range d {
		f := func(ctx context.Context) error {
			ctx, span := otel.Tracer(tname).Start(ctx, "update DUT")
			defer span.End()
			otil.AddValues(span, d.ID)
			if err := datastore.Get(ctx, &d); err != nil {
				return errors.Fmt("get DUT %v: %w", d.ID, err)
			}
			if d.AssignedDrone == "" {
				return nil
			}
			if validDrones[d.AssignedDrone] {
				return nil
			}
			d.AssignedDrone = ""
			if err := datastore.Put(ctx, &d); err != nil {
				return errors.Fmt("put DUT %v: %w", d.ID, err)
			}
			return nil
		}
		if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
			return errors.Fmt("free invalid DUTs: %w", err)
		}
	}
	return nil
}

// getValidDrones returns a map of all valid drones.
func getValidDrones(ctx context.Context, now time.Time) (_ map[entities.DroneID]bool, err error) {
	ctx, span := otil.FuncSpan(ctx)
	defer func() { otil.EndSpan(span, err) }()
	q := datastore.NewQuery(entities.DroneKind)
	var d []entities.Drone
	if err := datastore.GetAll(ctx, q, &d); err != nil {
		return nil, errors.Fmt("get valid drones: %w", err)
	}
	m := make(map[entities.DroneID]bool)
	for _, d := range d {
		if d.Expiration.After(now) {
			m[d.ID] = true
		}
	}
	return m, nil
}

// PruneExpiredDrones deletes Drones that have expired.  This function
// cannot be called in a transaction.
func PruneExpiredDrones(ctx context.Context, now time.Time) (err error) {
	ctx, span := otil.FuncSpan(ctx)
	defer func() { otil.EndSpan(span, err) }()
	var d []entities.Drone
	q := datastore.NewQuery(entities.DroneKind)
	if err := datastore.GetAll(ctx, q, &d); err != nil {
		return errors.Fmt("prune expired drones: get drones: %w", err)
	}
	for _, d := range d {
		if d.Expiration.After(now) {
			continue
		}
		if err := datastore.Delete(ctx, &d); err != nil {
			return errors.Fmt("prune expired drones: delete drone %v: %w", d.ID, err)
		}
		deleteAgentLoad(d.ID)
	}
	return nil
}

// PruneDrainedDUTs deletes DUTs that are draining and not assigned to
// any drone.  This function does not need to be called in a
// transaction.
func PruneDrainedDUTs(ctx context.Context) (err error) {
	ctx, span := otil.FuncSpan(ctx)
	defer func() { otil.EndSpan(span, err) }()
	var d []entities.DUT
	q := datastore.NewQuery(entities.DUTKind)
	q = q.Ancestor(entities.DUTGroupKey(ctx))
	q = q.Eq(entities.DrainingField, true)
	q = q.Eq(entities.AssignedDroneField, "")
	if err := datastore.GetAll(ctx, q, &d); err != nil {
		return errors.Fmt("get draining DUTs: %w", err)
	}
	for _, d := range d {
		if err := datastore.Delete(ctx, &d); err != nil {
			return errors.Fmt("delete DUT %v: %w", d.ID, err)
		}
	}
	return nil
}

// uint32ToInt converts a uint32 to an int.  In case of overflow, panic.
func uint32ToInt(a uint32) int {
	b := int(a)
	if b < 0 {
		panic(a)
	}
	return b
}
