// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"google.golang.org/api/iterator"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/types/known/timestamppb"

	gceproviderpb "go.chromium.org/luci/gce/api/config/v1"
)

const USAGE = `
$ ./gce_quota_checker [--project some-project-ID] path/to/vms.cfg path/to/another/vms.cfg

This will sum up all deployments in the specified vms.cfg files for the
specified GCP project, then check those against the allowed quotas for the
project. This checks:
- VM instances per region
- instanced per VPC network
- in-use IPs per region
- CPUs per region (N1)
- N2 CPUs per region
- HDD per region
- SSD per region
- local SSD per region + family

Hints:
- region == "us-east1"
- zone == "us-east1-d"
- family == "n1", "n2", "e2", etc

NOTE: need to run 'gcloud auth application-default login' locally first. Also,
authenticating on cloudtop (GCE) may be difficult. Should still work on non-GCE
platforms (eg gMac).
`

type quotaVals struct {
	max           int64
	starlarkUsage int64
	gcloudUsage   int64
	desc          string
}

func (q quotaVals) GetUsagePercent() float64 {
	return 100 * float64(q.starlarkUsage) / float64(q.max)
}

func (q quotaVals) GetDescPretty() string {
	return fmt.Sprintf("%s at %.2f%% (%d of %d)", q.desc, q.GetUsagePercent(), q.starlarkUsage, q.max)
}

type regionQuotas struct {
	// One-per instance quotas
	instancesQuota quotaVals
	ipsQuota       quotaVals

	// Some families' CPUs share the same quota, while others have their
	// own. Use cpusQuota for the former and cpusPerFamilyQuota for the
	// latter.
	cpusQuota          quotaVals
	cpusPerFamilyQuota map[string]*quotaVals

	// Disk
	hddQuota       quotaVals
	remoteSSDQuota quotaVals
	// GCE tracks this one across both region and VM family (n1, e2, etc).
	localSSDPerFamilyQuota map[string]*quotaVals
}

func parseFlags() (string, float64, bool, []string) {
	gcpProject := flag.String("project", "google.com:chromecompute", "ID of the project to get quota for.")
	isVerbose := flag.Bool("verbose", false, "Prints full quota usage.")
	cutoffPercent := flag.Float64("cutoff-percent", 100.0, "Percentage of a quota's consumption to fail on.")
	flag.Usage = func() {
		fmt.Printf("%v\n", USAGE)
		os.Exit(1)
	}
	flag.Parse()
	paths := flag.Args()
	if len(paths) == 0 || *cutoffPercent < 0.0 || *cutoffPercent > 100.0 {
		flag.Usage()
	}
	return *gcpProject, *cutoffPercent, *isVerbose, paths
}

func queryTimeSeriesQuota(ctx context.Context, client *monitoring.MetricClient, quotaName string, project string) *monitoring.TimeSeriesIterator {
	// Get all points in the metric in the past 1 day. Some of the metrics
	// don't seem to get updated very often, so need a big window.
	startTime := time.Now().UTC().Add(time.Hour * -24)
	endTime := time.Now().UTC()
	monitoringRequest := &monitoringpb.ListTimeSeriesRequest{
		Name:   "projects/" + project,
		Filter: fmt.Sprintf(`metric.type = "compute.googleapis.com/quota/%s"`, quotaName),
		Interval: &monitoringpb.TimeInterval{
			StartTime: &timestamppb.Timestamp{
				Seconds: startTime.Unix(),
			},
			EndTime: &timestamppb.Timestamp{
				Seconds: endTime.Unix(),
			},
		},
	}
	return client.ListTimeSeries(ctx, monitoringRequest)
}

func loadCfg(cfgPath string) *gceproviderpb.Configs {
	in, err := os.ReadFile(cfgPath)
	if err != nil {
		log.Fatalln("Error reading cfg file:", err)
	}
	configs := &gceproviderpb.Configs{}
	opts := prototext.UnmarshalOptions{
		DiscardUnknown: true,
	}
	if err := opts.Unmarshal(in, configs); err != nil {
		log.Fatalln("Failed to parse gce config:", err)
	}
	return configs
}

func zoneToRegion(zone string, possibleRegions []string) string {
	for _, region := range possibleRegions {
		if strings.HasPrefix(zone, region) {
			return region
		}
	}
	log.Fatalln("Couldn't find region for: ", zone)
	return zone
}

// getFamilyAndCoresFromType returns the family (eg: "e2" or "n1") and the
// core count of the machine type.
func getFamilyAndCoresFromType(machineType string) (string, int64) {
	// FIXME: Expand this as new machine types are encountered?
	if machineType == "g1-small" {
		return "g1", 1
	} else if machineType == "e2-medium" {
		return "e2", 2
	}
	// Regex matches e.g. "n1-standard-8" and pulls out "n1" as the family
	// and "8" as the core count.
	machineTypeRe := regexp.MustCompile(`^([^-]+)-[^-]+-(\d+)$`)
	matches := machineTypeRe.FindStringSubmatch(machineType)
	if len(matches) != 3 {
		log.Fatalln("Unknown machine type: ", machineType)
	}
	cores, err := strconv.Atoi(matches[2])
	if err != nil {
		log.Fatalln("Unknown core count in machine type: ", machineType)
	}
	return matches[1], int64(cores)
}

// getDiskStats returns the amount (HDD, remote SSD, local SSD) in GB that the
// disk will take-up.
func getDiskStats(disk *gceproviderpb.Disk) (int64, int64, int64) {
	if disk.Size == 0 {
		log.Fatalln("Disk doesn't have a size: ", disk)
	}
	if disk.Type != "" {
		parts := strings.Split(disk.Type, "/")
		diskType := parts[len(parts)-1]
		if diskType == "local-ssd" {
			return 0, 0, disk.Size
		} else if diskType == "pd-ssd" {
			return 0, disk.Size, 0
		} else {
			log.Fatalln("Unknown disk type: ", diskType)
		}

	}
	// If no type is specified, safe to assume it's HDD.
	return disk.Size, 0, 0
}

func getRegionQuotas(ctx context.Context, project string) (map[string]*regionQuotas, []string) {
	// Get regions and their quotas. Simply calling ListRegion will get
	// a variety of quota info for each region, which includes most of
	// what we care about.
	var regionNames []string
	quotasPerRegion := make(map[string]*regionQuotas)
	c, err := compute.NewRegionsRESTClient(ctx)
	if err != nil {
		log.Fatalln("Error init'ing gcloud client:", err)
	}
	defer func() {
		if err = c.Close(); err != nil {
			log.Fatalln(err)
		}
	}()

	req := &computepb.ListRegionsRequest{
		Project: project,
	}
	it := c.List(ctx, req)
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalln("Error listing regions:", err)
		}
		quotas := regionQuotas{
			localSSDPerFamilyQuota: make(map[string]*quotaVals),
			cpusPerFamilyQuota:     make(map[string]*quotaVals),
		}
		region := *resp.Name
		for _, quota := range resp.Quotas {
			switch *quota.Metric {
			case "CPUS":
				quotas.cpusQuota = quotaVals{max: int64(*quota.Limit), gcloudUsage: int64(*quota.Usage), desc: "CPUs in " + region}
			case "IN_USE_ADDRESSES":
				quotas.ipsQuota = quotaVals{max: int64(*quota.Limit), gcloudUsage: int64(*quota.Usage), desc: "IPs in " + region}
			case "INSTANCES":
				quotas.instancesQuota = quotaVals{max: int64(*quota.Limit), gcloudUsage: int64(*quota.Usage), desc: "Instances in " + region}
			case "DISKS_TOTAL_GB":
				quotas.hddQuota = quotaVals{max: int64(*quota.Limit), gcloudUsage: int64(*quota.Usage), desc: "HDD in " + region}
			case "SSD_TOTAL_GB":
				quotas.remoteSSDQuota = quotaVals{max: int64(*quota.Limit), gcloudUsage: int64(*quota.Usage), desc: "Remote SSDs in " + region}
			default:
				if strings.HasSuffix(*quota.Metric, "_CPUS") {
					cpuFamily := strings.ToLower(strings.TrimSuffix(*quota.Metric, "_CPUS"))
					quotas.cpusPerFamilyQuota[cpuFamily] = &quotaVals{max: int64(*quota.Limit), gcloudUsage: int64(*quota.Usage), desc: cpuFamily + " CPUs in " + region}
				}
			}
		}
		quotasPerRegion[region] = &quotas
		regionNames = append(regionNames, region)
	}
	return quotasPerRegion, regionNames
}

func getLocalSSDQuotas(ctx context.Context, project string, quotasPerRegion map[string]*regionQuotas) {
	// Get local-SSD per region per vm-family quotas. Need to query a quota
	// metric for this. Note: we query the "limit" here and then calculate
	// the expected "usage" later on by reading gce-provider vms.cfg files.
	client, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		log.Fatalln(err)
	}
	defer func() {
		if err = client.Close(); err != nil {
			log.Fatalln(err)
		}
	}()
	timeSeriesIterator := queryTimeSeriesQuota(ctx, client, "local_ssd_total_storage_per_vm_family/limit", project)
	for {
		timeSeriesResp, err := timeSeriesIterator.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			log.Fatalln(err)
		}
		var machineFamily string
		var region string
		if value, ok := timeSeriesResp.Metric.Labels["vm_family"]; ok {
			machineFamily = strings.ToLower(value)
		}
		if value, ok := timeSeriesResp.Resource.Labels["location"]; ok {
			region = value
		}
		regionQuota, ok := quotasPerRegion[region]
		if !ok {
			continue
		}
		// Points[0] should correspond to the most recent value for the
		// quota, useful in case the quota limit gets bumped for the
		// project.
		regionQuota.localSSDPerFamilyQuota[machineFamily] = &quotaVals{max: timeSeriesResp.Points[0].Value.GetInt64Value(), desc: "Local SSDs for " + machineFamily + " in " + region}
	}
}

func getNetworkQuotas(ctx context.Context, project string) map[string]*quotaVals {
	// Get the number of instances per network. GCE's monitoring API refers to
	// networks by their ID while while gce-provider uses their name. So need
	// to build a mapping between the two.
	networkClient, err := compute.NewNetworksRESTClient(ctx)
	if err != nil {
		log.Fatalln(err)
	}
	defer func() {
		if err = networkClient.Close(); err != nil {
			log.Fatalln(err)
		}
	}()

	metricClient, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		log.Fatalln(err)
	}
	defer func() {
		if err = metricClient.Close(); err != nil {
			log.Fatalln(err)
		}
	}()

	quotasPerNetwork := make(map[string]*quotaVals)
	networkIDToName := make(map[uint64]string)
	req := &computepb.ListNetworksRequest{
		Project: project,
	}
	it := networkClient.List(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		networkIDToName[*resp.Id] = *resp.Name
	}

	timeSeriesIterator := queryTimeSeriesQuota(ctx, metricClient, "instances_per_vpc_network/limit", project)
	for {
		timeSeriesResp, err := timeSeriesIterator.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalln(err)
		}
		for labelKey, labelVal := range timeSeriesResp.Resource.Labels {
			if labelKey == "network_id" {
				labelValInt, err := strconv.ParseUint(labelVal, 10, 64)
				if err != nil {
					log.Fatalln("Uknown network ID format: ", labelVal)
				}
				networkName, ok := networkIDToName[labelValInt]
				if !ok {
					log.Fatalln("Unknown network id: ", labelVal)
				}
				quotasPerNetwork[networkName] = &quotaVals{max: timeSeriesResp.Points[0].Value.GetInt64Value(), desc: "Instances in network " + networkName}
				break
			}
		}
	}
	return quotasPerNetwork
}

func parseCfgFiles(project string, cfgPaths []string, regionNames []string, quotasPerRegion map[string]*regionQuotas, quotasPerNetwork map[string]*quotaVals) {
	// Read vms.cfg. The 'configs' slice is used to flatten all configs
	// across all possible cfg files.
	var configs []*gceproviderpb.Config
	for _, cfgPath := range cfgPaths {
		thisConfig := loadCfg(cfgPath)
		configs = append(configs, thisConfig.Vms...)
	}

	// Parse vms.cfg
	for _, config := range configs {
		if config.Attributes.Project != project {
			continue
		}
		region := zoneToRegion(config.Attributes.Zone, regionNames)

		// Get max num instances
		maxInstances := config.Amount.Max
		for _, scheduledChange := range config.Amount.Change {
			if scheduledChange.Max > maxInstances {
				maxInstances = scheduledChange.Max
			}
		}
		quotasPerRegion[region].instancesQuota.starlarkUsage += int64(maxInstances)

		// Get network
		if len(config.Attributes.NetworkInterface) != 1 || config.Attributes.NetworkInterface[0].Network == "" {
			log.Fatalln("Unknown network config on ", config.Prefix)
		}
		network := config.Attributes.NetworkInterface[0].Network
		network, _ = strings.CutPrefix(network, "global/networks/")
		networkQuota, ok := quotasPerNetwork[network]
		if ok {
			// Some networks don't have a limit, in which case GCE's quota API won't
			// have reported anything about it.
			networkQuota.starlarkUsage += int64(maxInstances)
		}

		// Get IP address
		if len(config.Attributes.NetworkInterface[0].AccessConfig) > 1 {
			log.Fatalln("Unknown access config on ", config.Prefix)
		} else if len(config.Attributes.NetworkInterface[0].AccessConfig) == 1 {
			quotasPerRegion[region].ipsQuota.starlarkUsage += int64(maxInstances)
		}

		// Get core count. Families like n1 and e2 share the same base
		// CPU quota, but families like n2 and n2d have their own.
		parts := strings.Split(config.Attributes.MachineType, "/")
		mt := parts[len(parts)-1]
		family, cores := getFamilyAndCoresFromType(mt)
		maxCores := int64(maxInstances) * cores
		customFamilyQuota, hasCustomFamilyQuota := quotasPerRegion[region].cpusPerFamilyQuota[family]
		// In late 2024 / early 2025, GCE started reporting quotas for *every* CPU family.
		// Most of these quotas have "0" usage despite CPUs of the family being deployed.
		// In such a case, we assume those CPUs are deducted from the generic CPU quota.
		if hasCustomFamilyQuota {
			customFamilyQuota.starlarkUsage += maxCores
			if customFamilyQuota.gcloudUsage == 0 {
				// To be safe, deduct from the region-wide CPU quota if we suspect
				// GCE is reporting one of those ghost quotas here.
				quotasPerRegion[region].cpusQuota.starlarkUsage += maxCores
			}
		} else {
			quotasPerRegion[region].cpusQuota.starlarkUsage += maxCores
		}

		// Get disk info
		var totalHDD, totalRemoteSSD, totalLocalSSD int64
		for _, disk := range config.Attributes.Disk {
			hdd, remoteSSD, localSSD := getDiskStats(disk)
			totalHDD += hdd
			totalRemoteSSD += remoteSSD
			totalLocalSSD += localSSD
		}
		quotasPerRegion[region].hddQuota.starlarkUsage += int64(maxInstances) * totalHDD
		quotasPerRegion[region].remoteSSDQuota.starlarkUsage += int64(maxInstances) * totalRemoteSSD
		// Not all regions/machine types have local SSD quota. So
		// quotasPerRegion[region].localSSDPerFamilyQuota[family] might
		// not have an entry initialized if there's no deployment of
		// local SSDs for that region + family combo.
		if totalLocalSSD > 0 {
			quotasPerRegion[region].localSSDPerFamilyQuota[family].starlarkUsage += int64(maxInstances) * totalLocalSSD
		}
	}
}

func findQuotaErrors(quotasPerRegion map[string]*regionQuotas, quotasPerNetwork map[string]*quotaVals, cutoffPercent float64, isVerbose bool) []string {
	var quotaErrors []string
	// Flatten all quotas into a single slice for easier iterating.
	var allQuotas []quotaVals
	for _, quota := range quotasPerNetwork {
		allQuotas = append(allQuotas, *quota)
	}
	for _, quotas := range quotasPerRegion {
		allQuotas = append(
			allQuotas,
			quotas.instancesQuota,
			quotas.ipsQuota,
			quotas.cpusQuota,
			quotas.hddQuota,
			quotas.remoteSSDQuota,
		)
		for _, quota := range quotas.localSSDPerFamilyQuota {
			allQuotas = append(allQuotas, *quota)
		}
		for _, quota := range quotas.cpusPerFamilyQuota {
			allQuotas = append(allQuotas, *quota)
		}
	}
	for _, quota := range allQuotas {
		percent := quota.GetUsagePercent()
		desc := quota.GetDescPretty()
		if percent > cutoffPercent {
			quotaErrors = append(quotaErrors, desc)
		}
		// We're tracking quotas with zero usage (eg: CPUs in obscure
		// EMEA regions). So filter them out by only printing quotas
		// with some usage.
		if isVerbose && percent > 0 {
			fmt.Println(desc)
		}
	}
	return quotaErrors
}

func main() {
	ctx := context.Background()
	project, cutoffPercent, isVerbose, cfgPaths := parseFlags()

	// Query GCE for all relevant quotas for the project.
	quotasPerRegion, regionNames := getRegionQuotas(ctx, project)
	getLocalSSDQuotas(ctx, project, quotasPerRegion)
	quotasPerNetwork := getNetworkQuotas(ctx, project)

	// Parse vms.cfgs and determine max quota usage for all deployments for
	// the oroject.
	parseCfgFiles(project, cfgPaths, regionNames, quotasPerRegion, quotasPerNetwork)

	// Find where used > max for all quotas.
	quotaErrors := findQuotaErrors(quotasPerRegion, quotasPerNetwork, cutoffPercent, isVerbose)
	if len(quotaErrors) > 0 {
		if isVerbose {
			fmt.Println()
		}
		fmt.Fprintf(os.Stderr, "UH OH! One more quotas at or above %.2f\n", cutoffPercent)
		for _, quotaError := range quotaErrors {
			fmt.Fprintln(os.Stderr, quotaError)
		}
		os.Exit(1)
	}
}
