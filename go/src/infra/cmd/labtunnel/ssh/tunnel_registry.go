// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ssh

import (
	"encoding/json"
	"fmt"
	"strings"

	"infra/cmd/labtunnel/log"
)

// TunnelRegistry records mappings of remote hostnames to local hostnames that
// represent tunnels from the local to remote hostnames.
type TunnelRegistry struct {
	remoteToLocalHostnames map[string]string
}

func NewTunnelRegistry() *TunnelRegistry {
	return &TunnelRegistry{
		remoteToLocalHostnames: make(map[string]string),
	}
}

func (tr *TunnelRegistry) Set(remoteHostname, localHostname string) {
	remoteHostname = removeUsernameFromHostname(remoteHostname)
	localHostname = removeUsernameFromHostname(localHostname)
	if localHostname == "" {
		delete(tr.remoteToLocalHostnames, remoteHostname)
	}
	tr.remoteToLocalHostnames[remoteHostname] = localHostname
}

func (tr *TunnelRegistry) Get(remoteHostname string) (string, bool) {
	localHostname, ok := tr.remoteToLocalHostnames[remoteHostname]
	return localHostname, ok
}

func (tr *TunnelRegistry) String() string {
	marshalledJSON, err := json.Marshal(tr.remoteToLocalHostnames)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal remoteToLocalHostnames to JSON: %v", err))
	}
	return string(marshalledJSON)
}

func (tr *TunnelRegistry) PrintToLog() {
	log.Logger.Printf("Tunnel Registry (remote:local): %s\n", tr.String())
}

func removeUsernameFromHostname(hostname string) string {
	hostnameParts := strings.Split(hostname, "@")
	return hostnameParts[len(hostnameParts)-1]
}
