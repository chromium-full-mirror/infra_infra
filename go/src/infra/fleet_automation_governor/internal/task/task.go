// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// package task defines a configurable task.
package task

import (
	"context"
	"fmt"

	"go.chromium.org/infra/fleet_automation_governor/internal/dataframe"
	"go.chromium.org/infra/fleet_automation_governor/internal/driver"
	"go.chromium.org/infra/fleet_automation_governor/internal/fetcher"
)

type Fetcher interface {
	// Fetcher defines an interface for fetching data from a source.
	Fetch(context.Context) (dataframe.DataFrame, error)
}

type Driver interface {
	// Driver defines the interface for data drivers.
	Drive(context.Context, dataframe.DataFrame) error
}

type Task struct {
	// Name is the name of the task.
	Name string
}

// NewTask creates a new task.
func NewTask(ctx context.Context, name string) (*Task, error) {
	return &Task{
		Name: name,
	}, nil
}

// Run executes the task.
func (t *Task) Run(ctx context.Context) error {
	f := &fetcher.Swarmning{}
	df, err := f.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch: %w", err)
	}
	d := &driver.UFS{}
	if err := d.Drive(ctx, df); err != nil {
		return fmt.Errorf("failed to drive: %w", err)
	}

	return nil
}
