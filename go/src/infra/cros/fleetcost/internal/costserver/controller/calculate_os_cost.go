// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	fleetcostpb "infra/cros/fleetcost/api/models"
	fleetcostAPI "infra/cros/fleetcost/api/rpc"
	ufsFetcher "infra/cros/fleetcost/internal/costserver/inventory/ufs"
	"infra/cros/fleetcost/internal/utils"
	ufspb "infra/unifiedfleet/api/v1/models"
	lab "infra/unifiedfleet/api/v1/models/chromeos/lab"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
)

// CalculateCostForOsResource calculates the cost for an OS resource.
//
// So far, only ChromeOS devices are supported.
//
// ic            -- The fleet client. Allowed to be nil precisely when UFS is not used.
// deviceDataRes -- The device data for the DUT in question. Allowed to be nil precisely when UFS is not used.
// req           -- The underlying GetCostResultRequest associated with the GetCostResult RPC as a whole.
//
// TODO(b/366067524): Refactor to not use a request object.
// TODO(b/366033984): Refactor to not pass the FleetClient in as deeply and instead calculate just the information that we need earlier.
func CalculateCostForOsResource(ctx context.Context, ic ufsAPI.FleetClient, deviceDataRes *ufsAPI.GetDeviceDataResponse, req *fleetcostAPI.GetCostResultRequest) (*fleetcostpb.CostResult, error) {
	hostname := req.GetHostname()
	forgiveMissingEntries := req.GetForgiveMissingEntries()
	logging.Infof(ctx, "getting device data for hostname %q with forgive=%v", hostname, forgiveMissingEntries)
	switch getResourceType(deviceDataRes.GetResourceType(), req.GetNoUfs()) {
	case ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_CHROMEOS_DEVICE:
		logging.Infof(ctx, "detected that %q is a ChromeOS device", hostname)
		resp, err := calculateCostForSingleChromeosDut(ctx, ic, deviceDataRes.GetChromeOsDeviceData(), forgiveMissingEntries, req.GetNoUfs())
		return resp, errors.Annotate(err, "calculate ChromeOS device cost").Err()
	case ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_ATTACHED_DEVICE:
		return nil, errors.Reason("%s is an attached device, support is not implemented yet.", hostname).Err()
	case ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_SCHEDULING_UNIT:
		return nil, errors.Reason("%s is an scheduling unit, support is not implemented yet.", hostname).Err()
	default:
		return nil, errors.Reason("Cannot find a valid resource type for %s: %s", hostname, deviceDataRes.GetResourceType()).Err()
	}
}

// getResourceType gets the resource type from the hints in the request if hints are present and should be used.
//
// Otherwise, we follow the production path and get the information from UFS.
func getResourceType(realResourceType ufsAPI.GetDeviceDataResponse_ResourceType, noUFS bool) ufsAPI.GetDeviceDataResponse_ResourceType {
	if noUFS {
		return ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_CHROMEOS_DEVICE
	}
	return realResourceType
}

// calculateCostForSingleChromeosDut calculates the cost of a ChromeOS DUT.
func calculateCostForSingleChromeosDut(ctx context.Context, ic ufsAPI.FleetClient, data *ufspb.ChromeOSDeviceData, forgiveMissingEntries bool, noUFS bool) (*fleetcostpb.CostResult, error) {
	logging.Infof(ctx, "calculating cost for %q with forgive=%v", data.GetMachine().GetName(), forgiveMissingEntries)
	dut := data.GetLabConfig().GetChromeosMachineLse().GetDeviceLse().GetDut()
	peripherals := dut.GetPeripherals()
	servo := peripherals.GetServo()

	// TODO(gregorynisbet): In cases where we have multiple pools, we need some logic to
	//                      extract the most specific pool. We can probably achieve this by filtering out
	//                      uninformative pools like DUT_POOL_QUOTA and then using a very simple fallback mechanism
	//                      like the length of the name or lexicographic order.
	pools := dut.GetPools()
	pool := ""
	if len(pools) == 1 {
		pool = pools[0]
	}

	// TODO: add a map that convert UFS location to cost indicator location. Hardcode to all for now.
	location := fleetcostpb.Location_LOCATION_ALL
	if !noUFS && dut == nil {
		return nil, utils.MaybeErrorf(ctx, errors.Reason("%s is not a valid ChromeOS DUT", data.GetLabConfig().GetHostname()).Err())
	}

	m := data.GetMachine().GetChromeosMachine()

	sharedCost, err := getSharedCost(ctx, location, forgiveMissingEntries, pool)
	if err != nil {
		return nil, errors.Annotate(err, "calculate cost for single ChromeOS DUT: shared").Err()
	}

	dedicatedCost, err := getDUTDedicatedHardwareCost(ctx, m, servo, location, forgiveMissingEntries, pool)
	if err != nil {
		return nil, errors.Annotate(err, "calculate cost for single ChromeOS DUT: dedicated").Err()
	}
	cloudCost, err := getCloudCost(ctx, location, forgiveMissingEntries)
	if err != nil {
		return nil, errors.Annotate(err, "calculate cost for single ChromeOS DUT: cloud").Err()
	}

	// Cost for labstation, which is special
	if servo.GetServoHostname() != "" {
		labstationCost, err := getLabstationHardwareCost(ctx, ic, servo.GetServoHostname(), location, forgiveMissingEntries)
		if err != nil {
			return nil, utils.MaybeErrorf(ctx, errors.Annotate(err, "calculate cost for single chromeos dut").Err())
		}
		sharedCost = append(sharedCost, utils.MakeCostReportItem("labstation", labstationCost, labstationCost, "shared"))
	}

	out := utils.MakeCostReportExpr("total", "add")
	utils.AppendCostReportItem(out, dedicatedCost...)
	utils.AppendCostReportItem(out, sharedCost...)
	utils.AppendCostReportItem(out, cloudCost)

	dedicatedCostTotal := utils.SumCostReportItem(dedicatedCost...)
	sharedCostTotal := utils.SumCostReportItem(sharedCost...)
	cloudCostTotal := utils.SumCostReportItem(cloudCost)
	total := dedicatedCostTotal + sharedCostTotal + cloudCostTotal

	return &fleetcostpb.CostResult{
		DedicatedCost:    dedicatedCostTotal,
		SharedCost:       sharedCostTotal,
		CloudServiceCost: cloudCostTotal,
		Report: &fleetcostpb.CostReport{
			Total: total,
			Expr:  out,
		},
	}, nil
}

// getLabstationHardwareCost gets the hardware cost of a labstation
func getLabstationHardwareCost(ctx context.Context, ic ufsAPI.FleetClient, hostname string, location fleetcostpb.Location, forgiveMissingEntries bool) (float64, error) {
	data, err := ufsFetcher.GetChromeosDeviceData(ctx, ic, hostname)
	if err != nil {
		return 0, utils.MaybeErrorf(ctx, errors.Annotate(err, "get labstation cost").Err())
	}
	m := data.GetMachine().GetChromeosMachine()

	sharedCost := 0.0
	v, err := getAmortizedCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "labstation cost",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_LABSTATION,
		Primary:       m.GetBuildTarget(),
		Secondary:     m.GetModel(),
		Tertiary:      m.GetSku(),
		Location:      location,
	}, true, forgiveMissingEntries)
	if err != nil {
		return 0, utils.MaybeErrorf(ctx, errors.Annotate(err, "get labstation cost").Err())
	}
	sharedCost += v

	v, err = getAmortizedCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "usb hub cost",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_USBHUB,
		Primary:       "",
		Secondary:     "",
		Tertiary:      "",
		Location:      location,
	}, true, forgiveMissingEntries)
	if err != nil {
		return 0.0, err
	}
	sharedCost += v

	labMap, err := ufsFetcher.GetLabstationDutMapping(ctx, ic, []string{hostname})
	if err != nil {
		return 0, utils.MaybeErrorf(ctx, errors.Annotate(err, "get labstation cost").Err())
	}
	if l, ok := labMap[hostname]; ok {
		if len(l) > 0 {
			return sharedCost / float64(len(l)), nil
		}
	}
	return 0, utils.MaybeErrorf(ctx, errors.Reason("Unable to get number of DUTs under %s", hostname).Err())
}

// getSharedCost gets the shared costs except for labstation costs.
//
// "-" indicates that the name is unchanged.
// If two columns from a spreadsheet name map to the same thing, then they
// should be summed when read into the cost service.
//
// Model V2 spreadsheet                 Cost indicator name
// - server-acquisition                    -
// - server-maintenance                    -
// - network-infra-acquisition             -
// - network-infra-maintenance             -
// - quota-faft-opex                       -
// - connectivity-and-misc-testbeds        -
// - control-network-racks                 -
// - connectivity-rack-setup               rack-setup
// - connectivity-phase-deployments        phase-deployments
// - connectivity-annual-maintenance       annual-maintenance
// - quota-faft-rack-setup                 rack-setup
// - quota-faft-phase-deployments          phase-deployments
// - quota-faft-annual-maintenance         annual-maintenance
func getSharedCost(ctx context.Context, location fleetcostpb.Location, forgiveMissingEntries bool, pool string) ([]*fleetcostpb.CostReportItem, error) {
	var sharedCost []*fleetcostpb.CostReportItem
	v, err := getAmortizedCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "server acquisition",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_SERVER,
		Primary:       "server-acquisition",
		Secondary:     pool,
		Tertiary:      "",
		Location:      location,
	}, true, forgiveMissingEntries)
	if err != nil {
		return nil, errors.Annotate(err, "get shared cost").Err()
	}
	sharedCost = append(sharedCost, utils.MakeCostReportItem("server acquisition", v, v, "shared"))

	v, err = getAmortizedCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "server maintenance",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_SERVER,
		Primary:       "server-maintenance",
		Secondary:     pool,
		Tertiary:      "",
		Location:      location,
	}, true, forgiveMissingEntries)
	if err != nil {
		return nil, errors.Annotate(err, "get shared cost").Err()
	}
	sharedCost = append(sharedCost, utils.MakeCostReportItem("server maintenance", v, v, "shared"))

	v, err = getAmortizedCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "network infra acquisition",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_SERVER,
		Primary:       "network-infra-acquisition",
		Secondary:     pool,
		Tertiary:      "",
		Location:      location,
	}, true, forgiveMissingEntries)
	if err != nil {
		return nil, errors.Annotate(err, "get shared cost").Err()
	}
	sharedCost = append(sharedCost, utils.MakeCostReportItem("network infra acquisition", v, v, "shared"))

	v, err = getAmortizedCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "network infra maintenance",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_SERVER,
		Primary:       "network-infra-maintenance",
		Secondary:     pool,
		Tertiary:      "",
		Location:      location,
	}, true, forgiveMissingEntries)
	if err != nil {
		return nil, errors.Annotate(err, "get shared cost").Err()
	}
	sharedCost = append(sharedCost, utils.MakeCostReportItem("network infra maintenance", v, v, "shared"))

	v, err = getAmortizedCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "quota faft opex",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_SPACE,
		Primary:       "quota-faft-opex",
		Secondary:     pool,
		Tertiary:      "",
		Location:      location,
	}, true, forgiveMissingEntries)
	if err != nil {
		return nil, errors.Annotate(err, "get shared cost").Err()
	}
	sharedCost = append(sharedCost, utils.MakeCostReportItem("quota faft opex", v, v, "shared"))

	v, err = getAmortizedCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "connectivity and misc testbeds",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_SERVER,
		Primary:       "connectivity-and-misc-testbeds",
		Secondary:     pool,
		Tertiary:      "",
		Location:      location,
	}, true, forgiveMissingEntries)
	if err != nil {
		return nil, errors.Annotate(err, "get shared cost").Err()
	}
	sharedCost = append(sharedCost, utils.MakeCostReportItem("connectivity and misc testbeds", v, v, "shared"))

	v, err = getAmortizedCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "control network racks",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_SERVER,
		Primary:       "control-network-racks",
		Secondary:     pool,
		Tertiary:      "",
		Location:      location,
	}, true, forgiveMissingEntries)
	if err != nil {
		return nil, errors.Annotate(err, "get shared cost").Err()
	}
	sharedCost = append(sharedCost, utils.MakeCostReportItem("control network racks", v, v, "shared"))

	v, err = getAmortizedCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "rack setup",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_SERVER,
		Primary:       "rack-setup",
		Secondary:     pool,
		Tertiary:      "",
		Location:      location,
	}, true, forgiveMissingEntries)
	if err != nil {
		return nil, errors.Annotate(err, "get shared cost").Err()
	}
	sharedCost = append(sharedCost, utils.MakeCostReportItem("rack setup", v, v, "shared"))

	v, err = getAmortizedCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "phase deployments",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_SERVER,
		Primary:       "phase-deployments",
		Secondary:     pool,
		Tertiary:      "",
		Location:      location,
	}, true, forgiveMissingEntries)
	if err != nil {
		return nil, errors.Annotate(err, "get shared cost").Err()
	}
	sharedCost = append(sharedCost, utils.MakeCostReportItem("phase deployments", v, v, "shared"))

	v, err = getAmortizedCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "annual maintenance",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_SERVER,
		Primary:       "annual-maintenance",
		Secondary:     pool,
		Tertiary:      "",
		Location:      location,
	}, true, forgiveMissingEntries)
	if err != nil {
		return nil, errors.Annotate(err, "get shared cost").Err()
	}
	sharedCost = append(sharedCost, utils.MakeCostReportItem("annual maintenance", v, v, "shared"))

	return sharedCost, nil
}

// getDUTDedicatedHardwareCost gets the acquisition cost of a DUT and servo, which are the only two
// resources that are DUT-specific
func getDUTDedicatedHardwareCost(ctx context.Context, m *ufspb.ChromeOSMachine, servo *lab.Servo, location fleetcostpb.Location, forgiveMissingEntries bool, pool string) ([]*fleetcostpb.CostReportItem, error) {
	var out []*fleetcostpb.CostReportItem
	ent, err := getCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "DUT cost",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_DUT,
		Primary:       m.GetBuildTarget(),
		Secondary:     m.GetModel(),
		Tertiary:      m.GetSku(),
		Location:      location,
	}, true, forgiveMissingEntries)
	if err != nil {
		return nil, errors.Annotate(err, "dut hardware cost for %q %q %v", m.GetBuildTarget(), location.String(), forgiveMissingEntries).Err()
	}
	v, err := normalizeToHourlyCost(ent, forgiveMissingEntries)
	if err != nil {
		return nil, errors.Annotate(err, "get shared cost").Err()
	}
	out = append(out, utils.MakeCostReportItem("dut cost", v, v, "dedicated"))
	if servo != nil {
		servoCost, err := getAmortizedCostIndicatorValue(ctx, &indicatorAttribute{
			ErrorHint:     "servo cost",
			IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_SERVO,
			Primary:       servo.GetServoType(),
			Secondary:     "",
			Tertiary:      "",
			Location:      location,
		}, true, forgiveMissingEntries)

		if err != nil {
			return nil, errors.Annotate(err, "servo cost for %q %q %v", servo.GetServoType(), location.String(), forgiveMissingEntries).Err()
		}

		out = append(out, utils.MakeCostReportItem("servo cost", servoCost, servoCost, "dedicated"))
	}

	testbed, err := getAmortizedCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "testbed cost",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_TESTBED,
		Primary:       pool,
		Secondary:     "",
		Tertiary:      "",
		Location:      location,
	}, true, forgiveMissingEntries)
	if err != nil {
		return nil, errors.Annotate(err, "testbed cost for %q %v", location.String(), forgiveMissingEntries).Err()
	}
	out = append(out, utils.MakeCostReportItem("testbed cost", testbed, testbed, "dedicated"))

	return out, nil
}

func getCloudCost(ctx context.Context, location fleetcostpb.Location, forgiveMissingEntries bool) (*fleetcostpb.CostReportItem, error) {
	ent, err := getCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "annual cloud cost",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_CLOUD,
		Primary:       "",
		Secondary:     "",
		Tertiary:      "",
		Location:      location,
	}, true, forgiveMissingEntries)
	if err != nil {
		return nil, err
	}
	v, err := normalizeToHourlyCost(ent, forgiveMissingEntries)
	if err != nil {
		return nil, err
	}
	return utils.MakeCostReportItem("cloud", v, v, "cloud"), nil
}
