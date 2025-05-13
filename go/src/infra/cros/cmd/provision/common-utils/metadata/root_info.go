// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Plain Old Go Object for root disk information
package metadata

import commonutils "go.chromium.org/infra/cros/cmd/provision/common-utils"

// RootInfo stores Root information pertaining to a DUT
type RootInfo struct {
	Root          string
	RootDisk      string
	RootPartNum   string
	PartitionInfo *commonutils.PartitionInfo
}
