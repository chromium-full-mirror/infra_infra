// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package bols contains common implementation of bols.
package bols

import (
	"go.chromium.org/chromiumos/config/go/test/api/bols"
)

// Service holds common implementation of BOLS service.
type Service struct {
	bols.UnimplementedBolsServiceServer
}
