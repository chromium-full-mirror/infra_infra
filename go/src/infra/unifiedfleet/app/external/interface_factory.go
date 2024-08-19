// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package external

import (
	"context"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authclient "go.chromium.org/luci/auth"
	gitilesapi "go.chromium.org/luci/common/api/gitiles"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/server/auth"

	invV2Api "infra/appengine/cros/lab_inventory/api/v1"
	"infra/cros/hwid"
	"infra/libs/git"
	"infra/libs/sheet"
	"infra/unifiedfleet/app/config"
	"infra/unifiedfleet/app/util"
)

const (
	defaultCfgService = "luci-config.appspot.com"
	hwidEndpointScope = "https://www.googleapis.com/auth/chromeoshwid"
)

var spreadSheetScope = []string{authclient.OAuthScopeEmail, "https://www.googleapis.com/auth/spreadsheets", "https://www.googleapis.com/auth/drive.readonly"}

// InterfaceFactoryKey is the key used to store instance of InterfaceFactory in context.
var InterfaceFactoryKey = util.Key("ufs external-server-interface key")

// SheetInterfaceFactory is a constructor for a sheet.ClientInterface
type SheetInterfaceFactory func(ctx context.Context) (sheet.ClientInterface, error)

// GitInterfaceFactory is a constructor for a git.ClientInterface
type GitInterfaceFactory func(ctx context.Context, gitilesHost, project, branch string) (git.ClientInterface, error)

// GitTilesInterfaceFactory is a constructor for a gitiles.GitilesClient
type GitTilesInterfaceFactory func(ctx context.Context, gitilesHost string) (GitTilesClient, error)

// HwidInterfaceFactory is a constructor for a HWIDClient
type HwidInterfaceFactory func(ctx context.Context) (hwid.ClientInterface, error)

// DeviceConfigFactory is a constructor for a DeviceConfigClient
type DeviceConfigFactory func(ctx context.Context, inventoryHost string) (DeviceConfigClient, error)

// InterfaceFactory provides a collection of interfaces to external clients.
type InterfaceFactory struct {
	sheetInterfaceFactory    SheetInterfaceFactory
	gitInterfaceFactory      GitInterfaceFactory
	hwidInterfaceFactory     HwidInterfaceFactory
	gitTilesInterfaceFactory GitTilesInterfaceFactory
	deviceConfigFactory      DeviceConfigFactory
}

// GetServerInterface retrieves the ExternalServerInterface from context.
func GetServerInterface(ctx context.Context) (*InterfaceFactory, error) {
	if esif := ctx.Value(InterfaceFactoryKey); esif != nil {
		return esif.(*InterfaceFactory), nil
	}
	return nil, errors.Reason("InterfaceFactory not initialized in context").Err()
}

// WithServerInterface adds the external server interface to context.
func WithServerInterface(ctx context.Context) context.Context {
	return context.WithValue(ctx, InterfaceFactoryKey, &InterfaceFactory{
		sheetInterfaceFactory:    sheetInterfaceFactoryImpl,
		gitInterfaceFactory:      gitInterfaceFactoryImpl,
		gitTilesInterfaceFactory: gitTilesInterfaceFactoryImpl,
		hwidInterfaceFactory:     hwidInterfaceFactoryImpl,
		deviceConfigFactory:      deviceConfigFactoryImpl,
	})
}

// NewSheetInterface creates a new Sheet interface.
func (es *InterfaceFactory) NewSheetInterface(ctx context.Context) (sheet.ClientInterface, error) {
	if es.sheetInterfaceFactory == nil {
		es.sheetInterfaceFactory = sheetInterfaceFactoryImpl
	}
	return es.sheetInterfaceFactory(ctx)
}

func sheetInterfaceFactoryImpl(ctx context.Context) (sheet.ClientInterface, error) {
	// Testing sheet-access@unified-fleet-system-dev.iam.gserviceaccount.com, if works, will move it to config file.
	sheetSA := config.Get(ctx).GetSheetServiceAccount()
	if sheetSA == "" {
		return nil, status.Errorf(codes.FailedPrecondition, "sheet service account is not specified in config")
	}
	t, err := auth.GetRPCTransport(ctx, auth.AsActor, auth.WithServiceAccount(sheetSA), auth.WithScopes(spreadSheetScope...))
	if err != nil {
		return nil, err
	}
	return sheet.NewClient(ctx, &http.Client{Transport: t})
}

// NewGitInterface creates a new git interface.
func (es *InterfaceFactory) NewGitInterface(ctx context.Context, gitilesHost, project, branch string) (git.ClientInterface, error) {
	if es.gitInterfaceFactory == nil {
		es.gitInterfaceFactory = gitInterfaceFactoryImpl
	}
	return es.gitInterfaceFactory(ctx, gitilesHost, project, branch)
}

func gitInterfaceFactoryImpl(ctx context.Context, gitilesHost, project, branch string) (git.ClientInterface, error) {
	t, err := auth.GetRPCTransport(ctx, auth.AsSelf, auth.WithScopes(authclient.OAuthScopeEmail, gitilesapi.OAuthScope))
	if err != nil {
		return nil, err
	}
	return git.NewClient(ctx, &http.Client{Transport: t}, "", gitilesHost, project, branch)
}

// NewGitTilesInterface creates a new git interface.
func (es *InterfaceFactory) NewGitTilesInterface(ctx context.Context, gitilesHost string) (GitTilesClient, error) {
	if es.gitInterfaceFactory == nil {
		es.gitInterfaceFactory = gitInterfaceFactoryImpl
	}
	return es.gitTilesInterfaceFactory(ctx, gitilesHost)
}

func gitTilesInterfaceFactoryImpl(ctx context.Context, gitilesHost string) (GitTilesClient, error) {
	t, err := auth.GetRPCTransport(ctx, auth.AsSelf, auth.WithScopes(authclient.OAuthScopeEmail, gitilesapi.OAuthScope))
	if err != nil {
		return nil, err
	}
	return gitilesapi.NewRESTClient(&http.Client{Transport: t}, gitilesHost, true)
}

// NewHwidClientInterface creates a new Hwid server client interface.
func (es *InterfaceFactory) NewHwidClientInterface(ctx context.Context) (hwid.ClientInterface, error) {
	if es.hwidInterfaceFactory == nil {
		es.hwidInterfaceFactory = hwidInterfaceFactoryImpl
	}
	return es.hwidInterfaceFactory(ctx)
}

func hwidInterfaceFactoryImpl(ctx context.Context) (hwid.ClientInterface, error) {
	hwidSA := config.Get(ctx).GetHwidServiceAccount()
	if hwidSA == "" {
		return nil, status.Errorf(codes.FailedPrecondition, "hwid service account is not specified in config")
	}
	t, err := auth.GetRPCTransport(ctx, auth.AsActor, auth.WithServiceAccount(hwidSA), auth.WithScopes(authclient.OAuthScopeEmail, hwidEndpointScope))
	if err != nil {
		return nil, err
	}
	return &hwid.Client{
		Hc: &http.Client{Transport: t},
	}, nil
}

// NewDeviceConfigInterfaceFactory creates a new device config client
func (es *InterfaceFactory) NewDeviceConfigInterfaceFactory(ctx context.Context, inventoryHost string) (DeviceConfigClient, error) {
	if es.deviceConfigFactory == nil {
		es.deviceConfigFactory = deviceConfigFactoryImpl
	}
	return es.deviceConfigFactory(ctx, inventoryHost)
}

func deviceConfigFactoryImpl(ctx context.Context, inventoryHost string) (DeviceConfigClient, error) {
	t, err := auth.GetRPCTransport(ctx, auth.AsCredentialsForwarder)
	if err != nil {
		return nil, err
	}

	ic := invV2Api.NewInventoryPRPCClient(&prpc.Client{
		C:    &http.Client{Transport: t},
		Host: inventoryHost,
	})

	return &DualDeviceConfigClient{
		inventoryClient: ic,
	}, nil
}
