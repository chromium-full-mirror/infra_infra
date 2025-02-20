// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package localproxy

import (
	"context"
	"fmt"

	"go.chromium.org/infra/cros/recovery/internal/localtlw/ssh"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

var (
	// Mapping resource name to proxy address.
	hostProxyAddresses = map[string]string{}
	// Incrementally port number used to track which port will be used next.
	lastUsedProxyPort = 2500
)

// RegHost registers the hostname for proxy connections map.
// If hostname i snot known then new proxy will be created and register to map.
func RegHost(ctx context.Context, hostname string, jumpHostname string) error {
	if _, ok := hostProxyAddresses[hostname]; !ok {
		p := newProxy(ctx, hostname, lastUsedProxyPort, jumpHostname)
		if p.Port() == lastUsedProxyPort {
			lastUsedProxyPort++
		}
		SetHostProxyAddress(ctx, hostname, fmt.Sprintf("127.0.0.1:%d", p.Port()))
	}
	return nil
}

// SetHostProxyAddress sets the proxy address for the hostname.
func SetHostProxyAddress(ctx context.Context, hostname string, localProxyAddress string) {
	log.Infof(ctx, "Set hostname %q as a proxy address for hostname %q", localProxyAddress, hostname)
	hostProxyAddresses[hostname] = localProxyAddress
}

// BuildAddr creates address for SSH access for execution.
// If host is present in the hostProxyAddresses then the proxy address will be
// used instead of the provided hostname.
func BuildAddr(hostname string) string {
	proxyAddress, ok := hostProxyAddresses[hostname]
	if ok {
		return proxyAddress
	}
	return fmt.Sprintf("%s:%d", hostname, ssh.DefaultPort)
}
