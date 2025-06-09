// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package examples

import (
	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates"
	dynamiccommon "go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/generators"
)

// RemoveServiceForVMs demonstrates how one might remove a service that was added
// within the primary task list's dynamic updates by adding a dynamic update that removes
// the service for the VM only via the SecondaryDynamicUpdates.
func RemoveServiceForVMs(req *api.InternalTestplan) error {
	for _, schedOption := range req.SuiteInfo.GetSuiteMetadata().GetSchedulingUnitOptions() {
		for _, schedUnit := range schedOption.GetSchedulingUnits() {
			// Current implementation for VMs describes a single device environment.
			// So only the primary board needs to be addressed.
			board, ok := schedUnit.GetDynamicUpdateLookupTable()["board"]
			if !ok || !common.IsSupportedVMBoard(board) {
				continue
			}
			generator := generators.NewRemoveGenerator([]*api.FocalTaskFinder{
				dynamiccommon.FindByDynamicIdentifier("task_identifier_goes_here"),
			})
			return dynamic_updates.AppendUserDefinedDynamicUpdates(&schedUnit.SecondaryDynamicUpdates, generator.Generate)
		}
	}
	return nil
}
