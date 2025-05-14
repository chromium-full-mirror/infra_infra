// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package deviceinfo

import (
	buildmetadata "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/buildmetadata"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/targetproperties"
)

type TargetVariant struct {
	DeviceId    string
	BuildTarget *buildmetadata.BuildTarget
	Properties  *targetproperties.TargetPropertiesValues
}

type TargetId string

func (t *TargetVariant) Id() TargetId {
	return TargetId(t.DeviceId + "|" + t.BuildTarget.Id())
}

func (t *TargetVariant) Clone() *TargetVariant {
	return &TargetVariant{
		DeviceId:    t.DeviceId,
		BuildTarget: t.BuildTarget,
		Properties:  t.Properties.Clone(),
	}

}
