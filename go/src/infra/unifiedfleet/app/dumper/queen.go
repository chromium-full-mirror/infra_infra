// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dumper

import (
	"context"
	"strings"

	"golang.org/x/oauth2"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/server/auth"

	dronequeenapi "infra/appengine/drone-queen/api"
	ufspb "infra/unifiedfleet/api/v1/models"
	"infra/unifiedfleet/app/config"
	"infra/unifiedfleet/app/model/inventory"
	"infra/unifiedfleet/app/util"
)

type DroneQueenClientGenerator func(context.Context) (dronequeenapi.InventoryProviderClient, error)

var (
	// nsToPush contains all namespaces that should be pushed to drone queen.
	// Drone queen only cares about MachineLSEs for chromeOS-like applications
	// including both internal (`os`) and external (`os-partner`) devices.
	nsToPush = []string{util.OSNamespace, util.OSPartnerNamespace}

	droneQueenGenerator DroneQueenClientGenerator = getDroneQueenClient
)

// pushToDroneQueen push the ufs duts to drone queen
func pushToDroneQueen(ctx context.Context) (err error) {
	defer func() {
		dumpPushToDroneQueenTick.Add(ctx, 1, err == nil)
	}()
	logging.Infof(ctx, "pushToDroneQueen")
	client, err := droneQueenGenerator(ctx)
	if err != nil {
		return err
	}
	var availableDuts []*dronequeenapi.DeclareDutsRequest_Dut

	// loop through all namespaces, build a collection of DUTs, and then push
	for _, ns := range nsToPush {
		ctx, err = util.SetupDatastoreNamespace(ctx, ns)
		if err != nil {
			return err
		}
		// Get all the MachineLSEs
		// Set keysOnly to true to get only keys. This is faster and consumes less data.
		lseNameHive, err := inventory.ListAllMachineLSEsNameHive(ctx)
		if err != nil {
			err = errors.Annotate(err, "failed to list all MachineLSEs for chrome %s namespace", ns).Err()
			logging.Errorf(ctx, err.Error())
			return err
		}
		sUnits, err := getAllSchedulingUnits(ctx, false)
		if err != nil {
			return err
		}

		// Map for MachineLSEs associated with SchedulingUnit for easy search.
		lseInSUnitMap := make(map[string]bool)
		for _, su := range sUnits {
			if len(su.GetMachineLSEs()) > 0 {
				availableDuts = append(availableDuts, &dronequeenapi.DeclareDutsRequest_Dut{
					Name: su.GetName(),
					Hive: util.GetHiveForDut(su.GetName(), ""),
				})
				for _, lseName := range su.GetMachineLSEs() {
					lseInSUnitMap[lseName] = true
				}
			}
		}
		for lseName, hive := range lseNameHive {
			if !lseInSUnitMap[lseName] {
				// Do not include "cloudbots" prefix hives DUTS
				if strings.HasPrefix(hive, "cloudbots") {
					continue
				}
				availableDuts = append(availableDuts, &dronequeenapi.DeclareDutsRequest_Dut{
					Name: lseName,
					Hive: util.GetHiveForDut(lseName, hive),
				})
			}
		}
	}

	logging.Debugf(ctx, "DUTs to declare(%d): %+v", len(availableDuts), availableDuts)
	_, err = client.DeclareDuts(ctx, &dronequeenapi.DeclareDutsRequest{AvailableDuts: availableDuts})
	return err
}

// getDroneQueenClient returns the drone queen client
func getDroneQueenClient(ctx context.Context) (dronequeenapi.InventoryProviderClient, error) {
	queenHostname := config.Get(ctx).QueenService
	if queenHostname == "" {
		logging.Errorf(ctx, "no drone queen service configured")
		return nil, errors.New("no drone queen service configured")
	}
	ts, err := auth.GetTokenSource(ctx, auth.AsSelf)
	if err != nil {
		return nil, err
	}
	h := oauth2.NewClient(ctx, ts)
	return dronequeenapi.NewInventoryProviderPRPCClient(&prpc.Client{
		C:    h,
		Host: queenHostname,
	}), nil
}

func getAllSchedulingUnits(ctx context.Context, keysOnly bool) ([]*ufspb.SchedulingUnit, error) {
	var sUnits []*ufspb.SchedulingUnit
	for startToken := ""; ; {
		res, nextToken, err := inventory.ListSchedulingUnits(ctx, pageSize, startToken, nil, keysOnly)
		if err != nil {
			return nil, errors.Annotate(err, "get all SchedulingUnits").Err()
		}
		sUnits = append(sUnits, res...)
		if nextToken == "" {
			break
		}
		startToken = nextToken
	}
	return sUnits, nil
}
