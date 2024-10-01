// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cli implement cli execution of adb-service API.
package cli

import (
	"context"

	"go.chromium.org/luci/common/errors"
)

type clier struct {
}

func New() *clier {
	return &clier{}
}

func (v *clier) Run(ctx context.Context) error {
	return errors.Reason("parse args: cli isn't supported yet!").Err()
}
