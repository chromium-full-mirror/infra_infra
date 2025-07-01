// Copyright 2021 The Chromium Authors
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
		return nil, errors.Fmt("describe package: %w", err)
	}
	defer client.Close(ctx)
	pin, err := client.ResolveVersion(ctx, pkg, version)
	if err != nil {
		return nil, errors.Fmt("describe package: %w", err)
	}
	d, err := client.DescribeInstance(ctx, pin, nil)
	if err != nil {
		return nil, errors.Fmt("describe package: %w", err)
	}
	return d, nil
}
