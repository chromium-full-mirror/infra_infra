// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"fmt"

	"go.chromium.org/chromiumos/config/go/test/api"
)

func ValidateTestPlans(input *api.InternalTestplan, output *api.InternalTestplan) error {
	if output == nil {
		return fmt.Errorf("filter produced empty output")
	}
	// TODO: Add real validations.
	return nil
}
