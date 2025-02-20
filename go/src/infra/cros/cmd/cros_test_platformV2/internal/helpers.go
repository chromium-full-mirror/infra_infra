// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package internal

import (
	"context"
	"errors"
	"fmt"

	"go.chromium.org/chromiumos/config/go/test/api"
	dut_api "go.chromium.org/chromiumos/config/go/test/lab/api"

	managers "infra/cros/cmd/cros_test_platformV2/docker_managers"
	"infra/cros/cmd/cros_test_platformV2/executor"
	"infra/cros/cmd/cros_test_platformV2/tools/gcs"
)

func translateRequest(req *api.CTPv2Request) *api.InternalTestplan {
	internalStruct := &api.InternalTestplan{}
	targs := targetRequirements(req)
	suitemd := &api.SuiteMetadata{
		TargetRequirements: targs,
		Pool:               req.Pool,
	}

	internalStruct.SuiteInfo = &api.SuiteInfo{
		SuiteMetadata: suitemd,
		SuiteRequest:  req.SuiteRequest,
	}

	return internalStruct
}

func legacyswpoper(sws []*api.SWTarget) []*api.LegacySW {
	var legsws []*api.LegacySW
	for _, swTarg := range sws {
		switch sw := swTarg.SwTarget.(type) {
		case *api.SWTarget_LegacySw:
			legsws = append(legsws, sw.LegacySw)
		}
	}
	return legsws
}

func buildcrosDut(hw *api.LegacyHW) *api.SwarmingDefinition {
	dut := &dut_api.Dut{}

	Cros := &dut_api.Dut_ChromeOS{DutModel: &dut_api.DutModel{
		BuildTarget: hw.Board,
		ModelName:   hw.Model,
	}}
	dut.DutType = &dut_api.Dut_Chromeos{Chromeos: Cros}

	return &api.SwarmingDefinition{DutInfo: dut}
}

func targetRequirements(req *api.CTPv2Request) (targs []*api.TargetRequirements) {
	for _, targ := range req.Targets {
		switch hw := targ.HwTarget.Target.(type) {
		case *api.HWTarget_LegacyHw:

			// There will only be one set by the translation; but other filters might
			// expand this as they see fit.
			var hwDefs []*api.SwarmingDefinition
			hwDefs = append(hwDefs, buildcrosDut(hw.LegacyHw))

			legacysw := legacyswpoper(targ.SwTargets)

			builtTarget := &api.TargetRequirements{
				HwRequirements: &api.HWRequirements{
					HwDefinition: hwDefs,
				},
				SwRequirements: legacysw,
			}
			targs = append(targs, builtTarget)
		}
	}
	return targs

}

func createContainerManagerExecutor(ctx context.Context, cloud bool) (managers.ContainerManager, executor.Executor) {
	var containerMgr managers.ContainerManager
	var e executor.Executor
	if cloud {
		containerMgr = managers.NewCloudDummy()
		e = executor.NewCloudContainerExecutor(containerMgr)
	} else {
		containerMgr = managers.NewCtrManager()
		e = executor.NewCtrExecutor(containerMgr)
	}

	return containerMgr, e
}

func getFirstBoardFromLegacy(targs []*api.Targets) string {
	if len(targs) == 0 {
		return ""
	}
	switch hw := targs[0].HwTarget.Target.(type) {
	case *api.HWTarget_LegacyHw:
		return hw.LegacyHw.Board
	default:
		return ""
	}
}

func getFirstGcsPathFromLegacy(targs []*api.Targets) string {
	if len(targs) == 0 {
		return ""
	}
	if len(targs[0].SwTargets) == 0 {
		return ""
	}
	switch sw := targs[0].SwTargets[0].SwTarget.(type) {
	case *api.SWTarget_LegacySw:
		return sw.LegacySw.GcsPath
	default:
		return ""
	}
}

func gcsInfo(req *api.CTPv2Request) (string, string, error) {
	board := getFirstBoardFromLegacy(req.Targets)
	if board == "" {
		return "", "", errors.New("no board provided in legacy request")
	}

	gcsPath := getFirstGcsPathFromLegacy(req.Targets)
	if gcsPath == "" {
		return "", "", errors.New("no gcsPath provided in legacy request")
	}

	return board, gcsPath, nil
}

func buildExecutors(ctx context.Context, req *api.CTPv2Request, cloud bool) ([]executor.Executor, error) {
	execs := []executor.Executor{}

	// TODO currently there is a bit of a race between getting the metadata to run test-finder, and the boards provided
	// to the request. We don't want to run test-finder per board as that's extremely expensive to setup/act on, however
	// CFT design has test-finder being board specific. For initial MVP we will just use the first board in the request to
	// get the container MD from, but this will need to be solved long term.
	board, gcsPath, err := gcsInfo(req)
	if err != nil {
		return nil, err
	}

	containerMetadata, err := gcs.FetchImageData(ctx, board, gcsPath)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch image data: %w", err)
	}

	// First must always be the container Manager. All further executors will require the manager.
	contMang, contExec := createContainerManagerExecutor(ctx, cloud)
	execs = append(execs, contExec)

	defaultKarbon, defaultKoffe, err := executor.DefaultFilters(req, containerMetadata)
	if err != nil {
		return nil, fmt.Errorf("Unable to create default filters: %w", err)
	}

	for _, filter := range defaultKarbon {
		exec, err := executor.NewKarbonExecutor(contMang, filter, containerMetadata)
		if err != nil {
			return nil, err
		}
		execs = append(execs, exec)
	}
	for _, filter := range req.KarbonFilters {
		exec, err := executor.NewKarbonExecutor(contMang, filter, containerMetadata)
		if err != nil {
			return nil, err
		}
		execs = append(execs, exec)
	}

	for _, filter := range defaultKoffe {
		exec, err := executor.NewKoffeeExecutor(contMang, filter, containerMetadata)
		if err != nil {
			return nil, err
		}
		execs = append(execs, exec)
	}
	for _, filter := range req.KoffeeFilters {
		exec, err := executor.NewKoffeeExecutor(contMang, filter, containerMetadata)
		if err != nil {
			return nil, err
		}
		execs = append(execs, exec)
	}
	// Can expand to more as needed.
	return execs, nil
}
