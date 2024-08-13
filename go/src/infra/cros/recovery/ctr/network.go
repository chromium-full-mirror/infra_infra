// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ctr

// Network describe API to work with networks.
type Network interface {
	Name() string
}
type networkImpl struct {
	name string
}

// Name returns the name of container.
func (c *networkImpl) Name() string {
	return c.name
}
