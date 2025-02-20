// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package btpeer

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/scopes"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// activeHost finds active host related to the executed plan.
func activeHost(resource string, chromeos *tlw.ChromeOS) (*tlw.BluetoothPeer, error) {
	for _, btp := range chromeos.GetBluetoothPeers() {
		if btp.GetName() == resource {
			return btp, nil
		}
	}
	return nil, errors.Reason("active host: host not found").Err()
}

func getBtpeerScopeState(ctx context.Context, info *execs.ExecInfo) (*tlw.BluetoothPeerScopeState, error) {
	btpeer, err := activeHost(info.GetActiveResource(), info.GetChromeos())
	if err != nil {
		return nil, errors.Annotate(err, "failed to get active btpeer host").Err()
	}
	btpeerScopeStateKey := "btpeer_scope_state/" + info.GetActiveResource()
	var btpeerScopeState *tlw.BluetoothPeerScopeState
	if state, ok := scopes.ReadConfigParam(ctx, btpeerScopeStateKey); !ok {
		btpeerScopeState = &tlw.BluetoothPeerScopeState{
			Btpeer:                 btpeer,
			Chameleond:             &tlw.BluetoothPeerScopeState_Chameleond{},
			RaspiosCrosBtpeerImage: &tlw.BluetoothPeerScopeState_RaspiosCrosBtpeerImage{},
		}
		scopes.PutConfigParam(ctx, btpeerScopeStateKey, btpeerScopeState)
	} else {
		btpeerScopeState, ok = state.(*tlw.BluetoothPeerScopeState)
		if !ok {
			return nil, errors.Reason("failed to cast config param with key %q to a BluetoothPeerScopeState", btpeerScopeStateKey).Err()
		}
		if btpeerScopeState.GetBtpeer() != btpeer {
			return nil, errors.Reason("BluetoothPeerScopeState.Btpeer does not match active btpeer host").Err()
		}
		if btpeerScopeState.GetChameleond() == nil {
			return nil, errors.Reason("BluetoothPeerScopeState.Chameleond is nil").Err()
		}
	}
	return btpeerScopeState, nil
}
