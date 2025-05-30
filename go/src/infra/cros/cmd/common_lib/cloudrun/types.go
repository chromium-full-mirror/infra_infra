// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cloudrun provides cloud run specific functions.
package cloudrun

import "strings"

const (
	DefaultCPU = "4"
	DefaultMEM = "2Gi"
)

type Config struct {
	// CPU defaults to "4".
	// Can be 1, 2, 4, or 8.
	// For valid minimum CPU per memory see
	// https://cloud.google.com/run/docs/configuring/services/memory-limits#cpu-minimum
	CPU string
	// Memory defaults to "2Gi"
	// Memory Limit. Ex: 1024Mi, 4Gi.
	// For valid minimum memory per CPU see
	// https://cloud.google.com/run/docs/configuring/services/cpu#cpu-memory
	Memory string
}

func (config *Config) ToArgs() []string {
	args := []string{}

	if config.CPU == "" {
		config.CPU = DefaultCPU
	}
	args = append(args, "--cpu", config.CPU)

	if config.Memory == "" {
		config.Memory = DefaultMEM
	}
	args = append(args, "--memory", config.Memory)

	return args
}

type Target struct {
	Project string
	Region  string
	Name    string
}

type Container struct {
	Name    string
	Image   string
	Command string
	Args    []string
	Port    string
	Config  *Config
	Tag     string
}

func (container *Container) ToArgs() []string {
	args := []string{}

	if container.Tag != "" {
		args = append(args, "--tag", container.Tag, "--no-traffic")
	}

	args = append(args, []string{
		"--container", container.Name,
		"--image", container.Image,
		"--command", container.Command,
		"--args", strings.Join(container.Args, ","),
		"--port", container.Port,
		"--use-http2",
	}...)

	args = append(args, container.Config.ToArgs()...)

	return args
}
