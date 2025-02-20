// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package external

import (
	"context"

	"go.chromium.org/infra/appengine/cros/lab_inventory/app/frontend/fake"
)

// WithTestingContext allows for mocked external interface.
func WithTestingContext(ctx context.Context) context.Context {
	_, err := GetServerInterface(ctx)
	if err != nil {
		es := &InterfaceFactory{
			ufsInterfaceFactory: fakeUFSInterface,
		}
		return context.WithValue(ctx, InterfaceFactoryKey, es)
	}
	return ctx
}

func fakeUFSInterface(ctx context.Context, host string) (UFSClient, error) {
	return &fake.FleetClient{}, nil
}
