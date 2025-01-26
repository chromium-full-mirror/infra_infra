// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	conf "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	dut_api "go.chromium.org/chromiumos/config/go/test/lab/api"
	server "go.chromium.org/chromiumos/test/ctpv2/common/server_template"
)

func executor(req *api.InternalTestplan, log *log.Logger, commonParams *server.CommonFilterParams) (*api.InternalTestplan, error) {
	// Will be the map of hardware which we can make provisionInfo for directly
	foundHW := make(map[string]bool)

	var swRequirements *api.LegacySW

	for _, targetOptions := range req.GetSuiteInfo().GetSuiteMetadata().GetSchedulingUnitOptions() {
		for _, target := range targetOptions.GetSchedulingUnits() {
			generateProvisionInfoForSchedUnit(target, foundHW, log)
		}
	}

	// TODO(oldProto-azrahman): remove when schedulingOptions are fully rolled in.
	for _, target := range req.GetSuiteInfo().GetSuiteMetadata().GetSchedulingUnits() {
		generateProvisionInfoForSchedUnit(target, foundHW, log)
	}

	// TODO (oldProto-azrahman): remove old proto stuffs when schedulingUnits are fully rolled in.
	for _, legacyTarget := range req.GetSuiteInfo().GetSuiteMetadata().GetTargetRequirements() {

		for _, innerTarget := range legacyTarget.GetHwRequirements().GetHwDefinition() {
			swRequirements = legacyTarget.GetSwRequirement()
			generateProvisionInfo(innerTarget, swRequirements, log)
			addToFoundCache(innerTarget, foundHW)

		}
	}

	// Create Dynamic Updates and lookup information.
	if err := GenerateDynamicInfo(req); err != nil {
		return req, err
	}

	return req, nil
}

// generateProvisionInfoForSchedUnit generates provision info for provided scheduling unit
func generateProvisionInfoForSchedUnit(target *api.SchedulingUnit, hwFoundMap map[string]bool, log *log.Logger) {
	primaryHwTarget := target.GetPrimaryTarget().GetSwarmingDef()
	addToFoundCache(primaryHwTarget, hwFoundMap)
	generateProvisionInfo(primaryHwTarget, target.GetPrimaryTarget().GetSwReq(), log)

	for _, t := range target.GetCompanionTargets() {
		generateProvisionInfo(t.GetSwarmingDef(), t.GetSwReq(), log)
		addToFoundCache(t.GetSwarmingDef(), hwFoundMap)
	}
}

func addToFoundCache(hwTarget *api.SwarmingDefinition, foundHW map[string]bool) {
	switch hw := hwTarget.GetDutInfo().GetDutType().(type) {
	case *dut_api.Dut_Chromeos:
		board := hw.Chromeos.GetDutModel().GetBuildTarget()
		variant := hwTarget.GetVariant()
		if variant != "" {
			board = fmt.Sprintf("%s-%s", board, variant)
		}
		foundHW[board] = true
	}
}

// Extract the milestone # as an int64 from the standard gcs path.
func milestoneFromGcs(gcsPath string) (int64, error) {
	// example: gs://chromeos-image-archive/snappy-kernelnext-release/R122-15753.34.0
	parts := strings.Split(gcsPath, "/")

	if len(parts) < 2 {
		return 0, fmt.Errorf("improper or incomplete gcs path given: %s", gcsPath)
	}
	// Will now be R122-15753.34.0
	fullBuildNum := parts[len(parts)-1]

	// Will now be ['R122', '15753.34.0'] 0 index --> 'R122'
	f := strings.Split(fullBuildNum, "-")[0]

	// Replace R with "", then convert it to int64
	// 122
	milestone, err := strconv.ParseInt(strings.Replace(f, "R", "", -1), 10, 64)
	if err != nil {
		return -1, err
	}
	return milestone, nil
}

// Remove any variant info from the board and return the board, only
func boardOnly(targ string) (string, error) {
	f := strings.Split(targ, "-")
	if len(f) == 1 {
		return targ, nil
	}
	if len(f) == 0 {
		return "", fmt.Errorf("no board was provided: %s", targ)
	}
	return f[0], nil
}

func getVariantFromBuildTarg(board string) string {
	// foo-bar --> bar
	parts := strings.Split(board, "-")
	if len(parts) < 2 {
		return ""
	}
	return strings.Join(parts[1:][:], "-")

}

// format a GCS path using the standard string base.
func createGcsPath(board string, channel string, build string, milestone int64) string {
	return fmt.Sprintf("gs://chromeos-image-archive/%s-%s/R%v-%s", board, channel, milestone, build)
}

func generateCrosImageProvisionInfo(target *api.SwarmingDefinition, swReq *api.LegacySW, log *log.Logger) {
	current := target.GetProvisionInfo()
	path := swReq.GetGcsPath()
	log.Println(fmt.Sprintf("generateCrosImageProvisionInfo PATH: %s", path))
	log.Println(fmt.Sprintf("generateCrosImageProvisionInfo sw: %s", swReq))

	if path != "" {
		installInfo := &api.InstallRequest{
			ImagePath: &conf.StoragePath{
				Path:     path,
				HostType: conf.StoragePath_GS,
			},
		}

		info := &api.ProvisionInfo{
			Type:           api.ProvisionInfo_CROS,
			InstallRequest: installInfo,
		}
		target.ProvisionInfo = append(current, info)

	}
}

// Currently this is simple... just append the new provision info onto the existing.
// We are appending it in case another provsion filter has run and populated info.
func generateProvisionInfo(target *api.SwarmingDefinition, swReq *api.LegacySW, log *log.Logger) {
	switch target.GetDutInfo().GetDutType().(type) {
	case *dut_api.Dut_Chromeos:
		log.Println("chromeos DUT")
		generateCrosImageProvisionInfo(target, swReq, log)
	}

}

func getBuildsForMilestone(log *log.Logger, milestone int64) (string, []string, error) {
	log.Println("in parent check")
	bqIter, err := queryForBuilds(log)
	if err != nil {
		return "", nil, fmt.Errorf("unable to determine stabily: %s", err)
	}

	data := iterThroughData(bqIter, log)
	log.Println("past iter check")

	return determineBuilds(data, milestone)
}

// determineBuilds will return the latest build from the given milestone, and boards which built on that build.
// Variants are considered a unique board.
// Latest, will be either the most recent, or most recent-1; to cover for the case where the most recent is still
// "mid build", and boards have not completed their builds yet.
func determineBuilds(data []resSchema, tMilestone int64) (string, []string, error) {
	newestOnMilestone := ""
	secondNewestnewestOnMilestone := ""

	buildMap := make(map[string][]string)
	for _, d := range data {
		if d.Milestone != tMilestone {
			continue
		}

		if newestOnMilestone == "" {
			newestOnMilestone = d.Platform
		} else {
			if buildNewer(d.Platform, newestOnMilestone) {
				newestOnMilestone = d.Platform
			}
		}

		// If the build is not the newest on milestone, check if its the second newest
		if d.Platform != newestOnMilestone {
			if secondNewestnewestOnMilestone == "" {
				secondNewestnewestOnMilestone = d.Platform
			} else {
				if buildNewer(d.Platform, secondNewestnewestOnMilestone) {
					secondNewestnewestOnMilestone = d.Platform
				}
			}
		}

		_, ok := buildMap[d.Platform]
		if !ok {
			buildMap[d.Platform] = []string{d.Board}
		} else {
			buildMap[d.Platform] = append(buildMap[d.Platform], d.Board)
		}
	}

	if len(buildMap[newestOnMilestone]) >= len(buildMap[secondNewestnewestOnMilestone]) {
		return newestOnMilestone, buildMap[newestOnMilestone], nil
	}
	return secondNewestnewestOnMilestone, buildMap[secondNewestnewestOnMilestone], nil

}

func buildNewer(platform string, platform2 string) bool {
	parts := strings.Split(platform, ".")
	parts2 := strings.Split(platform2, ".")
	if len(parts) != 3 || len(parts2) != 3 {
		fmt.Println(platform, platform2)
		panic("not good:")
	}

	p1, _ := strconv.Atoi(parts[0])
	p2, _ := strconv.Atoi(parts2[0])

	m1, _ := strconv.Atoi(parts[1])
	m2, _ := strconv.Atoi(parts2[1])

	f1, _ := strconv.Atoi(parts[2])
	f2, _ := strconv.Atoi(parts2[2])

	// If the major version > other major version, its newer
	if p1 > p2 {
		return true
		// If the second ietems major version >, then its false
	} else if p2 > p1 {
		return false
		// If its equal, we do the same check with the minor versions
	} else if m1 > m2 {
		return true
	} else if m2 > m1 {
		return false
		// and finally the final version check
	} else if f1 > f2 {
		return true
	} else if f2 > f1 {
		return false
	}
	return false

}

func buildHWTarget(boardName string, variant string) *api.SwarmingDefinition {
	dut := &dut_api.Dut{}

	Cros := &dut_api.Dut_ChromeOS{DutModel: &dut_api.DutModel{
		BuildTarget: boardName,
	}}
	dut.DutType = &dut_api.Dut_Chromeos{Chromeos: Cros}
	def := &api.SwarmingDefinition{DutInfo: dut}
	if variant != "" {
		def.Variant = variant
	}

	return def
}

func buildHWTargetOld(boardName string, variant string, provInfo []*api.ProvisionInfo) []*api.SwarmingDefinition {
	dut := &dut_api.Dut{}

	Cros := &dut_api.Dut_ChromeOS{DutModel: &dut_api.DutModel{
		BuildTarget: boardName,
	}}
	dut.DutType = &dut_api.Dut_Chromeos{Chromeos: Cros}
	def := &api.SwarmingDefinition{DutInfo: dut,
		ProvisionInfo: provInfo,
	}
	if variant != "" {
		def.Variant = variant
	}

	f := []*api.SwarmingDefinition{def}
	return f
}

func generateProvisionInfoOld(current []*api.ProvisionInfo, path string) []*api.ProvisionInfo {
	installInfo := &api.InstallRequest{
		ImagePath: &conf.StoragePath{
			Path:     path,
			HostType: conf.StoragePath_GS,
		},
	}

	info := &api.ProvisionInfo{
		Type:           api.ProvisionInfo_CROS,
		InstallRequest: installInfo,
	}
	return append(current, info)
}

func main() {
	err := server.Server(executor, "provision_filter")
	if err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
