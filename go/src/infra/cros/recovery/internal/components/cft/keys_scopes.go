// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cft

func clientScopeKey(containerName string) string {
	return containerName + "-client"
}

func addressScopeKey(containerName string) string {
	return containerName + "-address"
}
