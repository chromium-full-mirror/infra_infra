// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package generators

import (
	"go.chromium.org/chromiumos/config/go/test/api"
)

type RemoveGenerator struct {
	FocalTaskFinders []*api.FocalTaskFinder

	DynamicUpdates []*api.UserDefinedDynamicUpdate
}

// NewRemoveGenerator initializes.
func NewRemoveGenerator(focalTaskFinders []*api.FocalTaskFinder) *RemoveGenerator {
	return &RemoveGenerator{
		FocalTaskFinders: focalTaskFinders,
		DynamicUpdates:   []*api.UserDefinedDynamicUpdate{},
	}
}

// Generate converts the focal task finders into remove update actions
// and returns the dynamic updates.
func (gen *RemoveGenerator) Generate() ([]*api.UserDefinedDynamicUpdate, error) {
	for _, focalTaskFinder := range gen.FocalTaskFinders {
		gen.DynamicUpdates = append(gen.DynamicUpdates, &api.UserDefinedDynamicUpdate{
			FocalTaskFinder: focalTaskFinder,
			UpdateAction: &api.UpdateAction{
				Action: &api.UpdateAction_Remove_{
					Remove: &api.UpdateAction_Remove{},
				},
			},
		})
	}

	return gen.DynamicUpdates, nil
}
