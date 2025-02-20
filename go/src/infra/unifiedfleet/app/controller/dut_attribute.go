// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/unifiedfleet/app/model/configuration"
)

// GetDutAttribute returns DutAttribute for the given DutAttribute ID from datastore.
func GetDutAttribute(ctx context.Context, id string) (*api.DutAttribute, error) {
	return configuration.GetDutAttribute(ctx, id)
}

// ListDutAttributes lists the DutAttributes from datastore.
func ListDutAttributes(ctx context.Context, keysOnly bool) ([]*api.DutAttribute, error) {
	return configuration.ListDutAttributes(ctx, keysOnly)
}
