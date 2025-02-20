// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package execs

import (
	"context"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

// DefaultPinger returns pinger for current resource name specified per plan.
func (ei *ExecInfo) DefaultPinger() components.Pinger {
	return ei.NewPinger(ei.runArgs.ResourceName)
}

// NewPinger returns pinger for requested resource.
func (ei *ExecInfo) NewPinger(resource string) components.Pinger {
	pinger := func(ctx context.Context, count int) error {
		log.Debugf(ctx, "Start ping %q %d times", resource, count)
		return ei.runArgs.Access.Ping(ctx, resource, count)
	}
	return pinger
}
