// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"math/rand"
	"runtime/debug"
	"time"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/cros/hwid"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/config"
	"go.chromium.org/infra/unifiedfleet/app/model/configuration"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

const cacheAge = time.Hour

// GetHwidData takes an hwid and returns the Sku and Variant in the form of
// HwidData proto. It will try the following in order:
// 1. Query from datastore. If under an hour old, return data.
// 2. If over an hour old or no data in datastore, attempt to query new data
// from HWID server.
// 3. If HWID server data available, cache into datastore and return data.
// 4. If server fails, return expired datastore data if present. If not, return
// nil and error.
func GetHwidData(ctx context.Context, c hwid.ClientInterface, hwid string) (data *ufspb.HwidData, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.Reason("Recovered from %v\n%s", r, debug.Stack()).Err()
		}
	}()

	// TODO(b/286921043): allow partners to access ACLed HWID Data
	if util.GetDatastoreNamespace(ctx) == util.OSPartnerNamespace {
		return nil, errors.New("hwid not available for partners")
	}

	hwidEnt, err := configuration.GetHwidData(ctx, hwid)
	if err != nil && !util.IsNotFoundError(err) {
		return nil, err
	}

	cacheExpired := false
	if hwidEnt != nil {
		cacheExpired = time.Now().UTC().After(hwidEnt.Updated.Add(cacheAge))
	}

	hwidServerOk := rand.Float32() < config.Get(ctx).GetHwidServiceTrafficRatio()
	if hwidServerOk && (hwidEnt == nil || cacheExpired) {
		hwidEntNew, err := fetchHwidData(ctx, c, hwid)
		if err != nil {
			logging.Warningf(ctx, "Error fetching HWID server data: %s", err)
		}

		if hwidEntNew != nil {
			hwidEnt = hwidEntNew
		}
	}
	return configuration.ParseHwidData(hwidEnt)
}

// fetchHwidData queries the hwid server with an hwid and stores the results
// into the UFS datastore.
func fetchHwidData(ctx context.Context, c hwid.ClientInterface, hwid string) (*configuration.HwidDataEntity, error) {
	newData, err := c.QueryHwid(ctx, hwid)
	if err != nil {
		return nil, err
	}

	hwidData := &ufspb.HwidData{
		DutLabel: newData,
		Hwid:     hwid,
	}
	hwidData = configuration.SetHwidDataWithDutLabels(hwidData)

	resp, err := configuration.UpdateHwidData(ctx, hwidData, hwid)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ListHwidData lists the HwidData in datastore.
func ListHwidData(ctx context.Context, pageSize int32, pageToken, filter string, keysOnly bool) ([]*ufspb.HwidData, string, error) {
	return configuration.ListHwidData(ctx, pageSize, pageToken, nil, keysOnly)
}
