// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package recovery

// These package imports register exec functions for the execs package.
import (
	_ "go.chromium.org/infra/cros/recovery/internal/execs/amt"
	_ "go.chromium.org/infra/cros/recovery/internal/execs/android"
	_ "go.chromium.org/infra/cros/recovery/internal/execs/btpeer"
	_ "go.chromium.org/infra/cros/recovery/internal/execs/chameleon"
	_ "go.chromium.org/infra/cros/recovery/internal/execs/cros"
	_ "go.chromium.org/infra/cros/recovery/internal/execs/ctr"
	_ "go.chromium.org/infra/cros/recovery/internal/execs/dolos"
	_ "go.chromium.org/infra/cros/recovery/internal/execs/dut"
	_ "go.chromium.org/infra/cros/recovery/internal/execs/env"
	_ "go.chromium.org/infra/cros/recovery/internal/execs/human_motion_robot"
	_ "go.chromium.org/infra/cros/recovery/internal/execs/metrics"
	_ "go.chromium.org/infra/cros/recovery/internal/execs/rpm"
	_ "go.chromium.org/infra/cros/recovery/internal/execs/servo"
	_ "go.chromium.org/infra/cros/recovery/internal/execs/stableversion"
	_ "go.chromium.org/infra/cros/recovery/internal/execs/wifirouter"
)
