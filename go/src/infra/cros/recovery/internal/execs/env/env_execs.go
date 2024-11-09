// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package env provide exec which based on environment variables.
package env

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"infra/cros/internal/env"
	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/namespace"
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
	if namespace.IsPartner(ctx) {
		return errors.Reason("is not cros partner namespace: is partner namespace detected").Err()
	}
	return nil
}

func init() {
	execs.Register("env_is_cloudbot", isCloudbot)
	execs.Register("env_is_not_cloudbot", isNotCloudbot)
	execs.Register("env_is_not_os_partner_namespace", isNotCrosPartnerNamespaceExec)
}
