// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"flag"
	"sort"
	"strings"

	"google.golang.org/genproto/protobuf/field_mask"

	ufspb "infra/unifiedfleet/api/v1/models"
	UfleetAPI "infra/unifiedfleet/api/v1/rpc"
	UfleetUtil "infra/unifiedfleet/app/util"
)

// ClearFieldValue specifying this value in update command will send empty value
// while doing partial updates using update field mask.
var ClearFieldValue string = "-"

// getMachineLSEPrototype gets the given MachineLSEPrototype
func getMachineLSEPrototype(ctx context.Context, ic UfleetAPI.FleetClient, name string) *ufspb.MachineLSEPrototype {
	if len(name) == 0 {
		return nil
	}
	res, _ := ic.GetMachineLSEPrototype(ctx, &UfleetAPI.GetMachineLSEPrototypeRequest{
		Name: UfleetUtil.AddPrefix(UfleetUtil.MachineLSEPrototypeCollection, name),
	})
	return res
}

// GetUpdateMask returns a *field_mask.FieldMask containing paths based on which flags have been set.
//
// paths is a map of cmd line option flags to field names of the object.
func GetUpdateMask(set *flag.FlagSet, paths map[string]string) *field_mask.FieldMask {
	m := &field_mask.FieldMask{}
	set.Visit(func(f *flag.Flag) {
		if path, ok := paths[f.Name]; ok {
			m.Paths = append(m.Paths, path)
		}
	})
	pathMap := make(map[string]bool)
	for _, p := range m.Paths {
		pathMap[p] = true
	}
	var deduplicatedPaths []string
	for k := range pathMap {
		deduplicatedPaths = append(deduplicatedPaths, k)
	}
	sort.Strings(deduplicatedPaths)
	m.Paths = deduplicatedPaths
	return m
}

// GetStringSlice converts the comma separated string to a slice of strings
func GetStringSlice(msg string) []string {
	if msg == "" {
		return nil
	}
	return strings.Split(strings.Replace(msg, " ", "", -1), ",")
}

// GenerateAssetUpdate generates an AssetUpdate request for location, model and board updates
func GenerateAssetUpdate(machine, model, board, zone, rack string) (*ufspb.Asset, []string) {
	if model == "" && board == "" && zone == "" && rack == "" {
		return nil, nil
	}
	asset := &ufspb.Asset{
		Name: UfleetUtil.AddPrefix(UfleetUtil.AssetCollection, machine),
		Location: &ufspb.Location{
			Zone: UfleetUtil.ToUFSZone(zone),
			Rack: rack,
		},
		Info: &ufspb.AssetInfo{
			Model:       model,
			BuildTarget: board,
		},
		Model: model,
	}
	// Create the update field mask.
	var paths []string
	// Override model with user provided option.
	if model != "" {
		paths = append(paths, "model")
	}
	// Override board with user provided option
	if board != "" {
		paths = append(paths, "info.build_target")
	}
	if asset.GetLocation().GetZone() != ufspb.Zone_ZONE_UNSPECIFIED {
		paths = append(paths, "location.zone")
		asset.Realm = UfleetUtil.ToUFSRealm(asset.GetLocation().GetZone().String())
	}
	if asset.GetLocation().GetRack() != "" {
		paths = append(paths, "location.rack")
	}
	return asset, paths
}
