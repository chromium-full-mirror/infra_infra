// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package jobs

import (
	"context"

	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/device_manager/internal/frontend"
)

// PersistToBigQuery persists the current DM records to BigQuery.
func PersistToBigQuery(ctx context.Context, serviceClients frontend.ServiceClients, project string) error {
	logging.Debugf(ctx, "Starting BQ job to persist DM records to project %s", project)
	return nil
}
