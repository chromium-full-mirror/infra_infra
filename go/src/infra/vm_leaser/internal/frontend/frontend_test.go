// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"errors"
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/vm_leaser/internal/constants"
)

func TestHandleLeaseVMError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	var allZones []string
	for _, a := range constants.AllQuotaZones {
		allZones = append(allZones, a...)
	}
	ftt.Run("Test handleLeaseVMError", t, func(t *ftt.Test) {
		t.Run("handleLeaseVMError - no error; return original request", func(t *ftt.Test) {
			req := &api.LeaseVMRequest{
				HostReqs: &api.VMRequirements{
					GceRegion:  "test-region",
					GceProject: "test-project",
				},
			}
			newReq := handleLeaseVMError(ctx, req, nil, nil)
			assert.Loosely(t, req, should.Match(newReq))
		})
		t.Run("handleLeaseVMError - QUOTA_EXCEEDED error; return request with new zone", func(t *ftt.Test) {
			req := &api.LeaseVMRequest{
				HostReqs: &api.VMRequirements{
					GceRegion:  "test-region",
					GceProject: "test-project",
				},
			}
			err := errors.New("QUOTA_EXCEEDED error test")
			quotaExceededZones := map[string]bool{}
			newReq := handleLeaseVMError(ctx, req, err, quotaExceededZones)
			assert.Loosely(t, newReq.GetHostReqs().GetGceRegion(), should.NotEqual("test-region"))
			assert.Loosely(t, newReq.GetHostReqs().GetGceRegion(), should.BeIn(allZones...))
		})
	})
}
