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

	fleet "infra/appengine/crosskylabadmin/api/fleet/v1"
	"infra/cros/cmd/labpack/cft"
	"infra/cros/cmd/labpack/internal/site"
	"infra/cros/recovery"
	"infra/cros/recovery/logger"
	"infra/cros/recovery/logger/metrics"
	"infra/cros/recovery/scopes"
	"infra/cros/recovery/tlw"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
)

type AccessData struct {
	// Swarming tags specified at task scheduling.
	TaskTags map[string]string
}

// NewAccess creates TLW Access for recovery engine.
func NewAccess(ctx context.Context, in *lab.LabpackInput, ad *AccessData, logRoot string, metrics metrics.Metrics, lg logger.Logger) (context.Context, tlw.Access, error) {
	hc, err := httpClient(ctx)
	if err != nil {
		return ctx, nil, errors.Annotate(err, "create tlw access: create http client").Err()
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
	// TODO(otabek): Make it critical after testing.
	if cft, err := cft.Prepare(ctx, cftInfor, metrics, lg); err != nil {
		lg.Infof("(NOT critical) Fail to prepare CFT containers!")
	} else {
		params[scopes.ParamKeyCTRClient] = cft
	}
	ctx = scopes.WithParams(ctx, params)
	access, err := recovery.NewLocalTLWAccess(ic, csac)
	if err != nil {
		return nil, nil, errors.Annotate(err, "create tlw access").Err()
	}
	return ctx, access, nil
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
