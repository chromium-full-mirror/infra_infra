// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cache enables interaction with the cache server.
package cache

import (
	"fmt"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
)

// IPEndpointToHostPort formats an IpEndpoint proto message into a "host:port" string.
func IPEndpointToHostPort(i *labapi.IpEndpoint) (string, error) {
	if i.GetAddress() == "" {
		return "", fmt.Errorf("IpEndpoint missing address")
	}
	if i.GetPort() == 0 {
		return "", fmt.Errorf("IpEndpoint missing port")
	}
	return fmt.Sprintf("%s:%d", i.GetAddress(), i.GetPort()), nil
}
