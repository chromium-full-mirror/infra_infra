// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"fmt"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	fleetcostpb "infra/cros/fleetcost/api/models"
	"infra/cros/fleetcost/internal/costserver/entities"
	ufsFetcher "infra/cros/fleetcost/internal/costserver/inventory/ufs"
	"infra/cros/fleetcost/internal/utils"
	ufspb "infra/unifiedfleet/api/v1/models"
	lab "infra/unifiedfleet/api/v1/models/chromeos/lab"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
)

// indicatorAttribute is the information that's necessary to look up a datastore record.
type indicatorAttribute struct {
	// ErrorHint is a description of what you were looking for.
	// It gets inserted into the error message.
	ErrorHint     string
	IndicatorType fleetcostpb.IndicatorType
	Board         string
	Model         string
	Sku           string
	Location      fleetcostpb.Location
}

// newIndicatorAttribute creates a new indicator attribute.
//
// TODO(gregorynisbet): Rethink the API for this function, maybe move it to utils.
func newIndicatorAttribute(errorHint string, typ fleetcostpb.IndicatorType, board string, model string, sku string, location fleetcostpb.Location) *indicatorAttribute {
	return &indicatorAttribute{
		ErrorHint:     errorHint,
		IndicatorType: typ,
		Board:         board,
		Model:         model,
		Sku:           sku,
		Location:      location,
	}
}

// FriendlyString produces a human-readable string for error messages.
//
// This string is NOT RELATED to how IndicatorAttributes or CostIndicatorEntities are actually stored
// in the database.
func (attribute *indicatorAttribute) FriendlyString() string {
	if attribute == nil {
		return "<nil>"
	}
	message := fmt.Sprintf("type=%s board=%s model=%s sku=%s loc=%s", attribute.IndicatorType.String(), attribute.Board, attribute.Model, attribute.Sku, attribute.Location.String())
	return message
}

// asEntity converts an IndicatorAttribute to a datastore Entity.
func (attribute *indicatorAttribute) asEntity() *entities.CostIndicatorEntity {
	if attribute == nil {
		return nil
	}
	return &entities.CostIndicatorEntity{
		CostIndicator: &fleetcostpb.CostIndicator{
			Type:     attribute.IndicatorType,
			Board:    attribute.Board,
			Model:    attribute.Model,
			Sku:      attribute.Sku,
			Location: attribute.Location,
		},
	}
}

// CalculateCostForOsResource calculates the cost for an OS resource.
//
// So far, only ChromeOS devices are supported.
func CalculateCostForOsResource(ctx context.Context, ic ufsAPI.FleetClient, hostname string, forgiveMissingEntries bool) (*fleetcostpb.CostResult, *fleetcostpb.CostReport, error) {
	logging.Infof(ctx, "getting device data for hostname %q with forgive=%b", hostname, forgiveMissingEntries)
	res, err := ic.GetDeviceData(ctx, &ufsAPI.GetDeviceDataRequest{Hostname: hostname})
	if err != nil {
		err := errors.Annotate(err, "calculate cost for os resource %q", hostname).Err()
		logging.Errorf(ctx, "%s\n", err)
		return nil, nil, err
	}
	switch res.GetResourceType() {
	case ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_CHROMEOS_DEVICE:
		logging.Infof(ctx, "detected that %q is a ChromeOS device", hostname)
		resp, rep, err := calculateCostForSingleChromeosDut(ctx, ic, res.GetChromeOsDeviceData(), forgiveMissingEntries)
		return resp, rep, errors.Annotate(err, "calculate ChromeOS device cost").Err()
	case ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_ATTACHED_DEVICE:
		return nil, nil, errors.Reason("%s is an attached device, support is not implemented yet.", hostname).Err()
	case ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_SCHEDULING_UNIT:
		return nil, nil, errors.Reason("%s is an scheduling unit, support is not implemented yet.", hostname).Err()
	default:
		return nil, nil, errors.Reason("Cannot find a valid resource type for %s: %s", hostname, res.GetResourceType()).Err()
	}
}

// calculateCostForSingleChromeosDut calculates the cost of a ChromeOS DUT.
func calculateCostForSingleChromeosDut(ctx context.Context, ic ufsAPI.FleetClient, data *ufspb.ChromeOSDeviceData, forgiveMissingEntries bool) (*fleetcostpb.CostResult, *fleetcostpb.CostReport, error) {
	logging.Infof(ctx, "calculating cost for %q with forgive=%v", data.GetMachine().GetName(), forgiveMissingEntries)
	dut := data.GetLabConfig().GetChromeosMachineLse().GetDeviceLse().GetDut()
	peripherals := dut.GetPeripherals()
	servo := peripherals.GetServo()

	// TODO: add a map that convert UFS location to cost indicator location. Hardcode to all for now.
	location := fleetcostpb.Location_LOCATION_ALL
	if dut == nil {
		return nil, nil, utils.MaybeErrorf(ctx, errors.Reason("%s is not a valid ChromeOS DUT", data.GetLabConfig().GetHostname()).Err())
	}

	m := data.GetMachine().GetChromeosMachine()

	sharedCost, err := getSharedCost(ctx, location, forgiveMissingEntries)
	if err != nil {
		return nil, nil, errors.Annotate(err, "calculate cost for single ChromeOS DUT: shared").Err()
	}

	dedicatedCost, err := getDUTDedicatedHardwareCost(ctx, m, servo, location, forgiveMissingEntries)
	if err != nil {
		return nil, nil, errors.Annotate(err, "calculate cost for single ChromeOS DUT: dedicated").Err()
	}
	cloudCost, err := getCloudCost(ctx, location, forgiveMissingEntries)
	if err != nil {
		return nil, nil, errors.Annotate(err, "calculate cost for single ChromeOS DUT: cloud").Err()
	}

	// Cost for labstation, which is special
	if servo.GetServoHostname() != "" {
		labstationCost, err := getLabstationHardwareCost(ctx, ic, servo.GetServoHostname(), location, forgiveMissingEntries)
		if err != nil {
			return nil, nil, utils.MaybeErrorf(ctx, errors.Annotate(err, "calculate cost for single chromeos dut").Err())
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
		}, &fleetcostpb.CostReport{
			Total: total,
			Expr:  out,
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
		Board:         m.GetBuildTarget(),
		Model:         m.GetModel(),
		Sku:           m.GetSku(),
		Location:      location,
	}, true, forgiveMissingEntries)
	if err != nil {
		return 0, utils.MaybeErrorf(ctx, errors.Annotate(err, "get labstation cost").Err())
	}
	sharedCost += v

	v, err = getAmortizedCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "usb hub cost",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_USBHUB,
		Board:         "",
		Model:         "",
		Sku:           "",
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
func getSharedCost(ctx context.Context, location fleetcostpb.Location, forgiveMissingEntries bool) ([]*fleetcostpb.CostReportItem, error) {
	var sharedCost []*fleetcostpb.CostReportItem
	v, err := getAmortizedCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "server acquisition",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_SERVER,
		Board:         "server-acquisition",
		Model:         "",
		Sku:           "",
		Location:      location,
	}, true, forgiveMissingEntries)
	if err != nil {
		return nil, errors.Annotate(err, "get shared cost").Err()
	}
	sharedCost = append(sharedCost, utils.MakeCostReportItem("server acquisition", v, v, "shared"))

	v, err = getAmortizedCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "server maintenance",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_SERVER,
		Board:         "server-maintenance",
		Model:         "",
		Sku:           "",
		Location:      location,
	}, true, forgiveMissingEntries)
	if err != nil {
		return nil, errors.Annotate(err, "get shared cost").Err()
	}
	sharedCost = append(sharedCost, utils.MakeCostReportItem("server maintenance", v, v, "shared"))

	v, err = getAmortizedCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "network infra acquisition",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_SERVER,
		Board:         "network-infra-acquisition",
		Model:         "",
		Sku:           "",
		Location:      location,
	}, true, forgiveMissingEntries)
	if err != nil {
		return nil, errors.Annotate(err, "get shared cost").Err()
	}
	sharedCost = append(sharedCost, utils.MakeCostReportItem("network infra acquisition", v, v, "shared"))

	v, err = getAmortizedCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "network infra maintenance",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_SERVER,
		Board:         "network-infra-maintenance",
		Model:         "",
		Sku:           "",
		Location:      location,
	}, true, forgiveMissingEntries)
	if err != nil {
		return nil, errors.Annotate(err, "get shared cost").Err()
	}
	sharedCost = append(sharedCost, utils.MakeCostReportItem("network infra maintenance", v, v, "shared"))

	v, err = getAmortizedCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "quota faft opex",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_SPACE,
		Board:         "quota-faft-opex",
		Model:         "",
		Sku:           "",
		Location:      location,
	}, true, forgiveMissingEntries)
	if err != nil {
		return nil, errors.Annotate(err, "get shared cost").Err()
	}
	sharedCost = append(sharedCost, utils.MakeCostReportItem("quota faft opex", v, v, "shared"))

	v, err = getAmortizedCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "connectivity and misc testbeds",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_SERVER,
		Board:         "connectivity-and-misc-testbeds",
		Model:         "",
		Sku:           "",
		Location:      location,
	}, true, forgiveMissingEntries)
	if err != nil {
		return nil, errors.Annotate(err, "get shared cost").Err()
	}
	sharedCost = append(sharedCost, utils.MakeCostReportItem("connectivity and misc testbeds", v, v, "shared"))

	v, err = getAmortizedCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "control network racks",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_SERVER,
		Board:         "control-network-racks",
		Model:         "",
		Sku:           "",
		Location:      location,
	}, true, forgiveMissingEntries)
	if err != nil {
		return nil, errors.Annotate(err, "get shared cost").Err()
	}
	sharedCost = append(sharedCost, utils.MakeCostReportItem("control network racks", v, v, "shared"))

	v, err = getAmortizedCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "rack setup",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_SERVER,
		Board:         "rack-setup",
		Model:         "",
		Sku:           "",
		Location:      location,
	}, true, forgiveMissingEntries)
	if err != nil {
		return nil, errors.Annotate(err, "get shared cost").Err()
	}
	sharedCost = append(sharedCost, utils.MakeCostReportItem("rack setup", v, v, "shared"))

	v, err = getAmortizedCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "phase deployments",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_SERVER,
		Board:         "phase-deployments",
		Model:         "",
		Sku:           "",
		Location:      location,
	}, true, forgiveMissingEntries)
	if err != nil {
		return nil, errors.Annotate(err, "get shared cost").Err()
	}
	sharedCost = append(sharedCost, utils.MakeCostReportItem("phase deployments", v, v, "shared"))

	v, err = getAmortizedCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "annual maintenance",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_SERVER,
		Board:         "annual-maintenance",
		Model:         "",
		Sku:           "",
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
func getDUTDedicatedHardwareCost(ctx context.Context, m *ufspb.ChromeOSMachine, servo *lab.Servo, location fleetcostpb.Location, forgiveMissingEntries bool) ([]*fleetcostpb.CostReportItem, error) {
	var out []*fleetcostpb.CostReportItem
	ent, err := getCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "DUT cost",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_DUT,
		Board:         m.GetBuildTarget(),
		Model:         m.GetModel(),
		Sku:           m.GetSku(),
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
			Board:         servo.GetServoType(),
			Model:         "",
			Sku:           "",
			Location:      location,
		}, true, forgiveMissingEntries)

		if err != nil {
			return nil, errors.Annotate(err, "servo cost for %q %q %v", servo.GetServoType(), location.String(), forgiveMissingEntries).Err()
		}

		out = append(out, utils.MakeCostReportItem("servo cost", servoCost, servoCost, "dedicated"))
	}
	return out, nil
}

func getCloudCost(ctx context.Context, location fleetcostpb.Location, forgiveMissingEntries bool) (*fleetcostpb.CostReportItem, error) {
	ent, err := getCostIndicatorValue(ctx, &indicatorAttribute{
		ErrorHint:     "annual cloud cost",
		IndicatorType: fleetcostpb.IndicatorType_INDICATOR_TYPE_CLOUD,
		Board:         "",
		Model:         "",
		Sku:           "",
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
