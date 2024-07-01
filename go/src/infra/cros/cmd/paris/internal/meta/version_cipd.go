// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"

	"go.chromium.org/luci/cipd/client/cipd"
	"go.chromium.org/luci/common/errors"
)

// describe returns information about a package instances.
func describe(ctx context.Context, pkg, version string) (*cipd.InstanceDescription, error) {
	client, err := cipd.NewClientFromEnv(ctx, cipd.ClientOptions{})
	if err != nil {
		return nil, errors.Annotate(err, "describe package").Err()
	}
	defer client.Close(ctx)
	pin, err := client.ResolveVersion(ctx, pkg, version)
	if err != nil {
		return nil, errors.Annotate(err, "describe package").Err()
	}
	d, err := client.DescribeInstance(ctx, pin, nil)
	if err != nil {
		return nil, errors.Annotate(err, "describe package").Err()
	}
	return d, nil
}
