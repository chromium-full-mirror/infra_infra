// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package generators

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/common"
)

type ModifyGenerator struct {
	FocalTaskFinder *api.FocalTaskFinder
	Modifications   []*api.UpdateAction_Modify_Modification

	DynamicUpdates []*api.UserDefinedDynamicUpdate
}

// NewModifyGenerate initializes.
func NewModifyGenerator(focalTaskFinder *api.FocalTaskFinder) *ModifyGenerator {
	return &ModifyGenerator{
		FocalTaskFinder: focalTaskFinder,
		Modifications:   []*api.UpdateAction_Modify_Modification{},
		DynamicUpdates:  []*api.UserDefinedDynamicUpdate{},
	}
}

// AddModification converts the payload and instructions into a modify update action
// and appends it to the dynamic updates.
func (gen *ModifyGenerator) AddModification(payload proto.Message, instructions map[string]string) error {
	payloadAny, err := anypb.New(payload)
	if err != nil {
		return errors.Annotate(err, "failed to marshal modification payload to any, %v", payload).Err()
	}

	gen.Modifications = append(gen.Modifications, &api.UpdateAction_Modify_Modification{
		Payload:      payloadAny,
		Instructions: instructions,
	})
	return nil
}

// Generate converts all the modifications to dynamic updates and returns.
func (gen *ModifyGenerator) Generate() ([]*api.UserDefinedDynamicUpdate, error) {
	gen.DynamicUpdates = append(gen.DynamicUpdates, common.ModifyActionWrapper(gen.FocalTaskFinder)(gen.Modifications))

	return gen.DynamicUpdates, nil
}
