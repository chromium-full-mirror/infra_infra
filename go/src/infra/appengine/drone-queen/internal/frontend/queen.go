// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"strconv"
	"time"

	"github.com/golang/protobuf/ptypes"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc/metadata"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"
	"go.chromium.org/luci/grpc/grpcutil"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"

	"go.chromium.org/infra/appengine/drone-queen/api"
	clients "go.chromium.org/infra/appengine/drone-queen/internal/clients"
	"go.chromium.org/infra/appengine/drone-queen/internal/config"
	"go.chromium.org/infra/appengine/drone-queen/internal/entities"
	"go.chromium.org/infra/appengine/drone-queen/internal/queries"
)

// Name used for OpenTelemetry tracers.
const tname = "go.chromium.org/infra/appengine/drone-queen/internal/frontend"

// Earliest supported version of drone agent.
const earliestSupportedVersion = 0

// DroneQueenImpl implements service interfaces.
type DroneQueenImpl struct {
	nowFunc func() time.Time
}

// ReportDrone implements service interfaces.
func (q *DroneQueenImpl) ReportDrone(ctx context.Context, req *api.ReportDroneRequest) (res *api.ReportDroneResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	res = &api.ReportDroneResponse{
		Status: api.ReportDroneResponse_OK,
	}
	id := entities.DroneID(req.GetDroneUuid())

	// Read drone agent version from context.
	// Reject service if unsupported.
	version := getVersionFromContext(ctx)
	if !isVersionSupported(ctx, version) {
		return nil, errors.Reason("drone version not supported").Err()
	}

	// Assign a new UUID if needed.
	if id == "" {
		id, err = queries.CreateNewDrone(ctx, q.now())
		if err != nil {
			return nil, err
		}
		res.DroneUuid = string(id)
	}
	// Refresh expiration time.
	d := entities.Drone{ID: id}
	f := func(ctx context.Context) error {
		ctx, span := otel.Tracer(tname).Start(ctx, "update drone")
		defer span.End()
		if err = datastore.Get(ctx, &d); err != nil {
			if datastore.IsErrNoSuchEntity(err) {
				res.Status = api.ReportDroneResponse_UNKNOWN_UUID
			}
			return errors.Annotate(err, "get drone %s", id).Err()
		}
		if q.now().After(d.Expiration) {
			res.Status = api.ReportDroneResponse_UNKNOWN_UUID
			return errors.Reason("drone expired").Err()
		}
		d.Expiration = q.now().Add(config.AssignmentDuration(ctx)).UTC()
		d.Description = req.GetDroneDescription()
		d.Hive = req.GetHive()
		d.Version = version
		if err = datastore.Put(ctx, &d); err != nil {
			return errors.Annotate(err, "refresh drone expiration").Err()
		}
		return nil
	}
	if err = datastore.RunInTransaction(ctx, f, nil); err != nil {
		// Specially handle specific status errors if they
		// were set.  These need to be reported in a regular
		// response.
		if res.Status != api.ReportDroneResponse_OK {
			return res, nil
		}
		return nil, err
	}
	// Assign new DUTs.
	var duts []*entities.DUT
	f = func(ctx context.Context) error {
		duts, err = queries.AssignNewDUTs(ctx, id, req.GetLoadIndicators(), req.GetHive(), version)
		return err
	}
	if err = datastore.RunInTransaction(ctx, f, nil); err != nil {
		return nil, err
	}
	// Populate response fields.
	res.ExpirationTime, err = ptypes.TimestampProto(d.Expiration)
	if err != nil {
		// Input time should always be valid.
		panic(err)
	}
	for _, d := range duts {
		if d.AssignedDrone != id {
			panic(d)
		}
		res.AssignedDuts = append(res.AssignedDuts, string(d.ID))
		if d.Draining {
			res.DrainingDuts = append(res.DrainingDuts, string(d.ID))
		}
	}
	return res, nil
}

// getVersionFromContext reads drone agent version from context metadata and returns it.
func getVersionFromContext(ctx context.Context) string {
	const fallback = "unknown"
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		logging.Debugf(ctx, "no metadata found in ReportDrone incoming context")
		return fallback
	}
	versions, ok := md["drone-agent-version"]
	if !ok {
		logging.Debugf(ctx, "'drone-agent-version' not found in ReportDrone incoming context metadata")
		return fallback
	}
	if len(versions) == 0 {
		logging.Debugf(ctx, "empty slice: 'drone-agent-version' in ReportDrone incoming context metadata")
		return fallback
	}
	if versions[0] == "unknown" {
		logging.Debugf(ctx, "'unknown' drone-agent version was passed by client")
		return fallback
	}
	return versions[0]
}

// isVersionSupported tests if drone agent version is greater than the earliest supported version.
func isVersionSupported(ctx context.Context, droneVersion string) bool {
	return isVersionSupported2(ctx, droneVersion, earliestSupportedVersion)
}

// isVersionSupported2 is used for unit tests.
func isVersionSupported2(ctx context.Context, droneVersion string, threshold int) bool {
	version, err := strconv.Atoi(droneVersion)
	if err != nil {
		logging.Debugf(ctx, "could not convert drone version to int: %v", err)
		return true
	}
	return version >= threshold
}

// listCloudbots returns list of running cloudbots swarming bots.
func listCloudbots(ctx context.Context) ([]*apipb.BotInfo, error) {
	c, err := clients.NewSwarmingClient(ctx, "chromeos-swarming.appspot.com", "chromeos")
	if err != nil {
		return nil, err
	}
	cursor := ""
	// Keep calling as long as there's a cursor indicating more bots to list.
	bots := make([]*apipb.BotInfo, 0)
	for {
		resp, err := c.ListBots(ctx, &apipb.BotsRequest{
			Limit:  500,
			Cursor: cursor,
			Dimensions: []*apipb.StringPair{
				{
					Key:   "bot_config",
					Value: "cloudbots_config.py",
				},
			},
			IsDead: apipb.NullableBool_FALSE,
		})
		if err != nil {
			return bots, err
		}
		bots = append(bots, resp.Items...)
		cursor = resp.Cursor
		if cursor == "" {
			break
		}
	}
	return bots, nil
}

// ReleaseDuts implements service interfaces.
func (q *DroneQueenImpl) ReleaseDuts(ctx context.Context, req *api.ReleaseDutsRequest) (res *api.ReleaseDutsResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	drone := entities.DroneID(req.GetDroneUuid())
	if drone == "" {
		return nil, errors.Reason("drone UUID must be supplied").Err()
	}
	for _, dut := range req.GetDuts() {
		dutID := entities.DUTID(dut)
		f := func(ctx context.Context) error {
			dut := entities.DUT{
				ID:    dutID,
				Group: entities.DUTGroupKey(ctx),
			}
			if err := datastore.Get(ctx, &dut); err != nil {
				if datastore.IsErrNoSuchEntity(err) {
					return nil
				}
				return errors.Annotate(err, "get DUT %s", dutID).Err()
			}
			if dut.AssignedDrone != drone {
				return nil
			}
			dut.AssignedDrone = ""
			if err := datastore.Put(ctx, &dut); err != nil {
				return errors.Annotate(err, "modify DUT %s", dutID).Err()
			}
			return nil
		}
		if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
			return nil, err
		}
	}
	return &api.ReleaseDutsResponse{}, nil
}

// DeclareDuts implements service interfaces.
func (q *DroneQueenImpl) DeclareDuts(ctx context.Context, req *api.DeclareDutsRequest) (res *api.DeclareDutsResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	fa := func(ctx context.Context) error {
		// Group key for DUT entity.
		dutGroupKey := entities.DUTGroupKey(ctx)
		// Get existing DUTs from datastore.
		q := datastore.NewQuery(entities.DUTKind).Ancestor(dutGroupKey)
		var existingDuts []entities.DUT
		if err := datastore.GetAll(ctx, q, &existingDuts); err != nil {
			return errors.Annotate(err, "get existing DUTs").Err()
		}
		// Create a map of existing DUTs for easy search.
		existingMap := make(map[entities.DUTID]*entities.DUT)
		for i := range existingDuts {
			existingMap[existingDuts[i].ID] = &existingDuts[i]
		}
		// Create a map of cloudbot DUTs for easy search.
		cbs, err := listCloudbots(ctx)
		if err != nil {
			logging.Errorf(ctx, "DeclareDuts: listCloudbots error %q", err)
		}
		cbMap := make(map[string]*apipb.BotInfo, len(cbs))
		for _, cb := range cbs {
			for _, d := range cb.GetDimensions() {
				if d.Key == "dut_name" {
					cbMap[d.Value[0]] = cb
					break
				}
			}
		}
		// Aggregate the DUTs to be created/updated.
		var updatedDuts []*entities.DUT
		// To track the DUTs which are declared in this call.
		declared := make(map[entities.DUTID]bool)
		for _, availableDut := range req.GetAvailableDuts() {
			if availableDut.GetName() == "" {
				continue
			}
			if _, ok := cbMap[availableDut.GetName()]; ok {
				// This DUT is still running on CloudBots.
				logging.Warningf(ctx, "DUT %s is still running on CloudBots. Not assigning to drone", availableDut.GetName())
				continue
			}
			dutID := entities.DUTID(availableDut.GetName())
			if dut, ok := existingMap[dutID]; ok {
				// This is an already existing DUT.
				if dut.Draining || dut.Hive != availableDut.GetHive() {
					// DUT is updated only if it's draining (as it is redeclared)
					// or the hive value is changed.
					// Undrain it as it is a re-declared DUT.
					dut.Draining = false
					// Update the hive value of the DUT.
					dut.Hive = availableDut.GetHive()
					updatedDuts = append(updatedDuts, dut)
				}
			} else {
				// This is a newly declared DUT.
				updatedDuts = append(updatedDuts,
					&entities.DUT{
						ID:    dutID,
						Group: dutGroupKey,
						Hive:  availableDut.GetHive(),
					})
			}
			// Mark the DUT as declared in this call.
			declared[dutID] = true
		}
		// Drain existing DUTs that were not declared.
		for i := range existingDuts {
			if !declared[existingDuts[i].ID] {
				existingDuts[i].Draining = true
				updatedDuts = append(updatedDuts, &existingDuts[i])
			}
		}
		// Update modified DUTs and add newly declared DUTs.
		if err := datastore.Put(ctx, updatedDuts); err != nil {
			return errors.Annotate(err, "add DUTs").Err()
		}
		return nil
	}
	if err := datastore.RunInTransaction(ctx, fa, nil); err != nil {
		return nil, err
	}
	return &api.DeclareDutsResponse{}, nil
}

// ListDrones implements service interfaces.
func (q *DroneQueenImpl) ListDrones(ctx context.Context, req *api.ListDronesRequest) (res *api.ListDronesResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	var drones []entities.Drone
	if err := datastore.GetAll(ctx, datastore.NewQuery(entities.DroneKind), &drones); err != nil {
		return nil, errors.Annotate(err, "get all drones").Err()
	}
	res = &api.ListDronesResponse{}
	for _, d := range drones {
		// TODO(ayatane): Log this error?
		t, _ := ptypes.TimestampProto(d.Expiration)
		res.Drones = append(res.Drones, &api.ListDronesResponse_Drone{
			Id:               string(d.ID),
			ExpirationTime:   t,
			DroneDescription: d.Description,
			Hive:             d.Hive,
		})
	}
	return res, nil
}

// ListDuts implements service interfaces.
func (q *DroneQueenImpl) ListDuts(ctx context.Context, req *api.ListDutsRequest) (res *api.ListDutsResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	var duts []entities.DUT
	if err := datastore.GetAll(ctx, datastore.NewQuery(entities.DUTKind), &duts); err != nil {
		return nil, errors.Annotate(err, "get all DUTs").Err()
	}
	res = &api.ListDutsResponse{}
	for _, d := range duts {
		res.Duts = append(res.Duts, &api.ListDutsResponse_Dut{
			Id:            string(d.ID),
			AssignedDrone: string(d.AssignedDrone),
			Draining:      d.Draining,
			Hive:          d.Hive,
		})
	}
	return res, nil
}

func (q *DroneQueenImpl) now() time.Time {
	if q.nowFunc != nil {
		return q.nowFunc()
	}
	return time.Now()
}
