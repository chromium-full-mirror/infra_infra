// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"

	"github.com/golang/protobuf/proto"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"
	"go.chromium.org/luci/server/router"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/controller"
	"go.chromium.org/infra/unifiedfleet/app/model/registration"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

// HaRTPushHandler handles the pubsub push responses from HaRT pubsub
//
// Decodes the response sent by PubSub and updates datastore. It doesn't
// return anything as required by https://cloud.google.com/pubsub/docs/push,
// this is because by default the return is 200 OK for http POST requests.
// It does return a http error if the datastore update fails.
func HaRTPushHandler(routerContext *router.Context) {
	// Add namespace as the response from HaRT doesn't have namespace.
	ctx, err := util.SetupDatastoreNamespace(routerContext.Request.Context(), util.OSNamespace)
	if err != nil {
		logging.Errorf(ctx, "HaRTPushHandler - Failed to add namespace to context")
		return
	}
	res, err := util.NewPSRequest(routerContext.Request)
	if err != nil {
		logging.Errorf(ctx, "HaRTPushHandler - Failed to read push req %v", err)
		return
	}
	data, err := res.DecodeMessage()
	if err != nil {
		logging.Errorf(ctx, "HaRTPushHandler - Failed to read data %v", err)
		return
	}
	// Decode the proto contained in the payload
	var response ufspb.AssetInfoResponse
	perr := proto.Unmarshal(data, &response)
	if perr != nil {
		// Avoid returning error, as the data contains some assets not
		// known to HaRT and those will always fail.
		logging.Errorf(ctx, "HaRTPushHandler - Failed to decode proto %v", perr)
		return
	}
	if response.GetRequestStatus() == ufspb.RequestStatus_OK {
		for _, info := range response.GetAssets() {
			if err := updateAssetInfoHelper(ctx, info); err != nil {
				logging.Errorf(ctx, "HaRTPushHandler - unable to update %s: %v",
					info.GetAssetTag(), err)
			}
		}
	}
	logging.Debugf(ctx, "Status: %v", response.GetRequestStatus())
	missing := response.GetMissingAssetTags()
	logging.Debugf(ctx, "Missing[%v]: %v", len(missing), missing)
	failed := response.GetFailedAssetTags()
	logging.Debugf(ctx, "Failed[%v]: %v", len(failed), failed)
	logging.Debugf(ctx, "Success reported for %v assets", len(response.GetAssets()))
}

// updateAssetInfoHelper updates both asset and machine with the provided asset info.
func updateAssetInfoHelper(ctx context.Context, info *ufspb.AssetInfo) error {
	f := func(ctx context.Context) error {
		// Update the asset first
		asset, err := updateAssetHelper(ctx, info)
		if err != nil {
			return err
		}
		if asset != nil {
			if err := updateMachineHelper(ctx, asset); err != nil {
				return err
			}
		}
		return nil
	}
	if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
		return err
	}
	return nil
}

// updateAssetHelper is a helper function to update a list of asset infos
func updateAssetHelper(ctx context.Context, iv2assetinfo *ufspb.AssetInfo) (*ufspb.Asset, error) {
	ufsAsset, err := registration.GetAsset(ctx, iv2assetinfo.GetAssetTag())
	if err != nil {
		return nil, errors.Annotate(err, "updateAssetHelper - Cannot update asset [%v], not found in DS",
			iv2assetinfo.GetAssetTag()).Err()
	}
	hc := &controller.HistoryClient{}
	// Make a copy for logging purposes
	oldAsset := proto.Clone(ufsAsset).(*ufspb.Asset)
	if info := updateAssetInfoFromHart(ufsAsset.GetInfo(), iv2assetinfo); info != nil {
		logging.Debugf(ctx, "updateAssetHelper - Updating %v", ufsAsset.GetName())
		ufsAsset.Info = info
		// Copy the model info.
		ufsAsset.Model = ufsAsset.Info.Model
		hc.LogAssetChanges(oldAsset, ufsAsset)
		if err := hc.SaveChangeEvents(ctx); err != nil {
			return nil, err
		}
		res, err := registration.BatchUpdateAssets(ctx, []*ufspb.Asset{ufsAsset})
		if err != nil {
			return nil, err
		}
		return res[0], nil
	} else {
		logging.Debugf(ctx, "updateAssetHelper - No update for %v. Got: %v", ufsAsset.GetName(), iv2assetinfo)
	}
	// No update required
	return nil, nil
}

// updateMachineHelper is a helper function to update machines based on asset infos.
func updateMachineHelper(ctx context.Context, asset *ufspb.Asset) error {
	if t := asset.GetType(); t == ufspb.AssetType_DUT || t == ufspb.AssetType_LABSTATION {
		logging.Debugf(ctx, "updateMachineHelper - Updating %v", asset.GetName())
		// For DUT and labstation assets update the machine. Doesn't make a
		// realm check since this is an automated process.
		machine, err := registration.GetMachine(ctx, asset.GetName())
		if err != nil {
			return errors.Annotate(err, "updateMachineHelper - Cannot update machine %s", asset.GetName()).Err()
		}
		hc := controller.GetMachineHistoryClient(machine)
		oldMachineCopy := proto.Clone(machine).(*ufspb.Machine)
		// Copy data from asset excluding mac, sku, phase and hwid. SSW will update these.
		machine.GetChromeosMachine().ReferenceBoard = asset.GetInfo().GetReferenceBoard()
		machine.GetChromeosMachine().BuildTarget = asset.GetInfo().GetBuildTarget()
		machine.GetChromeosMachine().Model = asset.GetInfo().GetModel()
		machine.GetChromeosMachine().GoogleCodeName = asset.GetInfo().GetGoogleCodeName()
		machine.GetChromeosMachine().CostCenter = asset.GetInfo().GetCostCenter()
		machine.GetChromeosMachine().Gpn = asset.GetInfo().GetGpn()
		machine.GetChromeosMachine().HwXComplianceVersion = asset.GetInfo().GetHwXComplianceVersion()
		machine.GetChromeosMachine().IsCbx = asset.GetInfo().GetIsCbx()
		machine.GetChromeosMachine().CbxFeatureType = asset.GetInfo().GetCbxFeatureType()
		machine.GetChromeosMachine().IsMixedX = asset.GetInfo().GetIsMixedX()
		machine.GetChromeosMachine().HasWifiBt = asset.GetInfo().GetHasWifiBt()
		machine.GetChromeosMachine().WifiBluetooth = asset.GetInfo().GetWifiBluetooth()
		hc.LogMachineChanges(oldMachineCopy, machine)
		if _, err := registration.BatchUpdateMachines(ctx, []*ufspb.Machine{machine}); err != nil {
			return errors.Annotate(err, "updateMachineHelper - Failed to update machine %s", machine.GetName()).Err()
		}
		if err := hc.SaveChangeEvents(ctx); err != nil {
			return errors.Annotate(err, "updateMachineHelper - Failed to save change events for %s", machine.GetName()).Err()
		}
	}
	// If it's not a DUT or Labstation there is no machine to update
	return nil
}

// updateAssetInfoFromHart copies cost_center, google_code_name, model,
// build_target, reference_board, gpn and phase from hartAssetInfo if
// any of these were updated.
func updateAssetInfoFromHart(ufsAssetInfo, hartAssetInfo *ufspb.AssetInfo) *ufspb.AssetInfo {
	var updated bool
	if ufsAssetInfo == nil {
		ufsAssetInfo = &ufspb.AssetInfo{
			AssetTag: hartAssetInfo.GetAssetTag(),
		}
	}
	if ufsAssetInfo.GetCostCenter() != hartAssetInfo.GetCostCenter() {
		updated = true
		// Update CostCenter if it's changed
		ufsAssetInfo.CostCenter = hartAssetInfo.GetCostCenter()
	}
	if ufsAssetInfo.GetGoogleCodeName() != hartAssetInfo.GetGoogleCodeName() {
		updated = true
		// Update GoogleCodeName if it's changed
		ufsAssetInfo.GoogleCodeName = hartAssetInfo.GetGoogleCodeName()
	}
	if ufsAssetInfo.GetModel() == "" && hartAssetInfo.GetModel() != "" {
		updated = true
		// Update Model if we don't have it
		ufsAssetInfo.Model = hartAssetInfo.GetModel()
	}
	if ufsAssetInfo.GetBuildTarget() == "" && hartAssetInfo.GetBuildTarget() != "" {
		updated = true
		// Update BuildTarget if we don't have it
		ufsAssetInfo.BuildTarget = hartAssetInfo.GetBuildTarget()
	}
	if ufsAssetInfo.GetReferenceBoard() != hartAssetInfo.GetReferenceBoard() {
		updated = true
		// Update ReferenceBoard if it's changed
		ufsAssetInfo.ReferenceBoard = hartAssetInfo.GetReferenceBoard()
	}
	if ufsAssetInfo.GetEthernetMacAddress() != hartAssetInfo.GetEthernetMacAddress() {
		updated = true
		// Update mac if it's changed
		ufsAssetInfo.EthernetMacAddress = hartAssetInfo.GetEthernetMacAddress()
	}
	if ufsAssetInfo.GetGpn() != hartAssetInfo.GetGpn() {
		updated = true
		// Update GPN if it's changed
		ufsAssetInfo.Gpn = hartAssetInfo.GetGpn()
	}
	if ufsAssetInfo.GetHwid() == "" && hartAssetInfo.GetHwid() != "" {
		updated = true
		// Update hwid if we don't have it
		ufsAssetInfo.Hwid = hartAssetInfo.GetHwid()
	}
	if ufsAssetInfo.GetPhase() != hartAssetInfo.GetPhase() {
		updated = true
		// Update phase if it's changed
		ufsAssetInfo.Phase = hartAssetInfo.GetPhase()
	}
	if ufsAssetInfo.GetSku() == "" && hartAssetInfo.GetSku() != "" {
		updated = true
		// Update sku if we don't have it
		ufsAssetInfo.Sku = hartAssetInfo.GetSku()
	}
	if ufsAssetInfo.GetReferenceDesign() != hartAssetInfo.GetReferenceDesign() {
		updated = true
		// Update reference design if it's changed
		ufsAssetInfo.ReferenceDesign = hartAssetInfo.GetReferenceDesign()
	}
	if ufsAssetInfo.GetProductStatus() != hartAssetInfo.GetProductStatus() {
		updated = true
		// Update product status if it's changed
		ufsAssetInfo.ProductStatus = hartAssetInfo.GetProductStatus()
	}
	if ufsAssetInfo.GetFingerprintSensor() != hartAssetInfo.GetFingerprintSensor() {
		updated = true
		// Update fingerprint sensor if it's changed
		ufsAssetInfo.FingerprintSensor = hartAssetInfo.GetFingerprintSensor()
	}
	if ufsAssetInfo.GetHwXComplianceVersion() != hartAssetInfo.GetHwXComplianceVersion() {
		updated = true
		// Update hwX compliance version if it's changed
		ufsAssetInfo.HwXComplianceVersion = hartAssetInfo.GetHwXComplianceVersion()
	}
	if ufsAssetInfo.GetTouchScreen() != hartAssetInfo.GetTouchScreen() {
		updated = true
		// Update touchscreen if it's changed
		ufsAssetInfo.TouchScreen = hartAssetInfo.GetTouchScreen()
	}
	if ufsAssetInfo.GetIsCbx() != hartAssetInfo.GetIsCbx() {
		updated = true
		// Update isCbx if it's changed
		ufsAssetInfo.IsCbx = hartAssetInfo.GetIsCbx()
	}
	if ufsAssetInfo.GetCbxFeatureType() != hartAssetInfo.GetCbxFeatureType() {
		updated = true
		// Update cbxFeatureType if it's changed
		ufsAssetInfo.CbxFeatureType = hartAssetInfo.GetCbxFeatureType()
	}
	if ufsAssetInfo.GetIsMixedX() != hartAssetInfo.GetIsMixedX() {
		updated = true
		// Update isMixedX status if it's changed
		ufsAssetInfo.IsMixedX = hartAssetInfo.GetIsMixedX()
	}
	if ufsAssetInfo.GetHasWifiBt() != hartAssetInfo.GetHasWifiBt() {
		updated = true
		ufsAssetInfo.HasWifiBt = hartAssetInfo.GetHasWifiBt()
	}
	if ufsAssetInfo.GetWifiBluetooth() != hartAssetInfo.GetWifiBluetooth() {
		updated = true
		ufsAssetInfo.WifiBluetooth = hartAssetInfo.GetWifiBluetooth()
	}
	// Avoid write to DB if nothing was updated
	if updated {
		return ufsAssetInfo
	}
	return nil
}
