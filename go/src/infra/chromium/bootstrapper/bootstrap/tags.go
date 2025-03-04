// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bootstrap

import (
	"time"

	"go.chromium.org/luci/common/errors/errtag"
)

var (
	// PatchRejected indicates that some portion of a patch was rejected.
	PatchRejected = errtag.Make("the patch could not be applied", true)
	// SleepBeforeExiting indicates that the top-level code should sleep before returning
	// control to the calling process, with the duration of the sleep being the tag's value.
	SleepBeforeExiting = errtag.Make("the properties file does not exist in the dependency project", time.Duration(0))
)
