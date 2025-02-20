// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package env provide exec which based on environment variables.
package env

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/internal/env"
	"go.chromium.org/infra/cros/recovery/internal/components/mh"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/namespace"
)

func isCloudbotExec(ctx context.Context, info *execs.ExecInfo) error {
	if env.IsCloudBot() {
		log.Debugf(ctx, "That is cloudbot enviroment!")
		return nil
	}
	return errors.Reason("is cloudbot: that is not cloubot").Err()
}

func isNotCloudbotExec(ctx context.Context, info *execs.ExecInfo) error {
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

func isMHBoxExec(ctx context.Context, info *execs.ExecInfo) error {
	if mh.IsMH() {
		log.Debugf(ctx, "That is MH box enviroment!")
		return nil
	}
	return errors.Reason("is MH box: that is not MH box").Err()
}

func isNotMHBoxExec(ctx context.Context, info *execs.ExecInfo) error {
	if mh.IsMH() {
		return errors.Reason("is not MH box: that is MH box").Err()
	}
	log.Debugf(ctx, "That is not MH box enviroment!")
	return nil
}

func init() {
	execs.Register("env_is_cloudbot", isCloudbotExec)
	execs.Register("env_is_not_cloudbot", isNotCloudbotExec)
	execs.Register("env_is_not_os_partner_namespace", isNotCrosPartnerNamespaceExec)
	execs.Register("env_is_mh_box", isMHBoxExec)
	execs.Register("env_is_not_mh_box", isNotMHBoxExec)
}
