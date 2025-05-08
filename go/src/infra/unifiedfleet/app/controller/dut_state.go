// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	chromeosLab "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/lab"
	"go.chromium.org/infra/unifiedfleet/app/config"
	"go.chromium.org/infra/unifiedfleet/app/model/inventory"
	"go.chromium.org/infra/unifiedfleet/app/model/state"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

// GetDutState returns the DutState for the ChromeOS device.
func GetDutState(ctx context.Context, id, hostname string) (*chromeosLab.DutState, error) {
	if id != "" {
		return state.GetDutStateACL(ctx, id)
	}
	dutStates, err := state.QueryDutStateByPropertyNames(ctx, map[string]string{"hostname": hostname}, false)
	if err != nil {
		return nil, err
	}
	if len(dutStates) == 0 {
		return nil, status.Errorf(codes.NotFound, fmt.Sprintf("Dut State not found for %s.", hostname))
	}
	return dutStates[0], nil
}

// ListDutStates lists the DutStates in datastore.
func ListDutStates(ctx context.Context, pageSize int32, pageToken, filter string, keysOnly bool) ([]*chromeosLab.DutState, string, error) {
	return listDutStatesWithExperimentalACLs(ctx, pageSize, pageToken, nil, keysOnly)
}

func listDutStatesWithExperimentalACLs(ctx context.Context, pageSize int32, pageToken string, filterMap map[string][]any, keysOnly bool) (res []*chromeosLab.DutState, nextPageToken string, err error) {
	if pageToken != "" {
		// See registration/machine.go.
		// ListMachinesACL runs a different API to compared to ListMachines
		// This results in ListMachinesACL getting a different type of page
		// token (a multicursor) compared to ListMachines. The function
		// IsMultiCursor is used here to tell which API to use.
		// This is required because this function gets called repeatedly
		// (due to limitations in RPC size) by clients (like shivas).
		//
		// The first time there is no page token so we choose which rpc
		// to use at random. But if there is more data to be returned
		// after the first run, datastore returns a page token. As the
		// token is different based on which API was used, we use it to
		// do the remaining transactions.
		//
		// Note: We can't use token from ListMachinesACL on ListMachines,
		// this doesn't always throw an error but the results are
		// undefined. We do get an error(with high accuracy) if we use
		// token from ListMachines on ListMachinesACL
		if datastore.IsMultiCursorString(pageToken) {
			logging.Infof(ctx, "ListDutStatesACL --- Continue Running in experimental API")
			// If we have a multicursor in our hand. Then we got to do the ACLs
			return state.ListDutStatesACL(ctx, pageSize, pageToken, filterMap, keysOnly)
		} else {
			return state.ListDutStates(ctx, pageSize, pageToken, filterMap, keysOnly)
		}
	}

	cutoff := config.Get(ctx).GetExperimentalAPI().GetListDutStatesACL()

	// If cutoff is set attempt to divert the traffic to new API
	if cutoff != 0 {
		// Roll the dice to determine which one to use
		roll := rand.Uint32() % 100
		cutoff := cutoff % 100
		if roll <= cutoff {
			logging.Infof(ctx, "ListDutStatesACL --- Running in experimental API")
			return state.ListDutStatesACL(ctx, pageSize, pageToken, filterMap, keysOnly)
		}
	}

	// default to old API
	return state.ListDutStates(ctx, pageSize, pageToken, filterMap, keysOnly)
}

// UpdateDutState updates the dut state for a ChromeOS DUT
func UpdateDutState(ctx context.Context, ds *chromeosLab.DutState) (*chromeosLab.DutState, error) {
	f := func(ctx context.Context) error {
		if ds == nil {
			return status.Errorf(codes.InvalidArgument, "dut state must not be null.")
		}
		// It's not ok that no such DUT (machine lse) exists in UFS.
		machineLSE, err := inventory.GetMachineLSE(ctx, ds.GetHostname())
		if err != nil {
			return err
		}

		if err := util.CheckPermission(ctx, util.ConfigurationsUpdate, machineLSE.GetRealm()); err != nil {
			return err
		}

		if err := assignRealmFromMachineLSE(ds, machineLSE); err != nil {
			return err
		}

		hc := &HistoryClient{}
		// It's ok that no old dut state for this DUT exists before.
		oldDS, _ := state.GetDutStateACL(ctx, ds.GetId().GetValue())

		if _, err := state.UpdateDutStates(ctx, []*chromeosLab.DutState{ds}); err != nil {
			return errors.Annotate(err, "Unable to update dut state for %s", ds.GetId().GetValue()).Err()
		}
		hc.LogDutStateChanges(oldDS, ds)

		// Update corresponding device labels for DUTs if applicable
		if machineLSE != nil && machineLSE.GetChromeosMachineLse() != nil {
			if err = updateChromeOSDeviceLabels(ctx, hc, machineLSE, nil, true); err != nil {
				return errors.Annotate(err, "Error updating device labels").Err()
			}
			if err = updateSchedulingUnitDeviceLabels(ctx, hc, machineLSE, true); err != nil {
				return errors.Annotate(err, "Error updating device labels").Err()
			}
		}
		return hc.SaveChangeEvents(ctx)
	}

	if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
		logging.Errorf(ctx, "UpdateDutState (%s, %s) - %s", ds.GetId().GetValue(), ds.GetHostname(), err)
		return nil, err
	}
	return ds, nil
}

// UpdateDutStateWithMasks updates the dut state for a ChromeOS DUT by specified masks.
func UpdateDutStateWithMasks(ctx context.Context, maskSet map[string]bool, ds *chromeosLab.DutState) (*chromeosLab.DutState, error) {
	f := func(ctx context.Context) error {
		if ds == nil {
			return status.Errorf(codes.InvalidArgument, "dut state must not be null.")
		}
		// It's not ok that no such DUT (machine lse) exists in UFS.
		machineLSE, err := inventory.GetMachineLSE(ctx, ds.GetHostname())
		if err != nil {
			return err
		}
		hc := &HistoryClient{}
		// It's ok that no old dut state for this DUT exists before.
		newDs, _ := state.GetDutState(ctx, ds.GetId().GetValue())
		var oldDs *chromeosLab.DutState
		if newDs == nil {
			// If old dut state is empty then we initiate new one as we do not want just copy everything from provided one.
			newDs = &chromeosLab.DutState{
				Id: ds.GetId(),
			}
			oldDs = nil
		} else {
			oldDs = proto.Clone(newDs).(*chromeosLab.DutState)
		}

		if err := assignRealmFromMachineLSE(ds, machineLSE); err != nil {
			return err
		}

		// Apply field by masks.
		if maskSet["dut_state.reason"] {
			newDs.DutStateReason = ds.GetDutStateReason()
		}
		if maskSet["dut_state.servo"] {
			newDs.Servo = ds.GetServo()
		}
		if maskSet["dut_state.servo_usb"] {
			newDs.ServoUsbState = ds.GetServoUsbState()
		}
		if maskSet["dut_state.repair_requests"] {
			newDs.RepairRequests = ds.GetRepairRequests()
		}
		if maskSet["dut_state.chameleon"] {
			newDs.Chameleon = ds.Chameleon
		}
		if maskSet["dut_state.audio_loopback_dongle"] {
			newDs.AudioLoopbackDongle = ds.GetAudioLoopbackDongle()
		}
		if maskSet["dut_state.bluetooth"] {
			newDs.BluetoothState = ds.GetBluetoothState()
		}
		if maskSet["dut_state.wifi"] {
			newDs.WifiState = ds.GetWifiState()
		}
		if maskSet["dut_state.wifi_peripheral"] {
			newDs.WifiPeripheralState = ds.GetWifiPeripheralState()
		}
		if maskSet["dut_state.working_btpeer_count"] {
			newDs.WorkingBluetoothBtpeer = ds.GetWorkingBluetoothBtpeer()
		}
		if maskSet["dut_state.cr50_phase"] {
			newDs.Cr50Phase = ds.GetCr50Phase()
		}
		if maskSet["dut_state.cr50_keyenv"] {
			newDs.Cr50KeyEnv = ds.GetCr50KeyEnv()
		}
		if maskSet["dut_state.storage"] {
			newDs.StorageState = ds.GetStorageState()
		}
		if maskSet["dut_state.battery"] {
			newDs.BatteryState = ds.GetBatteryState()
		}
		if maskSet["dut_state.cellular_modem"] {
			newDs.CellularModemState = ds.GetCellularModemState()
		}
		if maskSet["dut_state.rpm"] {
			newDs.RpmState = ds.GetRpmState()
		}
		if maskSet["dut_state.version_info"] {
			newDs.VersionInfo = ds.GetVersionInfo()
		}
		if maskSet["dut_state.dolos"] {
			newDs.DolosState = ds.GetDolosState()
		}
		if maskSet["dut_state.fw_ec_target"] {
			newDs.FwEcTarget = ds.GetFwEcTarget()
		}
		if maskSet["dut_state.fw_ap_target"] {
			newDs.FwApTarget = ds.GetFwApTarget()
		}
		if maskSet["dut_state.amt_manager_state"] {
			newDs.AmtManagerState = ds.GetAmtManagerState()
		}
		if ds.GetHostname() != "" {
			// Update hostname always as it can change and better to update.
			newDs.Hostname = ds.GetHostname()
		}
		if _, err := state.UpdateDutStates(ctx, []*chromeosLab.DutState{newDs}); err != nil {
			return errors.Annotate(err, "Unable to update dut state for %s", newDs.GetId().GetValue()).Err()
		}
		hc.LogDutStateChanges(oldDs, newDs)

		// Update corresponding device labels for DUTs if applicable
		if machineLSE != nil && machineLSE.GetChromeosMachineLse() != nil {
			if err = updateChromeOSDeviceLabels(ctx, hc, machineLSE, nil, true); err != nil {
				return errors.Annotate(err, "Error updating device labels").Err()
			}
			if err = updateSchedulingUnitDeviceLabels(ctx, hc, machineLSE, true); err != nil {
				return errors.Annotate(err, "Error updating device labels").Err()
			}
		}

		return hc.SaveChangeEvents(ctx)
	}

	if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
		logging.Errorf(ctx, "UpdateDutState (%s, %s) - %s", ds.GetId().GetValue(), ds.GetHostname(), err)
		return nil, err
	}
	return ds, nil
}

func assignRealmFromMachineLSE(ds *chromeosLab.DutState, machinelse *ufspb.MachineLSE) error {
	if ds == nil {
		return status.Error(codes.Internal, "assignRealmFromMachineLSE - DutState is nil")
	}
	if machinelse == nil {
		return status.Error(codes.Internal, "assignRealmFromMachineLSE - MachineLSE is nil")
	}
	ds.Realm = machinelse.GetRealm()
	return nil
}
