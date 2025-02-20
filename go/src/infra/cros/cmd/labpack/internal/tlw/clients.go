// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tlw

import (
	"context"
	"net/http"

	lab "go.chromium.org/chromiumos/infra/proto/go/lab"
	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"

	fleet "go.chromium.org/infra/appengine/crosskylabadmin/api/fleet/v1"
	"go.chromium.org/infra/cros/cmd/labpack/cft"
	"go.chromium.org/infra/cros/cmd/labpack/internal/site"
	"go.chromium.org/infra/cros/recovery"
	"go.chromium.org/infra/cros/recovery/ctr"
	"go.chromium.org/infra/cros/recovery/logger"
	"go.chromium.org/infra/cros/recovery/logger/metrics"
	"go.chromium.org/infra/cros/recovery/scopes"
	"go.chromium.org/infra/cros/recovery/tlw"
	"go.chromium.org/infra/cros/recovery/version"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

type AccessData struct {
	// Swarming tags specified at task scheduling.
	TaskTags map[string]string
}

// NewAccess creates TLW Access for recovery engine.
func NewAccess(ctx context.Context, in *lab.LabpackInput, ad *AccessData, logRoot string, metrics metrics.Metrics, lg logger.Logger) (context.Context, tlw.Access, cft.CftCloser, error) {
	hc, err := httpClient(ctx)
	if err != nil {
		return ctx, nil, nil, errors.Annotate(err, "create tlw access: create http client").Err()
	}
	ic := ufsAPI.NewFleetPRPCClient(&prpc.Client{
		C:       hc,
		Host:    in.InventoryService,
		Options: site.UFSPRPCOptions,
	})
	var csac fleet.InventoryClient
	if in.AdminService != "" {
		csac = fleet.NewInventoryPRPCClient(
			&prpc.Client{
				C:       hc,
				Host:    in.AdminService,
				Options: site.DefaultPRPCOptions,
			},
		)
	}
	params := scopes.GetParamCopy(ctx)
	if csac != nil {
		params[scopes.ParamKeyStableVersionServicePath] = in.AdminService
		ctx = version.WithClient(ctx, csac)
	}
	if ic != nil {
		params[scopes.ParamKeyInventoryServicePath] = in.InventoryService
	}
	params[scopes.ParamKeySwarmingTaskID] = in.SwarmingTaskId
	params[scopes.ParamKeyBuildbucketID] = in.Bbid
	if ad != nil && len(ad.TaskTags) > 0 {
		params[scopes.ParamKeySwarmingTaskTags] = ad.TaskTags
	}
	cftInfor := &cft.Info{
		UnitName:       in.UnitName,
		CreateStep:     !in.NoStepper,
		RootDir:        logRoot,
		SwarmingTaskID: in.SwarmingTaskId,
		BBID:           in.Bbid,
	}
	var cftCloser cft.CftCloser
	if !in.GetDisableCft() {
		var cftService ctr.ServiceInfo
		cftService, cftCloser, err = cft.Prepare(ctx, cftInfor, metrics, lg)
		if err != nil {
			lg.Infof("Fail to prepare CTR service to manager CFT containers!")
			return nil, nil, nil, errors.Annotate(err, "create tlw access: fail to prepare cft").Err()
		}
		params[scopes.ParamKeyCTRClient] = cftService
	}
	ctx = scopes.WithParams(ctx, params)
	access, err := recovery.NewLocalTLWAccess(ic)
	if err != nil {
		return nil, nil, nil, errors.Annotate(err, "create tlw access").Err()
	}
	return ctx, access, cftCloser, nil
}

// httpClient returns an HTTP client with authentication set up.
func httpClient(ctx context.Context) (*http.Client, error) {
	o := auth.Options{
		Method: auth.LUCIContextMethod,
		Scopes: []string{
			auth.OAuthScopeEmail,
			"https://www.googleapis.com/auth/cloud-platform",
		},
	}
	a := auth.NewAuthenticator(ctx, auth.SilentLogin, o)
	c, err := a.Client()
	return c, errors.Annotate(err, "create http client").Err()
}
