// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build !windows
// +build !windows

package cros

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPing(t *testing.T) {
	// Success case.
	assert.Nil(t, flexPingAMT("127.0.0.1", 1))
	// 192.0.2.0/24 is TEST-NET-1.
	assert.ErrorContains(t, flexPingAMT("192.0.2.1", 1), "failed to ping AMT")
}
