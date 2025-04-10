// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dynamic_updates

import (
	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/interfaces"
)

// AppendUserDefinedDynamicUpdates calles the provided generator function and appends the
// dynamic updates the to parent list of dynamic updates.
func AppendUserDefinedDynamicUpdates(currentUpdates *[]*api.UserDefinedDynamicUpdate, generateFunc interfaces.UserDefinedDynamicUpdateGenerator) error {
	dynamicUpdates, err := generateFunc()
	if err != nil {
		return err
	}

	*currentUpdates = append(*currentUpdates, dynamicUpdates...)
	return nil
}
