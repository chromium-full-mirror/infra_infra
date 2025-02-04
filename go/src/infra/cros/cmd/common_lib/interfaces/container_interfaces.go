// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package interfaces

import (
	"context"

	"go.chromium.org/chromiumos/config/go/test/api"
)

type ContainerType string

// ContainerInterface defines the contract a container will have to satisfy.
type ContainerInterface interface {
	// GetContainerType returns the container type.
	GetContainerType() ContainerType

	// Initialize initializes the container.
	Initialize(context.Context, *api.Template) error

	// StartContainer starts the container.
	StartContainer(context.Context) (*api.StartContainerResponse, error)

	// GetContainer gets the container related info.
	GetContainer(context.Context) (*api.GetContainerResponse, error)

	// ProcessContainer processes the container.
	// Ideally, this aggregates initialize, start and get together.
	ProcessContainer(context.Context, *api.Template) (string, error)

	// StopContainer stops the container.
	StopContainer(context.Context) error

	// GetLogsLocation returns the container activity logs location.
	GetLogsLocation() (string, error)
}

// AbstractContainer satisfies the container requirement that is common to all.
type AbstractContainer struct {
	ContainerInterface
}
