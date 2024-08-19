// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package external

import (
	"context"

	"infra/cros/hwid"
	"infra/libs/git"
	"infra/libs/sheet"
	"infra/unifiedfleet/app/frontend/fake"
)

// WithTestingContext allows for mocked external interface.
func WithTestingContext(ctx context.Context) context.Context {
	_, err := GetServerInterface(ctx)
	if err != nil {
		es := &InterfaceFactory{
			sheetInterfaceFactory:    fakeSheetInterfaceFactory,
			gitInterfaceFactory:      fakeGitInterfaceFactory,
			gitTilesInterfaceFactory: fakeGitTilesInterfaceFactory,
			hwidInterfaceFactory:     fakeHwidInterfaceFactory,
			deviceConfigFactory:      fakeDeviceConfigFactory,
		}
		return context.WithValue(ctx, InterfaceFactoryKey, es)
	}
	return ctx
}

func fakeSheetInterfaceFactory(ctx context.Context) (sheet.ClientInterface, error) {
	return &fake.SheetClient{}, nil
}

func fakeGitInterfaceFactory(ctx context.Context, gitilesHost, project, branch string) (git.ClientInterface, error) {
	return &fake.GitClient{}, nil
}

func fakeGitTilesInterfaceFactory(ctx context.Context, gitilesHost string) (GitTilesClient, error) {
	return &fake.GitTilesClient{}, nil
}

func fakeHwidInterfaceFactory(ctx context.Context) (hwid.ClientInterface, error) {
	return &fake.HwidClient{}, nil
}

func fakeDeviceConfigFactory(ctx context.Context, invHost string) (DeviceConfigClient, error) {
	return &fake.DeviceConfigClient{}, nil
}
