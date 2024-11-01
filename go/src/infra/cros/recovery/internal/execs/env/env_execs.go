// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package env provide exec which based on environment variables.
package env

import (
	"context"

	"google.golang.org/grpc/metadata"

	"go.chromium.org/luci/common/errors"

	"infra/cros/internal/env"
	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
	ufsUtil "infra/unifiedfleet/app/util"
)

func isCloudbot(ctx context.Context, info *execs.ExecInfo) error {
	if env.IsCloudBot() {
		log.Debugf(ctx, "That is cloudbot enviroment!")
		return nil
	}
	return errors.Reason("is cloudbot: that is not cloubot").Err()
}

func isNotCloudbot(ctx context.Context, info *execs.ExecInfo) error {
	if env.IsCloudBot() {
		return errors.Reason("is not cloudbot: that is cloubot").Err()
	}
	log.Debugf(ctx, "That is not cloudbot enviroment!")
	return nil
}

func isNotCrosPartnerNamespaceExec(ctx context.Context, info *execs.ExecInfo) error {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		return errors.Reason("is cros partner namespace: no metadata found in context").Err()
	}
	for _, nsValue := range md.Get(ufsUtil.Namespace) {
		log.Debugf(ctx, "Context namespace value: %q", nsValue)
		if nsValue == ufsUtil.OSPartnerNamespace {
			return errors.Reason("is not cros partner namespace: detected %q value", ufsUtil.OSPartnerNamespace).Err()
		}
	}
	return nil
}

func init() {
	execs.Register("env_is_cloudbot", isCloudbot)
	execs.Register("env_is_not_cloudbot", isNotCloudbot)
	execs.Register("env_is_not_os_partner_namespace", isNotCrosPartnerNamespaceExec)
}
