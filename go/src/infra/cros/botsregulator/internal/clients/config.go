// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package clients

import (
	"context"

	"go.chromium.org/luci/config"
	"go.chromium.org/luci/config/cfgclient"
)

// MockConfigClientKey is used for testing.
var MockConfigClientKey contextKey = "used in tests only for setting the mock ConfigClient"

func NewConfigClient(ctx context.Context) config.Interface {
	if mockClient, ok := ctx.Value(MockConfigClientKey).(config.Interface); ok {
		return mockClient
	}
	return cfgclient.Client(ctx)
}
