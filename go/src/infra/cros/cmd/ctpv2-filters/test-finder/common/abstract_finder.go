// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package common is the common package.
package common

import (
	"context"
	"log"

	"go.chromium.org/chromiumos/config/go/test/api"
)

type FinderHarness string

// FinderInterface defines the contract an executor will have to satisfy.
type FinderInterface interface {
	// GetFinderHarness returns the executor type.
	GetFinderHarness() FinderHarness

	// ExecuteCommand executes the provided command via current executor.
	FindTestsAB() (*api.InternalTestplan, error)
}

// AbstractFinder satisfies the executor requirement that is common to all.
type AbstractFinder struct {
	FinderInterface

	FinderHarness FinderHarness

	Testplan *api.InternalTestplan
	ctx      context.Context
	Logger   *log.Logger
}

// NewAbstractFinder provides a new Abstract Finder
func NewAbstractFinder(ctx context.Context, req *api.InternalTestplan, exType FinderHarness, log *log.Logger) *AbstractFinder {
	return &AbstractFinder{
		FinderHarness: exType,
		Testplan:      req,
		ctx:           ctx,
		Logger:        log,
	}
}

// GetFinderHarness returns the type of the FinderHarness.
func (ex *AbstractFinder) GetFinderHarness() FinderHarness {
	return ex.FinderHarness
}
