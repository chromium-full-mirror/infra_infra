// Copyright 2019 The LUCI Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package stableversion provides functions to store stableversion info in datastore
package stableversion

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	libsv "go.chromium.org/infra/cros/stableversion"
)

const (
	CrosStableVersionKind     = "crosStableVersion"
	FaftStableVersionKind     = "faftStableVersion"
	FirmwareStableVersionKind = "firmwareStableVersion"
)

// CrosStableVersionEntity is a datastore entity that maps a board;model to a CrOS version.
type CrosStableVersionEntity struct {
	_kind string `gae:"$kind,crosStableVersion"`
	// ID is a board;model e.g. eve, eve.
	ID string `gae:"$id"`
	// Cros is a CrOS version e.g. R1-2.3.4.
	Cros string
}

// ImposeVersion takes in a CrOS stable version entity and a new cros version.
// If the version is empty, we delete the record.
//
// Silently do nothing if the payload and the ID are the same.
func (e *CrosStableVersionEntity) ImposeVersion(ctx context.Context, cros string) error {
	if e == nil {
		return errors.Reason("impose cros version: e cannot be nil").Err()
	}
	switch {
	case cros == "":
		return errors.Annotate(datastore.Delete(ctx, e), "impose cros version for %q: delete", e.ID).Err()
	case cros == e.Cros:
		return nil
	default:
		e.Cros = cros
		return errors.Annotate(datastore.Put(ctx, e), "impose cros version for %q: put", e.ID).Err()
	}
}

// FaftStableVersionEntity is a datastore entity that maps a board;model to a faft version.
type FaftStableVersionEntity struct {
	_kind string `gae:"$kind,faftStableVersion"`
	// ID is a board;model e.g. eve, eve.
	ID string `gae:"$id"`
	// Faft is a faft version e.g. octopus-release/R100-10000.100.0.
	Faft string
}

// ImposeVersion takes in a Faft stable version entity and a new cros version.
// If the version is empty, we delete the record.
//
// Silently do nothing if the payload and the ID are the same.
func (e *FaftStableVersionEntity) ImposeVersion(ctx context.Context, faft string) error {
	if e == nil {
		return errors.Reason("impose faft version: e cannot be nil").Err()
	}
	switch {
	case faft == "":
		return errors.Annotate(datastore.Delete(ctx, e), "impose faft version for %q: delete", e.ID).Err()
	case faft == e.Faft:
		return nil
	default:
		e.Faft = faft
		return errors.Annotate(datastore.Put(ctx, e), "impose faft version for %q: put", e.ID).Err()
	}
}

// FirmwareStableVersionEntity is a datastore entity that maps a board;model to a firmware version.
type FirmwareStableVersionEntity struct {
	_kind string `gae:"$kind,firmwareStableVersion"`
	// ID is a board;model e.g. eve, eve.
	ID string `gae:"$id"`
	// Firmware is a firmware version e.g. Google_Rammus.10000.100.0
	Firmware string
}

// ImposeVersion takes in a CrOS stable version entity and a new cros version.
// If the version is empty, we delete the record.
//
// Silently do nothing if the payload and the ID are the same.
func (e *FirmwareStableVersionEntity) ImposeVersion(ctx context.Context, firmware string) error {
	if e == nil {
		return errors.Reason("impose firmware version: e cannot be nil").Err()
	}
	switch {
	case firmware == "":
		return errors.Annotate(datastore.Delete(ctx, e), "impose firmware version for %q: delete", e.ID).Err()
	case firmware == e.Firmware:
		return nil
	default:
		e.Firmware = firmware
		return errors.Annotate(datastore.Put(ctx, e), "impose firmware version for %q: put", e.ID).Err()
	}
}

// GetCrosStableVersion gets a stable version for ChromeOS from datastore
func GetCrosStableVersion(ctx context.Context, buildTarget string, model string) (string, error) {
	key, err := libsv.JoinBuildTargetModel(buildTarget, model)
	if buildTarget == "" {
		return "", fmt.Errorf("GetCrosStableVersion: buildTarget cannot be empty")
	}
	justBoard, err := libsv.JoinBuildTargetModel(buildTarget, "")

	// look up stable version by combined key
	entity := &CrosStableVersionEntity{ID: key}
	err = datastore.Get(ctx, entity)
	if err == nil {
		return entity.Cros, nil
	}
	logging.Infof(ctx, "failed to find per-model stable version %q", err.Error())

	// look up stable version by combined key with empty model.
	// This will look like xxx-board;
	entity = &CrosStableVersionEntity{ID: justBoard}
	err = datastore.Get(ctx, entity)
	if err == nil {
		return entity.Cros, nil
	}
	logging.Infof(ctx, "failed to find per-board stable version in new format %q", err.Error())

	// fall back to looking up stable version by build target alone.
	entity = &CrosStableVersionEntity{ID: libsv.FallbackBuildTargetKey(buildTarget)}
	if err := datastore.Get(ctx, entity); err != nil {
		if datastore.IsErrNoSuchEntity(err) {
			return "", status.Errorf(codes.NotFound, fmt.Sprintf("Entity not found for %s", key))
		}
		return "", errors.Annotate(err, "GetCrosStableVersion").Err()
	}
	return entity.Cros, nil
}

// PutSingleCrosStableVersion is a convenience wrapper around PutManyCrosStableVersion
func PutSingleCrosStableVersion(ctx context.Context, buildTarget string, model string, cros string) error {
	key, err := libsv.JoinBuildTargetModel(buildTarget, model)
	if err != nil {
		logging.Infof(ctx, "falling back to buildTarget key!")
		key = buildTarget
	}
	return PutManyCrosStableVersion(ctx, map[string]string{key: cros})
}

// PutManyCrosStableVersion writes many stable versions for ChromeOS to datastore
func PutManyCrosStableVersion(ctx context.Context, crosOfKey map[string]string) error {
	removeEmptyKeyOrValue(ctx, crosOfKey)
	var entities []*CrosStableVersionEntity
	for key, cros := range crosOfKey {
		entities = append(entities, &CrosStableVersionEntity{ID: key, Cros: cros})
	}
	if err := datastore.Put(ctx, entities); err != nil {
		return errors.Annotate(err, "PutManyCrosStableVersion").Err()
	}
	return nil
}

// GetFirmwareStableVersion takes a buildtarget and a model and produces a firmware stable version from datastore
func GetFirmwareStableVersion(ctx context.Context, buildTarget string, model string) (string, error) {
	key, err := libsv.JoinBuildTargetModel(buildTarget, model)
	if err != nil {
		return "", errors.Annotate(err, "GetFirmwareStableVersion").Err()
	}
	entity := &FirmwareStableVersionEntity{ID: key}
	if err := datastore.Get(ctx, entity); err != nil {
		if datastore.IsErrNoSuchEntity(err) {
			return "", status.Errorf(codes.NotFound, fmt.Sprintf("Entity not found for %s", key))
		}
		return "", errors.Annotate(err, "GetFirmwareStableVersion").Err()
	}
	return entity.Firmware, nil
}

// PutSingleFirmwareStableVersion is a convenience wrapper around PutManyFirmwareStableVersion
func PutSingleFirmwareStableVersion(ctx context.Context, buildTarget string, model string, firmware string) error {
	key, err := libsv.JoinBuildTargetModel(buildTarget, model)
	if err != nil {
		return err
	}
	return PutManyFirmwareStableVersion(ctx, map[string]string{key: firmware})
}

// PutManyFirmwareStableVersion takes a map from build_target+model keys to firmware versions and persists it to datastore
func PutManyFirmwareStableVersion(ctx context.Context, firmwareOfJoinedKey map[string]string) error {
	removeEmptyKeyOrValue(ctx, firmwareOfJoinedKey)
	var entities []*FirmwareStableVersionEntity
	for key, firmware := range firmwareOfJoinedKey {
		entities = append(entities, &FirmwareStableVersionEntity{ID: key, Firmware: firmware})
	}
	if err := datastore.Put(ctx, entities); err != nil {
		return errors.Annotate(err, "PutManyFirmwareStableVersion").Err()
	}
	return nil
}

// GetFaftStableVersion takes a model and a buildtarget and produces a faft stable version from datastore
func GetFaftStableVersion(ctx context.Context, buildTarget string, model string) (string, error) {
	key, err := libsv.JoinBuildTargetModel(buildTarget, model)
	if err != nil {
		return "", errors.Annotate(err, "GetFaftStableVersion").Err()
	}
	entity := &FaftStableVersionEntity{ID: key}
	if err := datastore.Get(ctx, entity); err != nil {
		if datastore.IsErrNoSuchEntity(err) {
			return "", status.Errorf(codes.NotFound, fmt.Sprintf("Entity not found for %s", key))
		}
		return "", errors.Annotate(err, "GetFaftStableVersion").Err()
	}
	return entity.Faft, nil
}

// PutSingleFaftStableVersion is a convenience wrapper around PutManyFaftStableVersion
func PutSingleFaftStableVersion(ctx context.Context, buildTarget string, model string, faft string) error {
	key, err := libsv.JoinBuildTargetModel(buildTarget, model)
	if err != nil {
		return err
	}
	return PutManyFaftStableVersion(ctx, map[string]string{key: faft})
}

// PutManyFaftStableVersion takes a model, buildtarget, and faft stableversion and persists it to datastore
func PutManyFaftStableVersion(ctx context.Context, faftOfJoinedKey map[string]string) error {
	removeEmptyKeyOrValue(ctx, faftOfJoinedKey)
	var entities []*FaftStableVersionEntity
	for key, faft := range faftOfJoinedKey {
		entities = append(entities, &FaftStableVersionEntity{ID: key, Faft: faft})
	}
	if err := datastore.Put(ctx, entities); err != nil {
		return errors.Annotate(err, "PutManyFaftStableVersion").Err()
	}
	return nil
}

// removeEmptyKeyOrValue destructively drops empty keys or values from versionMap
func removeEmptyKeyOrValue(ctx context.Context, versionMap map[string]string) {
	removedTally := 0
	for k, v := range versionMap {
		if k == "" || v == "" {
			logging.Infof(ctx, "removed non-conforming key-value pair (%s) -> (%s)", k, v)
			delete(versionMap, k)
			removedTally++
			continue
		}
	}
	if removedTally > 0 {
		logging.Infof(ctx, "removed (%d) pairs for containing \"\" as key or value", removedTally)
	}
}
