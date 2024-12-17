// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"log"
	"strings"
)

// NetworkCreate represents `docker network create`.
type NetworkCreate struct {
	Name string // name of network to be created
}

func (c *NetworkCreate) Execute(ctx context.Context) (string, string, error) {
	args := []string{"network", "create", c.Name}

	dockerVersionCmd := DockerVersion{}
	versionStr, _, _ := dockerVersionCmd.Execute(ctx)
	log.Println("`docker -v` -> ", versionStr)

	if strings.Contains(strings.ToLower(versionStr), "podman") {
		args = append(args, "-o", "mtu=1450")
	} else {
		args = append(args, "-o", "com.docker.network.driver.mtu=1450")
	}

	return execute(ctx, dockerCmd, args)
}

// NetworkRemove represents `docker network remove`.
type NetworkRemove struct {
	Names []string // names (or ids) of network to be removed
}

func (c *NetworkRemove) Execute(ctx context.Context) (string, string, error) {
	args := []string{"network", "remove"}
	args = append(args, c.Names...)
	return execute(ctx, dockerCmd, args)
}

// NetworkInspect represents `docker network inspect`.
type NetworkInspect struct {
	Names  []string // names (or ids) of network to be inspected
	Format string   // value for the --format option
}

func (c *NetworkInspect) Execute(ctx context.Context) (string, string, error) {
	args := []string{"network", "inspect"}
	if strings.TrimSpace(c.Format) != "" {
		args = append(args, "-f", c.Format)
	}
	args = append(args, c.Names...)
	return execute(ctx, dockerCmd, args)
}

// NetworkList represents `docker network ls`.
type NetworkList struct {
	Names  []string // names (or ids) of network to be listed
	Format string   // value for the --format option. e.g. {{.ID}}
}

func (c *NetworkList) Execute(ctx context.Context) (string, string, error) {
	args := []string{"network", "ls"}
	if strings.TrimSpace(c.Format) != "" {
		args = append(args, "--format", c.Format)
	}
	for _, name := range c.Names {
		// Wrap name with ^ and $ for exact matching.
		args = append(args, "--filter", "name=^"+name+"$")
	}
	return execute(ctx, dockerCmd, args)
}
