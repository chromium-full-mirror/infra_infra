// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package recovery

import (
	"go.chromium.org/infra/cros/recovery/internal/localtlw"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// NewLocalTLWAccess provides instance of local implementation of TLW Access.
func NewLocalTLWAccess(ufs localtlw.UFSClient) (tlw.Access, error) {
	return localtlw.New(ufs)
}
