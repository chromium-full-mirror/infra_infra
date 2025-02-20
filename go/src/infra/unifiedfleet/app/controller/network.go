// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/model/configuration"
)

// get a number of free IPs.
func getFreeIP(ctx context.Context, vlanName string, pageSize int) ([]*ufspb.IP, error) {
	var out []*ufspb.IP
	err := configuration.RunFreeIPs(ctx, vlanName, func(ip *ufspb.IP) (bool, error) {
		if len(out) >= pageSize {
			return false, nil
		}
		out = append(out, ip)
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
