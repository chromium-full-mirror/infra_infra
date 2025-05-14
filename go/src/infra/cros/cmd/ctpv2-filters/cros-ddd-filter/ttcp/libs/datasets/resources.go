// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package datasets

import (
	buildmetadata "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/buildmetadata"
	dlmmetadata "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/dlmmetadata"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/hwid/db"
)

type AllDatasetsResources struct {
	Buildmetadata *buildmetadata.BuildMetadataResources
	HwidDB        *db.HwidDbResources
	Dlmmetadata   *dlmmetadata.DlmResources
}
