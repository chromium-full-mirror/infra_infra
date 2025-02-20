// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"time"

	cloudBQ "cloud.google.com/go/bigquery"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	kartepb "go.chromium.org/infra/cros/karte/api"
	kbqpb "go.chromium.org/infra/cros/karte/api/bigquery"
	"go.chromium.org/infra/cros/karte/internal/filterexp"
	"go.chromium.org/infra/cros/karte/internal/identifiers"
	"go.chromium.org/infra/cros/karte/internal/scalars"
	"go.chromium.org/infra/libs/skylab/common/heuristics"
)

// defaultBatchSize is the default size of a batch for a datastore query.
const defaultBatchSize = 50_000

// ActionKind is the kind of an action
const ActionKind = "ActionKind"

// ObservationKind is the kind of an observation.
const ObservationKind = "ObservationKind"

// ActionEntity is the datastore entity for actions.
//
// Remember to check the setActionEntityFields function.
type ActionEntity struct {
	_kind          string    `gae:"$kind,ActionKind"`
	ID             string    `gae:"$id"`
	Kind           string    `gae:"kind"`
	SwarmingTaskID string    `gae:"swarming_task_id"`
	BuildbucketID  string    `gae:"buildbucket_id"`
	AssetTag       string    `gae:"asset_tag"`
	StartTime      time.Time `gae:"start_time"`
	StopTime       time.Time `gae:"stop_time"`
	CreateTime     time.Time `gae:"receive_time"`
	Status         int32     `gae:"status"`
	FailReason     string    `gae:"fail_reason,noindex"`
	SealTime       time.Time `gae:"seal_time"` // After the seal time has passed, no further modifications may be made.
	Hostname       string    `gae:"hostname"`
	Model          string    `gae:"model"`
	Board          string    `gae:"board"`
	RecoveredBy    string    `gae:"recovered_by"`
	Restarts       int32     `gae:"restarts"`
	PlanName       string    `gae:"plan_name"`
	AllowFail      int32     `gae:"allow_fail"`
	ActionType     int32     `gae:"action_type"`
	// Count the number of times that an action entity was modified by a request.
	ModificationCount int32 `gae:"modification_count"`
	// Deprecated fields!
	ErrorReason string `gae:"error_reason,noindex"` // succeeded by "fail_reason'.
}

// maxStringFieldLength b:267100941
const maxStringFieldLength = 1400

// Normalize normalizes an action entity for writing.
func (e *ActionEntity) Normalize() {
	e.ErrorReason = heuristics.TruncateErrorString(e.ErrorReason)
	e.FailReason = heuristics.TruncateErrorString(e.FailReason)
}

// Validate validates an action entity for writing.
func (e ActionEntity) Validate() error {
	var errs []error
	if e.ID == "" {
		errs = append(errs, errors.Reason("ID cannot be empty").Err())
	}
	if len(e.ErrorReason) > maxStringFieldLength {
		errs = append(errs, errors.Reason("ErrorReason is too long").Err())
	}
	return errors.Join(errs...)
}

// CreateActionKey creates a datastore.Key for an action.
func CreateActionKey(ctx context.Context, t time.Time, disambiguation uint32) (*datastore.Key, error) {
	if ok := t.Location() == time.UTC; !ok {
		return nil, errors.Reason("create action key: location %q must be UTC", t.Location()).Err()
	}
	id, err := identifiers.MakeRawID(t, disambiguation)
	if err != nil {
		return nil, errors.Annotate(err, "create action key").Err()
	}
	return datastore.KeyForObjErr(ctx, &ActionEntity{ID: id})
}

// ConvertToAction converts a datastore action entity to an action proto.
func (e *ActionEntity) ConvertToAction() *kartepb.Action {
	if e == nil {
		return nil
	}
	return &kartepb.Action{
		Name:              e.ID,
		Kind:              e.Kind,
		SwarmingTaskId:    e.SwarmingTaskID,
		BuildbucketId:     e.BuildbucketID,
		AssetTag:          e.AssetTag,
		StartTime:         scalars.ConvertTimeToTimestampPtr(e.StartTime),
		StopTime:          scalars.ConvertTimeToTimestampPtr(e.StopTime),
		CreateTime:        scalars.ConvertTimeToTimestampPtr(e.CreateTime),
		Status:            scalars.ConvertInt32ToActionStatus(e.Status),
		FailReason:        e.FailReason,
		SealTime:          scalars.ConvertTimeToTimestampPtr(e.SealTime),
		Hostname:          e.Hostname,
		ModificationCount: e.ModificationCount,
		Model:             e.Model,
		Board:             e.Board,
		Restarts:          e.Restarts,
		RecoveredBy:       e.RecoveredBy,
		PlanName:          e.PlanName,
		AllowFail:         kartepb.Action_AllowFail(e.AllowFail),
		ActionType:        kartepb.Action_ActionType(e.ActionType),
	}
}

// ConvertToValueSaver converts a datastore action entity to a ValueSaver.
func (e *ActionEntity) ConvertToValueSaver() cloudBQ.ValueSaver {
	if e == nil {
		return nil
	}
	return &kbqpb.Action{
		Name:           e.ID,
		Kind:           e.Kind,
		SwarmingTaskId: e.SwarmingTaskID,
		BuildbucketId:  e.BuildbucketID,
		AssetTag:       e.AssetTag,
		StartTime:      scalars.ConvertTimeToTimestampPtr(e.StartTime),
		StopTime:       scalars.ConvertTimeToTimestampPtr(e.StopTime),
		CreateTime:     scalars.ConvertTimeToTimestampPtr(e.CreateTime),
		Status:         scalars.ConvertActionStatusIntToString(e.Status),
		FailReason:     e.FailReason,
		SealTime:       scalars.ConvertTimeToTimestampPtr(e.SealTime),
		Hostname:       e.Hostname,
		Model:          e.Model,
		Board:          e.Board,
		// ModificationCount is intentionally absent from BigQuery table.
		RecoveredBy: e.RecoveredBy,
		Restarts:    e.Restarts,
		PlanName:    e.PlanName,
		AllowFail:   kartepb.Action_AllowFail_name[e.AllowFail],
		ActionType:  kartepb.Action_ActionType_name[e.ActionType],
	}
}

// ObservationEntity is the datastore entity for observations.
// Only one of value_string or value_number can have a non-default value. If this constraint is not satisfied, then the record is ill-formed.
type ObservationEntity struct {
	_kind       string  `gae:"$kind,ObservationKind"`
	ID          string  `gae:"$id"`
	ActionID    string  `gae:"action_id"`
	MetricKind  string  `gae:"metric_kind"`
	ValueString string  `gae:"value_string"`
	ValueNumber float64 `gae:"value_number"`
}

// GetType returns whether an observation record is a number or a string.
func (e *ObservationEntity) GetType() string {
	if e.ValueString != "" {
		return "string"
	}
	return "num"
}

// ConvertToValueSaver converts an observation entity into a record that can be saved to bigquery.
func (e *ObservationEntity) ConvertToValueSaver() cloudBQ.ValueSaver {
	if e == nil {
		return nil
	}
	return &kbqpb.Observation{
		Name:        e.ID,
		ActionName:  e.ActionID,
		MetricKind:  e.MetricKind,
		Type:        e.GetType(),
		ValueString: e.ValueString,
		ValueNumber: e.ValueNumber,
	}
}

// cmp compares two ObservationEntities. ObservationEntities are linearly ordered by all their fields.
// This order is not related to the semantics of an ObservationEntity.
func (e *ObservationEntity) cmp(o *ObservationEntity) int {
	if e._kind > o._kind {
		return +1
	}
	if e._kind < o._kind {
		return -1
	}
	if e.ID > o.ID {
		return +1
	}
	if e.ID < o.ID {
		return -1
	}
	if e.ActionID > o.ActionID {
		return +1
	}
	if e.ActionID < o.ActionID {
		return -1
	}
	if e.MetricKind > o.MetricKind {
		return +1
	}
	if e.MetricKind < o.MetricKind {
		return -1
	}
	if e.ValueString > o.ValueString {
		return +1
	}
	if e.ValueNumber < o.ValueNumber {
		return -1
	}
	return 0
}

// Validate performs shallow validation on an observation entity.
// It enforces the constraint that only one of ValueString or ValueNumber can have a non-zero value.
func (e *ObservationEntity) Validate() error {
	if e.ValueString == "" && e.ValueNumber == 0.0 {
		return status.Errorf(codes.Internal, "datastore.go Validate: observation entity can have at most one value")
	}
	return nil
}

// ConvertToObservation converts a datastore observation entity to an observation proto.
// ConvertToObservation does NOT perform validation on the observation entity it is given;
// this function assumes that its receiver is shallowly valid.
func (e *ObservationEntity) ConvertToObservation() *kartepb.Observation {
	obs := &kartepb.Observation{
		Name:       e.ID,
		ActionName: e.ActionID,
		MetricKind: e.MetricKind,
	}
	if e.ValueString != "" {
		obs.Value = &kartepb.Observation_ValueString{
			ValueString: e.ValueString,
		}
	} else {
		obs.Value = &kartepb.Observation_ValueNumber{
			ValueNumber: e.ValueNumber,
		}
	}
	return obs
}

// ActionEntitiesQuery is a wrapped query of action entities bearing a page token.
type ActionEntitiesQuery struct {
	// Token is the pagination token used by datastore.
	Token string
	// Query is a wrapped datastore query.
	Query *datastore.Query
}

// ActionQueryAncillaryData returns ancillary data computed as part of advancing through
// an action entities query.
//
// Currently, we return the biggest (earliest) and smallest (latest) version seen.
type ActionQueryAncillaryData struct {
	BiggestID       string
	SmallestID      string
	BiggestVersion  string
	SmallestVersion string
}

// UpdateWith takes an action entity query and a list of queries to update from and applies them.
func (a *ActionQueryAncillaryData) updateWith(d *ActionQueryAncillaryData) {
	a.BiggestID = maxValue(a.BiggestID, d.BiggestID)
	a.SmallestID = minValue(a.SmallestID, d.SmallestID)
	a.BiggestVersion = maxValue(a.BiggestVersion, d.BiggestVersion)
	a.SmallestVersion = minValue(a.SmallestVersion, d.SmallestVersion)
}

// minValue computes the minimum of two Karte version strings lexicographically.
func minValue(a string, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	if a <= b {
		return a
	}
	return b
}

// maxValue computes the maximum of two Karte version strings lexicographically.
func maxValue(a string, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	if a <= b {
		return b
	}
	return a
}

// errToken indicates that the query is in the error state and we cannot proceed.
const errToken = "ERR 39c4bb59-2008-49c1-973c-36954c60b92c cb441ab0-4ed1-4c13-969e-dfe9f4be9588"

// stopToken indicates that we've reached the end of the input.
const stopToken = "STOP 39c4bb59-2008-49c1-973c-36954c60b92c 6035406e-c3e0-4db7-937c-ba6a41010694"

// Next takes a batch size and returns the next batch of action entities from a query.
func (q *ActionEntitiesQuery) Next(ctx context.Context, batchSize int32) ([]*ActionEntity, ActionQueryAncillaryData, error) {
	var d ActionQueryAncillaryData
	if q == nil {
		return nil, d, errors.Reason("next: action entities query cannot be nil").Err()
	}
	switch q.Token {
	case errToken:
		return nil, d, errors.Reason("next: query is in error state").Err()
	case stopToken:
		return nil, d, errors.Reason("next: query is stopped").Err()
	}
	if batchSize == 0 {
		batchSize = defaultBatchSize
		logging.Debugf(ctx, "applied default batch size %d\n", defaultBatchSize)
	}
	// A rootedQuery is rooted at the position implied by the pagination token.
	rootedQuery := q.Query
	if q.Token != "" {
		cursor, err := datastore.DecodeCursor(ctx, q.Token)
		if err != nil {
			return nil, ActionQueryAncillaryData{}, errors.Annotate(err, "next action entity: decoding cursor").Err()
		}
		rootedQuery = q.Query.Start(cursor)
	}
	rootedQuery = rootedQuery.Limit(batchSize)
	var entities []*ActionEntity
	err := datastore.Run(ctx, rootedQuery, func(ent *ActionEntity, cb datastore.CursorCB) error {
		// Record the ancillary info! What versions did we see?
		version := identifiers.GetIDVersion(ent.ID)
		d.updateWith(&ActionQueryAncillaryData{
			SmallestVersion: version,
			BiggestVersion:  version,
			SmallestID:      ent.ID,
			BiggestID:       ent.ID,
		})
		entities = append(entities, ent)
		// This inequality is weak because this block must run on the last iteration
		// when the query is successful.
		// If the query stops early, we can assume that we have reached the end of the result set
		// and therefore the response token should be empty.
		if len(entities) >= int(batchSize) {
			tok, err := cb()
			if err != nil {
				return errors.Annotate(err, "next action entity (entities: %d)", len(entities)).Err()
			}
			q.Token = tok.String()
			return nil
		}
		q.Token = stopToken
		return nil
	})
	logging.Infof(ctx, "Version range for batch %v", d)
	if err != nil {
		return nil, d, errors.Annotate(err, "next action entity: after running query").Err()
	}
	return entities, d, nil
}

// newActionEntitiesQuery makes an action entities query that starts at the position implied
// by the given token and lists all action entities matching the condition described in the
// filter.
func newActionEntitiesQuery(token string, filter string) (*ActionEntitiesQuery, error) {
	expr, err := filterexp.Parse(filter)
	if err != nil {
		// TODO(gregorynisbet): Pick more consistent strategy for assigning error statuses.
		return nil, status.Errorf(codes.InvalidArgument, "make action entities query: %s", err)
	}
	q, err := filterexp.ApplyConditions(
		datastore.NewQuery(ActionKind),
		expr,
	)
	if err != nil {
		return nil, errors.Annotate(err, "make action entities query").Err()
	}
	return &ActionEntitiesQuery{
		Token: token,
		Query: q,
	}, nil
}

// newActionNameRangeQuery takes a beginning name and an end name and produces a query.
//
// This query will apply to names strictly in the range [begin, end).
func newActionNameRangeQuery(begin time.Time, end time.Time) (*ActionEntitiesQuery, error) {
	if ok := begin.Location() == time.UTC; !ok {
		return nil, errors.Reason("new action name range query: begin location must be UTC not %q", begin.Location().String()).Err()
	}
	if ok := end.Location() == time.UTC; !ok {
		return nil, errors.Reason("new action name range query: end location must be UTC not %q", end.Location().String()).Err()
	}
	q := datastore.NewQuery(ActionKind)
	// The datastore query will actually reject invalid arguments on its own, but we can give the user
	// a better error message if we check the arguments ourselves.
	switch {
	case begin.After(end):
		return nil, errors.Reason("begin time %v is after end time %v", begin, end).Err()
	case begin.Equal(end):
		return nil, errors.Reason("rejecting likely erroneous call: begin time %q and end time are equal %q", begin.String(), end.String()).Err()
	}
	q = q.Gte("receive_time", begin).Lt("receive_time", end)
	return &ActionEntitiesQuery{
		Query: q,
		Token: "",
	}, nil
}

// ObservationEntitiesQuery is a wrapped query of action entities bearing a page token.
type ObservationEntitiesQuery struct {
	// Token is the pagination token used by datastore.
	Token string
	// Query is a wrapped datastore query.
	Query *datastore.Query
}

// Next takes a batch size and returns the next batch of observation entities from a query.
func (q *ObservationEntitiesQuery) Next(ctx context.Context, batchSize int32) ([]*ObservationEntity, error) {
	if batchSize == 0 {
		batchSize = defaultBatchSize
		logging.Debugf(ctx, "applied default batch size %d\n", defaultBatchSize)
	}
	var nextToken string
	// A rootedQuery is rooted at the position implied by the pagination token.
	rootedQuery := q.Query
	if q.Token != "" {
		cursor, err := datastore.DecodeCursor(ctx, q.Token)
		if err != nil {
			return nil, errors.Annotate(err, "next observation entity").Err()
		}
		rootedQuery = q.Query.Start(cursor)
	}
	rootedQuery = rootedQuery.Limit(batchSize)
	var entities []*ObservationEntity
	err := datastore.Run(ctx, rootedQuery, func(ent *ObservationEntity, cb datastore.CursorCB) error {
		entities = append(entities, ent)
		// This inequality is weak because this block must run on the last iteration
		// when the query is successful.
		// If the query stops early, we can assume that we have reached the end of the result set
		// and therefore the response token should be empty.
		if len(entities) >= int(batchSize) {
			tok, err := cb()
			if err != nil {
				return errors.Annotate(err, "next observation entity").Err()
			}
			nextToken = tok.String()
		}
		return nil
	})
	if err != nil {
		return nil, errors.Annotate(err, "next observation entity").Err()
	}
	q.Token = nextToken
	return entities, nil
}

// newObservationEntitiesQuery makes an action entities query that starts at the position
// implied by the page token and lists all action entities.
func newObservationEntitiesQuery(token string, filter string) (*ObservationEntitiesQuery, error) {
	expr, err := filterexp.Parse(filter)
	if err != nil {
		return nil, errors.Annotate(err, "make observation entities query").Err()
	}
	q, err := filterexp.ApplyConditions(
		datastore.NewQuery(ObservationKind),
		expr,
	)
	if err != nil {
		return nil, errors.Annotate(err, "make observation entities query").Err()
	}
	return &ObservationEntitiesQuery{
		Token: token,
		Query: q,
	}, nil
}

// convertActionToActionEntity takes an action and converts it to an action entity.
func convertActionToActionEntity(action *kartepb.Action) (*ActionEntity, error) {
	if action == nil {
		return nil, status.Errorf(codes.Internal, "convert action to action entity: action is nil")
	}
	return &ActionEntity{
		ID:             action.GetName(),
		Kind:           action.Kind,
		SwarmingTaskID: action.SwarmingTaskId,
		BuildbucketID:  action.BuildbucketId,
		AssetTag:       action.AssetTag,
		StartTime:      scalars.ConvertTimestampPtrToTime(action.StartTime),
		StopTime:       scalars.ConvertTimestampPtrToTime(action.StopTime),
		CreateTime:     scalars.ConvertTimestampPtrToTime(action.CreateTime),
		Status:         scalars.ConvertActionStatusToInt32(action.Status),
		FailReason:     action.FailReason,
		SealTime:       scalars.ConvertTimestampPtrToTime(action.SealTime),
		Hostname:       action.Hostname,
		Model:          action.GetModel(),
		Board:          action.GetBoard(),
		Restarts:       action.GetRestarts(),
		RecoveredBy:    action.GetRecoveredBy(),
		PlanName:       action.GetPlanName(),
		AllowFail:      int32(action.GetAllowFail()),
		ActionType:     int32(action.GetActionType()),
	}, nil
}

// PutActionEntities writes action entities to datastore.
func PutActionEntities(ctx context.Context, entities ...*ActionEntity) error {
	// The autogenerated ID should be a string, not an integer.
	// If the value for the ID field is "", an integer value will be
	// autogenerated behind the scenes.
	for _, entity := range entities {
		entity.Normalize()
		if err := entity.Validate(); err != nil {
			return err
		}
	}
	return datastore.Put(ctx, entities)
}

// GetActionEntityByID gets an action entity by its ID. If we confirm the absence of an entity successfully, no error is returned.
func GetActionEntityByID(ctx context.Context, id string) (*ActionEntity, error) {
	actionEntity := &ActionEntity{ID: id}
	if err := datastore.Get(ctx, actionEntity); err != nil {
		return nil, errors.Annotate(err, "get action entity by id").Err()
	}
	return actionEntity, nil
}

// convertObservationToObservationEntity takes an observation and converts it to an observation entity.
func convertObservationToObservationEntity(observation *kartepb.Observation) (*ObservationEntity, error) {
	if observation == nil {
		return nil, status.Errorf(codes.Internal, "convert observation to observation entity: action is nil")
	}
	return &ObservationEntity{
		ID:          observation.GetName(),
		ActionID:    observation.GetActionName(),
		MetricKind:  observation.GetMetricKind(),
		ValueString: observation.GetValueString(),
		ValueNumber: observation.GetValueNumber(),
	}, nil
}

// PutObservationEntities writes multiple observation entities to datastore.
func PutObservationEntities(ctx context.Context, entities ...*ObservationEntity) error {
	// The autogenerated ID should be a string, not an integer.
	// If the value for the ID field is "", an integer value will be
	// autogenerated behind the scenes.
	for _, entity := range entities {
		if entity.ID == "" {
			return errors.Reason("put action: entity with empty ID").Err()
		}
	}
	return datastore.Put(ctx, entities)
}
