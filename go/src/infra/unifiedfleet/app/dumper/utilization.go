// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dumper

import (
	"context"
	"fmt"
	"strings"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/tsmon"
	"go.chromium.org/luci/common/tsmon/field"
	"go.chromium.org/luci/common/tsmon/metric"
	"go.chromium.org/luci/common/tsmon/monitor"
	"go.chromium.org/luci/common/tsmon/store"
	"go.chromium.org/luci/common/tsmon/target"
	"go.chromium.org/luci/server"
	tsmonsrv "go.chromium.org/luci/server/tsmon"

	"infra/cros/dutstate"
	invV1 "infra/libs/skylab/inventory"
	ufspb "infra/unifiedfleet/api/v1/models"
	"infra/unifiedfleet/app/config"
	"infra/unifiedfleet/app/model/inventory"
	"infra/unifiedfleet/app/model/registration"
	"infra/unifiedfleet/app/util"
)

// inventoryCounter collects number of DUTs per bucket and status.
type inventoryCounter map[bucket]int

// suMetric is the metric name for scheduling unit count.
var suMetric = metric.NewInt(
	"chromeos/skylab/inventory/scheduling_unit_count",
	"The number of scheduling units in a given bucket",
	nil,
	field.String("board"),
	field.String("model"),
	field.String("pool"),
	field.String("environment"),
	field.String("zone"),
	field.String("swarming_instance"),
	field.String("status"),
	field.String("os"),
)

// Only MachineLSEs for chromeOS and browser
var utilizationExportNamespaces = []string{util.OSNamespace, util.BrowserNamespace}

var suMetricState *tsmon.State

// initializeUFSInventoryTsmonState creates a tsmon.State for tracking the suMetric.
func initializeUFSInventoryTsmonState(srv *server.Server) error {
	suMetricState = tsmon.NewState()
	suMetricState.SetStore(store.NewInMemory(&target.Task{
		DataCenter:  "appengine",
		ServiceName: srv.Options.TsMonServiceName,
		JobName:     srv.Options.TsMonJobName,
		HostName:    srv.Options.Hostname,
	}))
	suMetricState.InhibitGlobalCallbacksOnFlush()
	var mon monitor.Monitor
	switch {
	case srv.Options.Prod && srv.Options.TsMonAccount != "":
		var err error
		mon, err = tsmonsrv.NewProdXMonitor(srv.Context, 4096, srv.Options.TsMonAccount)
		if err != nil {
			return err
		}
	case !srv.Options.Prod:
		mon = monitor.NewDebugMonitor("")
	default:
		mon = monitor.NewNilMonitor()
	}
	suMetricState.SetMonitor(mon)
	return nil
}

// reportUFSInventoryCronHandler runs the ufs dut metric collection
// for each namespace in `utilizationExportNamespaces`
func reportUFSInventoryCronHandler(ctx context.Context) (err error) {
	var errs []error
	// Add metric state to context
	mctx := tsmon.WithState(ctx, suMetricState)
	// Reset the metric to stop reporting no-longer-existing devices and states.
	suMetricState.Store().Reset(mctx, suMetric)
	for _, ns := range utilizationExportNamespaces {
		datastoreNamespace := util.ClientToDatastoreNamespace[ns]
		ctx, err = util.SetupDatastoreNamespace(ctx, datastoreNamespace)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		var c inventoryCounter
		if c, err = reportUFSInventoryForNamespace(ctx, ns); err != nil {
			errs = append(errs, err)
			continue
		}
		// Report the metrics
		for b, count := range c {
			suMetric.Set(mctx, int64(count), b.board, b.model, b.pool, b.environment, b.zone, b.swarmingInstance, b.status, b.os)
		}
	}
	// Flush the metrics
	if err := suMetricState.ParallelFlush(ctx, nil, 32); err != nil {
		err = errors.Annotate(err, "failed to flush values to monitoring").Err()
		errs = append(errs, err)
	}
	return errors.Append(errs...)
}

// reportUFSInventoryForNamespace push the ufs duts metrics to tsmon
func reportUFSInventoryForNamespace(ctx context.Context, ns string) (c inventoryCounter, err error) {
	logging.Infof(ctx, "Reporting UFS inventory DUT metrics for namespace %s", ns)

	env := config.Get(ctx).SelfStorageBucket
	// Get all the MachineLSEs
	lses, err := getAllMachineLSEs(ctx, false)
	if err != nil {
		return nil, err
	}
	idTolseMap := make(map[string]*ufspb.MachineLSE, 0)
	for _, lse := range lses {
		idTolseMap[lse.GetName()] = lse
	}
	// Get all Machines
	machines, err := getAllMachines(ctx, false)
	if err != nil {
		return nil, err
	}
	idTomachineMap := make(map[string]*ufspb.Machine, 0)
	for _, machine := range machines {
		idTomachineMap[machine.GetName()] = machine
	}

	// Scheduling Units are OS namespace only
	var lseInSUnitMap map[string]bool
	c = make(inventoryCounter)
	if ns == util.OSNamespace {
		sUnits, err := getAllSchedulingUnits(ctx, false)
		if err != nil {
			return nil, err
		}
		// Map for MachineLSEs associated with SchedulingUnit for easy search.
		lseInSUnitMap = make(map[string]bool)
		for _, su := range sUnits {
			if len(su.GetMachineLSEs()) > 0 {
				suLses := make([]*ufspb.MachineLSE, len(su.GetMachineLSEs()))
				for i, lseID := range su.GetMachineLSEs() {
					suLses[i] = idTolseMap[lseID]
				}

				b, err := getBucketForSchedulingUnit(su, suLses, idTomachineMap, env)
				if err != nil {
					logging.Warningf(ctx, err.Error())
					continue
				}
				c[*b]++
				for _, lseName := range su.GetMachineLSEs() {
					lseInSUnitMap[lseName] = true
				}
			}
		}
	}
	for _, lse := range lses {
		name := lse.GetName()
		if ns == util.OSNamespace && lseInSUnitMap[name] {
			continue
		}
		machine, err := getMachineForLse(lse, idTomachineMap)
		if err != nil {
			logging.Warningf(ctx, err.Error())
			continue
		}
		b := getBucketForDevice(lse, machine, env, ns)
		c[*b]++
	}
	logging.Infof(ctx, "report UFS inventory metrics for %d device buckets", len(c))
	return c, nil
}

// getMachineForLse returns the Machine that's attached to the MachineLSE
// iff the MachineLSE references exactly one Machine
func getMachineForLse(lse *ufspb.MachineLSE, idTomachineMap map[string]*ufspb.Machine) (*ufspb.Machine, error) {
	machines := lse.GetMachines()
	if n := len(machines); n != 1 {
		return nil, errors.Reason("report ufs inventory cron handler: %d machines %v associated with %q", n, machines, lse.GetName()).Err()
	}
	machine, ok := idTomachineMap[machines[0]]
	if !ok {
		return nil, errors.Reason("report ufs inventory cron handler: machine %s not found for LSE %s", machines[0], lse.GetName()).Err()
	}
	return machine, nil
}

// getBucketForDevice instantiates a *bucket for a given MachineLSE and
// corresponding Machine
func getBucketForDevice(lse *ufspb.MachineLSE, machine *ufspb.Machine, env string, ns string) *bucket {
	b := &bucket{
		board:                 machine.GetChromeosMachine().GetBuildTarget(),
		model:                 machine.GetChromeosMachine().GetModel(),
		pool:                  "[None]",
		environment:           env,
		zone:                  lse.GetZone(),
		swarmingInstance:      "[None]",
		status:                dutstate.ConvertFromUFSState(lse.GetResourceState()).String(),
		os:                    "[None]",
		maintenanceConfigName: "[None]",
	}
	if lse.GetMaintenanceConfigName() != "" {
		b.maintenanceConfigName = lse.GetMaintenanceConfigName()
	}
	switch ns {
	case util.OSNamespace:
		if dut := lse.GetChromeosMachineLse().GetDeviceLse().GetDut(); dut != nil {
			b.pool = getReportPool(dut.GetPools())
		}
		if labstation := lse.GetChromeosMachineLse().GetDeviceLse().GetLabstation(); labstation != nil {
			b.pool = getReportPool(labstation.GetPools())
		}
		b.swarmingInstance = "chromeos-swarming"
		b.os = "chromeos"
	case util.BrowserNamespace:
		b.pool = getReportPool(lse.GetOwnership().GetPools())
		b.swarmingInstance = lse.GetOwnership().GetSwarmingInstance()
		if lse.GetChromeBrowserMachineLse() != nil {
			b.os = strings.ToLower(lse.GetChromeBrowserMachineLse().GetOsVersion().GetValue())
		}
	}
	return b
}

// machineFieldToValueFunc is a helper type for extracting the DUT fields
// for a given scheduling unit
type machineFieldToValueFunc func(machine *ufspb.Machine) string

var (
	machineBoardValueFunc = func(machine *ufspb.Machine) string { return machine.GetChromeosMachine().GetBuildTarget() }
	machineModelValueFunc = func(machine *ufspb.Machine) string { return machine.GetChromeosMachine().GetModel() }
)

// getBucketForSchedulingUnit instantiates a *bucket for a given SchedulingUnit
// and corresponding DUTs.
// Depending on the ExposeType, the bucket dimensions are based on a combination
// of the primary DUT values and an aggregate on all DUTs
func getBucketForSchedulingUnit(su *ufspb.SchedulingUnit, lses []*ufspb.MachineLSE, idTomachineMap map[string]*ufspb.Machine, env string) (*bucket, error) {
	b := &bucket{
		board:            "[None]",
		model:            "[None]",
		pool:             getReportPool(su.GetPools()),
		environment:      env,
		zone:             "[None]",
		swarmingInstance: "chromeos-swarming",
		status:           schedulingUnitStatusFromLses(lses),
		os:               "chromeos",
	}
	// fields from all DUTs
	switch su.GetExposeType() {
	case ufspb.SchedulingUnit_DEFAULT:
		fallthrough
	case ufspb.SchedulingUnit_DEFAULT_PLUS_PRIMARY:
		board, err := schedulingUnitLabelForLses(lses, idTomachineMap, machineBoardValueFunc)
		if err != nil {
			return nil, err
		}
		model, err := schedulingUnitLabelForLses(lses, idTomachineMap, machineModelValueFunc)
		if err != nil {
			return nil, err
		}
		b.board = board
		b.model = model
	case ufspb.SchedulingUnit_STRICTLY_PRIMARY_ONLY:
		// nothing from all DUTs
	default:
		return nil, errors.Reason("Unknown SchedulingUnit Expose Type for %s", su.GetName()).Err()
	}
	// fields from primary
	var primaryLse *ufspb.MachineLSE
	for _, lse := range lses {
		if lse.GetName() == su.GetPrimaryDut() {
			primaryLse = lse
			break
		}
	}
	switch su.GetExposeType() {
	case ufspb.SchedulingUnit_STRICTLY_PRIMARY_ONLY:
		if primaryLse == nil {
			return nil, errors.Reason("Could not find primary MachineLSE %s for scheduling unit %s", su.GetPrimaryDut(), su.GetName()).Err()
		}
		machine, err := getMachineForLse(primaryLse, idTomachineMap)
		if err != nil {
			return nil, err
		}
		b.board = machine.GetChromeosMachine().GetBuildTarget()
		b.model = machine.GetChromeosMachine().GetModel()
		fallthrough
	case ufspb.SchedulingUnit_DEFAULT_PLUS_PRIMARY:
		if primaryLse == nil {
			return nil, errors.Reason("Could not find primary MachineLSE %s for scheduling unit %s", su.GetPrimaryDut(), su.GetName()).Err()
		}
		b.zone = primaryLse.GetZone()
	case ufspb.SchedulingUnit_DEFAULT:
		// nothing from the primary DUT
	default:
		return nil, errors.Reason("Unknown SchedulingUnit Expose Type for %s", su.GetName()).Err()
	}
	return b, nil
}

// schedulingUnitLabelForLses calculates an overall label for a scheduling unit
// given a list of MachineLSEs
func schedulingUnitLabelForLses(lses []*ufspb.MachineLSE, idTomachineMap map[string]*ufspb.Machine, f machineFieldToValueFunc) (string, error) {
	machines := make([]*ufspb.Machine, len(lses))
	for i, lse := range lses {
		machine, err := getMachineForLse(lse, idTomachineMap)
		if err != nil {
			return "", err
		}
		machines[i] = machine
	}
	labelSet := make(map[string]struct{}) // Set of all label values
	for _, machine := range machines {
		machineLabel := f(machine)
		if len(machineLabel) > 0 {
			labelSet[machineLabel] = struct{}{}
		}
	}
	labels := make([]string, 0, len(labelSet))
	for k := range labelSet {
		labels = append(labels, k)
	}
	return summarizeValues(labels), nil
}

// schedulingUnitStatusFromLses calculates a weighted status based on all DUTs
// to represent the scheduling unit
func schedulingUnitStatusFromLses(lses []*ufspb.MachineLSE) string {
	states := make([]string, len(lses))
	for i, lse := range lses {
		s := dutstate.ConvertFromUFSState(lse.GetResourceState()).String()
		states[i] = s
	}
	return util.SchedulingUnitDutState(states)
}

// bucket contains static DUT dimensions.
//
// These dimensions do not change often. If all DUTs with a given set of
// dimensions are removed, the related metric is not automatically reset. The
// metric will get reset eventually.
type bucket struct {
	board                 string
	model                 string
	pool                  string
	environment           string
	zone                  string
	swarmingInstance      string
	status                string
	os                    string
	maintenanceConfigName string
}

func (b *bucket) String() string {
	return fmt.Sprintf("board: %s, model: %s, pool: %s, env: %s, zone: %q, swarmingInstance: %s, status: %s, os: %s", b.board, b.model, b.pool, b.environment, b.zone, b.swarmingInstance, b.status, b.os)
}

func summarizeValues(vs []string) string {
	switch len(vs) {
	case 0:
		return "[None]"
	case 1:
		return vs[0]
	default:
		return "[Multiple]"
	}
}

func isManagedPool(p string) bool {
	_, ok := invV1.SchedulableLabels_DUTPool_value[p]
	return ok
}

func getReportPool(pools []string) string {
	p := summarizeValues(pools)
	if isManagedPool(p) {
		return fmt.Sprintf("managed:%s", p)
	}
	return p
}

func getAllMachineLSEs(ctx context.Context, keysOnly bool) ([]*ufspb.MachineLSE, error) {
	var lses []*ufspb.MachineLSE
	for startToken := ""; ; {
		res, nextToken, err := inventory.ListMachineLSEs(ctx, pageSize, startToken, nil, keysOnly)
		if err != nil {
			return nil, errors.Annotate(err, "get all MachineLSEs").Err()
		}
		lses = append(lses, res...)
		if nextToken == "" {
			break
		}
		startToken = nextToken
	}
	return lses, nil
}

func getAllMachines(ctx context.Context, keysOnly bool) ([]*ufspb.Machine, error) {
	var machines []*ufspb.Machine
	for startToken := ""; ; {
		res, nextToken, err := registration.ListMachines(ctx, pageSize, startToken, nil, keysOnly)
		if err != nil {
			return nil, errors.Annotate(err, "get all Machines").Err()
		}
		machines = append(machines, res...)
		if nextToken == "" {
			break
		}
		startToken = nextToken
	}
	return machines, nil
}
