// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package dumper

import (
	"context"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	chromeosLab "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/lab"
	"go.chromium.org/infra/unifiedfleet/app/controller"
	"go.chromium.org/infra/unifiedfleet/app/model/inventory"
	"go.chromium.org/infra/unifiedfleet/app/model/registration"
	"go.chromium.org/infra/unifiedfleet/app/model/state"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

type indexTableFn func(context.Context, string, *string) error

// IndexAssets updates the asset table thereby reindexing any new indexes that
// might be added to it. It is meant to be run during low-traffic/maintenance
// times as it attempts to index the entire table.
func IndexAssets(ctx context.Context) error {
	f := func(newCtx context.Context, namespace string, startToken *string) error {
		var err error
		var res []*ufspb.Asset
		res, *startToken, err = registration.ListAssets(newCtx, pageSize, *startToken, nil, false)
		if err != nil {
			return errors.Annotate(err, "indexAssets[%s] -- Failed to list", namespace).Err()
		}
		logging.Infof(ctx, "indexAssets -- Indexing %v assets in %s", len(res), namespace)
		// Update the assets back to datastore
		_, err = registration.BatchUpdateAssets(newCtx, res)
		if err != nil {
			return errors.Annotate(err, "indexAssets[%s] -- Failed to update", namespace).Err()
		}
		return nil
	}
	return indexTable(ctx, "assets", f)
}

// IndexMachines updates the machine table thereby reindexing any new indexes that
// might be added to it. It is meant to be run during low-traffic/maintenance
// times as it attempts to index the entire table.
func IndexMachines(ctx context.Context) error {
	f := func(newCtx context.Context, namespace string, startToken *string) error {
		var err error
		var res []*ufspb.Machine
		res, *startToken, err = registration.ListMachines(newCtx, pageSize, *startToken, nil, false)
		if err != nil {
			return errors.Annotate(err, "indexMachines[%s] -- Failed to list", namespace).Err()
		}
		logging.Infof(ctx, "indexMachines -- Indexing %v machines in %s", len(res), namespace)
		// Update the machines back to datastore
		_, err = registration.BatchUpdateMachines(newCtx, res)
		if err != nil {
			return errors.Annotate(err, "indexMachines[%s] -- Failed to update", namespace).Err()
		}
		return nil
	}
	return indexTable(ctx, "machines", f)
}

// indexRacks updates the rack table thereby reindexing any new indexes that
// might be added to it. It is meant to be run during low-traffic/maintenance
// times as it attempts to index the entire table.
func indexRacks(ctx context.Context) error {
	f := func(newCtx context.Context, namespace string, startToken *string) error {
		var err error
		var res []*ufspb.Rack
		res, *startToken, err = registration.ListRacks(newCtx, pageSize, *startToken, nil, false)
		if err != nil {
			return errors.Annotate(err, "indexRacks[%s] -- Failed to list", namespace).Err()
		}
		logging.Infof(ctx, "indexRacks -- Indexing %v racks in %s", len(res), namespace)
		// Update the rack back to datastore
		_, err = registration.BatchUpdateRacks(newCtx, res)
		if err != nil {
			return errors.Annotate(err, "indexRacks[%s] -- Failed to update", namespace).Err()
		}
		return nil
	}
	return indexTable(ctx, "racks", f)
}

func indexTable(ctx context.Context, tableName string, fn indexTableFn) error {
	logging.Infof(ctx, "indexTable -- Starting to index the %s table", tableName)
	for _, ns := range util.ClientToDatastoreNamespace {
		newCtx, err := util.SetupDatastoreNamespace(ctx, ns)
		if err != nil {
			logging.Errorf(ctx, "indexTable -- internal error, can't setup namespace %s. %v", ns, err)
			continue
		}
		for startToken := ""; ; {
			f := func(newCtx context.Context) error {
				return fn(newCtx, ns, &startToken)
			}
			if err := datastore.RunInTransaction(newCtx, f, nil); err != nil {
				// Log the error. No point in throwing it here as it will be ignored
				logging.Errorf(newCtx, "Cannot index %s in %s: %v", tableName, ns, err)
			}
			if startToken == "" {
				break
			}
		}
	}
	logging.Infof(ctx, "indexTable -- Done indexing the %s table", tableName)
	return nil
}

// indexMachineLSEs reads the entire machineLSE table in all namespaces, updates the realm
// field for the table by reading the corresponding machines. And writes the updated
// table back to datastore
func indexMachineLSEs(ctx context.Context) error {
	f := func(ctx context.Context, ns string, token *string) error {
		var err error
		var lses []*ufspb.MachineLSE
		lses, *token, err = inventory.ListMachineLSEs(ctx, pageSize, *token, nil, false)
		if err != nil {
			return errors.Annotate(err, "indexMachineLSEs[%s] -- Failed to list", ns).Err()
		}
		logging.Infof(ctx, "indexMachineLSEs -- Indexing %v MachineLSEs in %s", len(lses), ns)
		for _, lse := range lses {
			machines := lse.GetMachines()
			if len(machines) == 1 && machines[0] != "" {
				machine, err := registration.GetMachine(ctx, machines[0])
				if err != nil {
					logging.Errorf(ctx, "indexMachineLSEs[%s] -- Failed to get %s", lse.GetName(), machines[0])
					continue
				}
				if machine.GetRealm() == "" {
					logging.Errorf(ctx, "indexMachineLSEs[%s] -- Failed to add realm. Missing realm %s", lse.GetName(), machines[0])
					continue
				}
				lse.Realm = machine.GetRealm()
			} else {
				logging.Errorf(ctx, "indexMachineLSEs[%s] -- Failed to update realms. Need exactly one machine [%v]", lse.GetName(), machines)
			}
		}
		// Update the MachineLSEs back to datastore
		_, err = inventory.BatchUpdateMachineLSEs(ctx, lses)
		if err != nil {
			return errors.Annotate(err, "indexMachineLSEs[%s] -- Failed to update", ns).Err()
		}
		return nil
	}
	return indexTable(ctx, "machineLSEs", f)
}

// dutStates reads the entire DutState table in all namespaces, updates the realm
// field for the table by reading the corresponding machines. And writes the updated
// table back to datastore
// UpdateDutStates
func indexDutStates(ctx context.Context) error {
	f := func(ctx context.Context, ns string, token *string) error {
		var err error
		var dutStates []*chromeosLab.DutState
		dutStates, *token, err = state.ListDutStates(ctx, pageSize, *token, nil, false)
		if err != nil {
			return errors.Annotate(err, "indexDutStates[%s] -- Failed to list", ns).Err()
		}
		logging.Infof(ctx, "indexDutStates -- Indexing %v DutStates in %s", len(dutStates), ns)
		for _, dutState := range dutStates {
			machineLSE, err := inventory.GetMachineLSE(ctx, dutState.GetHostname())
			if err != nil {
				logging.Errorf(ctx, "indexDutStates[%s] -- Failed to update realms, as not able to extract machineLSE for the give DutState", dutState.GetHostname())
				continue
			}
			if machineLSE.GetRealm() == "" {
				logging.Errorf(ctx, "indexDutStates[%s] -- Failed to add realm. Missing realm in machineLSE: %s", dutState.GetHostname(), machineLSE)
				continue
			}
			dutState.Realm = machineLSE.GetRealm()
		}
		_, err = state.UpdateDutStates(ctx, dutStates)
		if err != nil {
			return errors.Annotate(err, "indexDutStates[%s] -- Failed to update", ns).Err()
		}
		return nil
	}
	return indexTable(ctx, "dutStates", f)
}

// backfillDeviceLabelsMachineLSEs reads the entire machineLSE table in all namespaces
// and updates the device labels table accordingly
func backfillDeviceLabelsMachineLSEs(ctx context.Context) error {
	f := func(ctx context.Context, ns string, token *string) error {
		var err error
		var lses []*ufspb.MachineLSE
		lses, *token, err = inventory.ListMachineLSEs(ctx, pageSize, *token, nil, false)
		if err != nil {
			return errors.Annotate(err, "backfillDeviceLabelsMachineLSEs[%s] -- Failed to list", ns).Err()
		}
		logging.Infof(ctx, "backfillDeviceLabelsMachineLSEs -- Backfilling %v MachineLSEs in %s", len(lses), ns)
		deviceLabelsList := make([]*ufspb.DeviceLabels, len(lses))
		for i, lse := range lses {
			deviceLabelsList[i], err = controller.GetMachineLSELabels(ctx, lse)
			if err != nil {
				logging.Errorf(ctx, "backfillDeviceLabelsMachineLSEs[%s] -- Error generating device labels for %s", lse.GetName())
				continue
			}
		}
		// Update the DeviceLabels back to datastore
		_, err = inventory.BatchUpdateDeviceLabels(ctx, deviceLabelsList)
		if err != nil {
			return errors.Annotate(err, "backfillDeviceLabelsMachineLSEs[%s] -- Failed to update", ns).Err()
		}
		return nil
	}
	return indexTable(ctx, "backfillDeviceLabelsMachineLSEs", f)
}

// backfillDeviceLabelsVMs reads the entire VM table in all namespaces
// and updates the device labels table accordingly
func backfillDeviceLabelsVMs(ctx context.Context) error {
	f := func(ctx context.Context, ns string, token *string) error {
		var err error
		var vms []*ufspb.VM
		vms, *token, err = inventory.ListVMs(ctx, pageSize, pageSize, *token, nil, false, nil)
		if err != nil {
			return errors.Annotate(err, "backfillDeviceLabelsVMs[%s] -- Failed to list", ns).Err()
		}
		logging.Infof(ctx, "backfillDeviceLabelsVMs -- Backfilling %v VMs in %s", len(vms), ns)
		deviceLabelsList := make([]*ufspb.DeviceLabels, len(vms))
		for i, vm := range vms {
			deviceLabelsList[i] = controller.GetBrowserVMLabels(vm)
		}
		// Update the DeviceLabels back to datastore
		_, err = inventory.BatchUpdateDeviceLabels(ctx, deviceLabelsList)
		if err != nil {
			return errors.Annotate(err, "backfillDeviceLabelsVMs[%s] -- Failed to update", ns).Err()
		}
		return nil
	}
	return indexTable(ctx, "backfillDeviceLabelsVMs", f)
}

// backfillDeviceLabelsSchedulingUnits reads the entire scheduling units table in all namespaces
// and updates the device labels table accordingly
func backfillDeviceLabelsSchedulingUnits(ctx context.Context) error {
	f := func(ctx context.Context, ns string, token *string) error {
		var err error
		var sus []*ufspb.SchedulingUnit
		sus, *token, err = inventory.ListSchedulingUnits(ctx, pageSize, *token, nil, false)
		if err != nil {
			return errors.Annotate(err, "backfillDeviceLabelsSchedulingUnits[%s] -- Failed to list", ns).Err()
		}
		logging.Infof(ctx, "backfillDeviceLabelsSchedulingUnits -- Backfilling %v SchedulingUnits in %s", len(sus), ns)
		deviceLabelsList := make([]*ufspb.DeviceLabels, len(sus))
		for i, su := range sus {
			deviceLabelsList[i], err = controller.GetSchedulingUnitLabels(ctx, su, nil)
			if err != nil {
				logging.Errorf(ctx, "backfillDeviceLabelsSchedulingUnits[%s] -- Error generating device labels for %s", su.GetName())
				continue
			}
		}
		// Update the DeviceLabels back to datastore
		_, err = inventory.BatchUpdateDeviceLabels(ctx, deviceLabelsList)
		if err != nil {
			return errors.Annotate(err, "backfillDeviceLabelsSchedulingUnits[%s] -- Failed to update", ns).Err()
		}
		return nil
	}
	return indexTable(ctx, "backfillDeviceLabelsSchedulingUnits", f)
}
