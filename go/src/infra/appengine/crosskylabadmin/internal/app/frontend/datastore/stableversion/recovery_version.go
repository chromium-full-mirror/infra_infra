// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package stableversion provides functions to store stableversion info in datastore
package stableversion

import (
	"context"
	"strings"

	"go.chromium.org/chromiumos/infra/proto/go/lab_platform"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	"infra/cros/stableversion/keys"
)

const (
	StableVersionKind = "recoveryVersion"
)

// StableVersionEntity is a datastore entity that maps a key to a recovery version.
type StableVersionEntity struct {
	_kind string `gae:"$kind,recoveryVersion"`
	// ID is a combination of board;model;pool e.g. eve;eve;quota.
	ID string `gae:"$id"`
	// Cros is a CrOS version e.g. R1-2.3.4.
	Version *lab_platform.StableVersion
}

func findVersion(ctx context.Context, key keys.Builder) (*lab_platform.StableVersion, error) {
	entity := &StableVersionEntity{ID: key.String()}
	err := datastore.Get(ctx, entity)
	if err == nil {
		return entity.Version, nil
	}
	logging.Infof(ctx, "Find recovery version: no version for key=%q: %s", key, err.Error())
	return nil, errors.Annotate(err, "find recovery version").Err()
}

// FindVersion find a stable-version from datastore.
func FindVersion(ctx context.Context, board, model string, pools []string) (*lab_platform.StableVersion, error) {
	logging.Infof(ctx, "Starting search for a version by board=%q, model=%q, pool=%v", board, model, pools)
	if board == "" {
		return nil, errors.Reason("find version: board cannot be empty").Err()
	}
	if model == "" {
		return nil, errors.Reason("find version: model cannot be empty").Err()
	}
	// Add empty pool to validate version by board and model only as last option.
	pools = append(pools, "")
	for _, pool := range pools {
		key := keys.New(board, model, pool)
		if v, err := findVersion(ctx, key); err != nil {
			logging.Debugf(ctx, "find version: fail to find versio for key=%q", key)
		} else if v != nil {
			logging.Debugf(ctx, "find version: found for key=%q", key)
			return v, nil
		}
	}
	return nil, errors.Reason("find version: no version found").Err()
}

// WriteVersions writes versions to datastore.
func WriteVersions(ctx context.Context, versions []*lab_platform.StableVersion) error {
	if len(versions) == 0 {
		logging.Infof(ctx, "Write versions: no version to safe!")
		// nothing to save.
		return nil
	}
	versionMap := removeBadVersions(ctx, versions)
	var oldRecords []*StableVersionEntity
	if err := datastore.GetAll(ctx, datastore.NewQuery(StableVersionKind), &oldRecords); err != nil {
		return errors.Annotate(err, "write versions: fail to read record from datastore").Err()
	}
	if len(oldRecords) > 0 {
		var notValidRecords []*StableVersionEntity
		for _, record := range oldRecords {
			key := targetToKey(record.Version)
			if versionMap[key.String()] == nil {
				notValidRecords = append(notValidRecords, record)
			}
		}
		if len(notValidRecords) > 0 {
			if err := datastore.Delete(ctx, notValidRecords); err != nil {
				return errors.Annotate(err, "write versions: fail to remove expired records").Err()
			}
		}
	}
	var newRecords []*StableVersionEntity
	for key, v := range versionMap {
		newRecords = append(newRecords, &StableVersionEntity{
			ID:      key,
			Version: v,
		})
	}
	if len(newRecords) > 0 {
		if err := datastore.Put(ctx, newRecords); err != nil {
			return errors.Annotate(err, "write versions: fail to save new records").Err()
		}
	}
	return nil
}

func targetToKey(v *lab_platform.StableVersion) keys.Builder {
	return keys.New(v.GetTarget().GetBoard(), v.GetTarget().GetModel(), v.GetTarget().GetPool())
}

func validateTarget(t *lab_platform.StableVersionTarget) error {
	if t == nil {
		return errors.Reason("validate target: target is empty").Err()
	}
	t.Board = strings.TrimSpace(strings.ToLower(t.GetBoard()))
	t.Model = strings.TrimSpace(strings.ToLower(t.GetModel()))
	t.Pool = strings.TrimSpace(strings.ToLower(t.GetPool()))
	if t.Board == "" || t.Model == "" {
		return errors.Reason("validate target: board/model is empty").Err()
	}
	return nil
}

func validateVersion(v *lab_platform.StableVersion) error {
	if err := validateTarget(v.GetTarget()); err != nil {
		return errors.Annotate(err, "validate version").Err()
	}
	if v.GetOsVersion() == "" || v.GetOsImagePath() == "" {
		return errors.Reason("validate version: os data is empty").Err()
	}
	return nil
}

// removeEmptyKeyOrValue destructively drops empty keys or values from versionMap
func removeBadVersions(ctx context.Context, versions []*lab_platform.StableVersion) map[string]*lab_platform.StableVersion {
	resMap := make(map[string]*lab_platform.StableVersion, len(versions))
	var totalRemovedVersions int
	for _, v := range versions {
		if err := validateVersion(v); err != nil {
			totalRemovedVersions++
			logging.Debugf(ctx, "Skip version due to bad target: %v", v.GetTarget())
			continue
		}
		key := targetToKey(v)
		if resMap[key.String()] != nil {
			totalRemovedVersions++
			logging.Debugf(ctx, "Skip version due to duplicate target: %q", key.String())
			continue
		}
		resMap[key.String()] = v
	}
	if totalRemovedVersions > 0 {
		logging.Infof(ctx, "removed bad vesrions:  total %d version skipped", totalRemovedVersions)
	}
	return resMap
}
