// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"flag"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/server"

	"go.chromium.org/infra/vm_leaser/internal/cron"
)

// stringListFlags is binded as an array flag usable in command line
type stringListFlags []string

func (i *stringListFlags) String() string {
	return "string"
}

func (i *stringListFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

var gcpProjects stringListFlags

func main() {
	flag.Var(
		&gcpProjects,
		"gcp-projects",
		"The GCP projects where VMs should be managed",
	)

	srvEnv := flag.String(
		"service-env",
		"dev",
		"The service deployment environment (dev/prod).",
	)

	server.Main(nil, nil, func(srv *server.Server) error {
		logging.Infof(srv.Context, "Registering cron server.")
		logging.Infof(srv.Context, "Starting VM lifecycle management for %v GCP projects", gcpProjects)

		opts := cron.Options{
			GcpProjects: gcpProjects,
			ServiceEnv:  srvEnv,
		}
		cron.RegisterCronServer(srv, opts)
		return nil
	})
}
