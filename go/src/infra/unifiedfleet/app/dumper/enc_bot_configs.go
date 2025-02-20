// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dumper

import (
	"context"

	"go.chromium.org/infra/unifiedfleet/app/controller"
)

// getBotConfigs reads the bot configs.
//
// Bot configs are read from the files specified in the UFS config
// to get Ownership and security data needed by Puppet ENC
// which is stored in the datastore per bot(machine, vm etc).
func getBotConfigs(ctx context.Context) (retErr error) {
	defer func() {
		syncSecurityConfigsTick.Add(ctx, 1, retErr == nil)
	}()
	retErr = controller.ImportBotConfigs(ctx)
	return retErr
}
