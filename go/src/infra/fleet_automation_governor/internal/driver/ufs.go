// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package driver

import (
	"context"

	"go.chromium.org/infra/fleet_automation_governor/internal/dataframe"
)

type UFS struct {
}

func (u *UFS) Drive(ctx context.Context, t dataframe.DataFrame) error {
	return nil
}
