// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package fetcher

import (
	"context"

	"go.chromium.org/infra/fleet_automation_governor/internal/dataframe"
)

type Swarmning struct {
}

func (s *Swarmning) Fetch(ctx context.Context) (dataframe.DataFrame, error) {
	return nil, nil
}
