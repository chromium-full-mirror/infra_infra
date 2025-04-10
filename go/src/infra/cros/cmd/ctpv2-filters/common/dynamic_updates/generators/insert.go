// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package generators

import (
	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/interfaces"
)

type InsertGenerator struct {
	DynamicUpdates []*api.UserDefinedDynamicUpdate
}

// NewInsertGenerator initializes.
func NewInsertGenerator() *InsertGenerator {
	return &InsertGenerator{
		DynamicUpdates: []*api.UserDefinedDynamicUpdate{},
	}
}

// AddInsertion converts the task into an insert update action
// and appends it to the dynamic updates.
func (gen *InsertGenerator) AddInsertion(
	task *api.CrosTestRunnerDynamicRequest_Task,
	insertInstruction interfaces.InsertInstructionGetter) {

	instructions := insertInstruction(task)
	gen.DynamicUpdates = append(gen.DynamicUpdates, &api.UserDefinedDynamicUpdate{
		FocalTaskFinder: instructions.FocalTaskFinder,
		UpdateAction:    instructions.UpdateAction,
	})
}

// Generate returns the DynamicUpdates.
func (gen *InsertGenerator) Generate() ([]*api.UserDefinedDynamicUpdate, error) {
	return gen.DynamicUpdates, nil
}
