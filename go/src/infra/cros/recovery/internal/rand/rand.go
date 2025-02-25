// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package rand

import (
	"math/rand"
	"sync"
	"time"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var (
	seededRandMu sync.Mutex
	seededRand   *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))
)

// String generates randomg string with expected length.
func String(length int) string {
	return stringWithCharset(length, charset)
}

// stringWithCharset generates string based on charset and required length.
func stringWithCharset(length int, charset string) string {
	seededRandMu.Lock()
	defer seededRandMu.Unlock()
	b := make([]byte, length)
	for i := range b {
		n := seededRand.Intn(len(charset))
		b[i] = charset[n]
	}
	return string(b)
}
