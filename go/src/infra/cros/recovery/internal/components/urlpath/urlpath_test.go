// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package urlpath

import (
	"context"
	"testing"

	"go.chromium.org/infra/cros/recovery/scopes"
)

var enrichWithTrackingIdsCases = []struct {
	testName    string
	input       string
	out         string
	expectedErr bool
	swarmingId  string
	bbid        string
}{
	{
		"Empty",
		"",
		"",
		false,
		"",
		"",
	},
	{
		"full 1",
		"postgres://user:pass@host.com:5432/path?k=v#f",
		"postgres://user:pass@host.com:5432/swarming/sw1/bbid/bb1/path?k=v#f",
		false,
		"sw1",
		"bb1",
	},
	{
		"full 2",
		"postgres://user:pass@host.com:5432/download/chromeos-image-archive/board-release/R99-XXXXX.XX.0/image.bin?k=v#f",
		"postgres://user:pass@host.com:5432/swarming/sw2/bbid/bb3/download/chromeos-image-archive/board-release/R99-XXXXX.XX.0/image.bin?k=v#f",
		false,
		"sw2",
		"bb3",
	},
	{
		"full 3 (without ids)",
		"postgres://user:pass@host.com:5432/download/chromeos-image-archive/board-release/R99-XXXXX.XX.0/image.bin?k=v#f",
		"postgres://user:pass@host.com:5432/swarming/none/bbid/none/download/chromeos-image-archive/board-release/R99-XXXXX.XX.0/image.bin?k=v#f",
		false,
		"",
		"",
	},
}

func TestEnrichWithTrackingIds(t *testing.T) {
	t.Parallel()
	for _, tt := range enrichWithTrackingIdsCases {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			params := scopes.GetParamCopy(ctx)
			params[scopes.ParamKeySwarmingTaskID] = tt.swarmingId
			params[scopes.ParamKeyBuildbucketID] = tt.bbid
			ctx = scopes.WithParams(ctx, params)
			got, actualErr := EnrichWithTrackingIds(ctx, tt.input)
			if tt.expectedErr {
				if actualErr == nil {
					t.Errorf("Case %q: expected error but got nil", tt.testName)
				}
			} else if actualErr != nil {
				t.Errorf("Case %q: does not expected error but got %q", tt.testName, actualErr)

			} else if got != tt.out {
				t.Errorf("EnrichWithTrackingIds(%q) = %q, want %q", tt.testName, tt.out, got)
			}
		})
	}
}
