// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"

	"go.chromium.org/luci/common/errors"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/model/configuration"
)

// ListOSes lists the chrome os_version
func ListOSes(ctx context.Context, pageSize int32, pageToken string, filter string, keysOnly bool) ([]*ufspb.OSVersion, string, error) {
	var filterMap map[string][]any
	var err error
	if filter != "" {
		filterMap, err = getFilterMap(filter, configuration.GetOSVersionIndexedFieldName)
		if err != nil {
			return nil, "", errors.Annotate(err, "Failed to read filter for listing os versions").Err()
		}
	}
	return configuration.ListOSes(ctx, pageSize, pageToken, filterMap, keysOnly)
}
