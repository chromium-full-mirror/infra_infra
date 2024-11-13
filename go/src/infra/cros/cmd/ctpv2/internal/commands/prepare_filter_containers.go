// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	buildapi "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/ctpv2/data"
)

// PrepareFilterContainersInfoCmd represents prepare filter containers info cmd.
type PrepareFilterContainersInfoCmd struct {
	*interfaces.AbstractSingleCmdByNoExecutor

	// Deps
	CtpReq       *testapi.CTPRequest
	CredsFile    string
	CTPversion   string
	Experiments  []string
	IsAlRun      bool
	IsPartnerRun bool
	// Updates
	ContainerInfoQueue   *list.List
	ContainerMetadataMap map[string]*buildapi.ContainerImageInfo
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *PrepareFilterContainersInfoCmd) ExtractDependencies(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.FilterStateKeeper:
		err = cmd.extractDepsFromFilterStateKeepr(ctx, sk)

	default:
		return fmt.Errorf("StateKeeper '%T' is not supported by cmd type %s.", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during extracting dependencies for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

// UpdateStateKeeper updates the state keeper with info from the cmd.
func (cmd *PrepareFilterContainersInfoCmd) UpdateStateKeeper(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.FilterStateKeeper:
		err = cmd.updateLocalTestStateKeeper(ctx, sk)
	}

	if err != nil {
		return errors.Annotate(err, "error during updating for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *PrepareFilterContainersInfoCmd) extractDepsFromFilterStateKeepr(
	ctx context.Context,
	sk *data.FilterStateKeeper) error {

	if sk.CtpReq == nil {
		return fmt.Errorf("Cmd %q missing dependency: CtpV2Req", cmd.GetCommandType())
	}
	cmd.Experiments = sk.BuildState.Build().Input.Experiments
	cmd.CTPversion = sk.CTPversion
	cmd.CredsFile = sk.DockerKeyFile
	cmd.CtpReq = sk.CtpReq
	cmd.IsAlRun = sk.IsAlRun
	cmd.IsPartnerRun = sk.IsPartnerRun
	return nil
}

func (cmd *PrepareFilterContainersInfoCmd) updateLocalTestStateKeeper(
	ctx context.Context,
	sk *data.FilterStateKeeper) error {

	if cmd.ContainerInfoQueue != nil {
		sk.ContainerInfoQueue = cmd.ContainerInfoQueue
	}

	if cmd.ContainerMetadataMap != nil {
		sk.ContainerMetadataMap = cmd.ContainerMetadataMap
	}

	return nil
}

func getBuildFromGCSPath(gcsPath string) int {
	g := strings.Split(gcsPath, "/")
	R := g[len(g)-1]
	Major := strings.Split(R, "-")
	RN := Major[1]

	build, _ := strconv.Atoi(strings.Split(RN, ".")[0])
	return build
}

// Execute executes the command.
func (cmd *PrepareFilterContainersInfoCmd) Execute(ctx context.Context) error {
	var err error
	step, ctx := build.StartStep(ctx, "Prepare containers for filters")
	defer func() { step.End(err) }()

	// -- Create final container metadata map --

	// TODO (dbeckett): currently there is a bit of a race between getting the metadata to run test-finder, and the boards provided
	// to the request. We don't want to run test-finder per board as that's extremely expensive to setup/act on, however
	// CFT design has test-finder being board specific. For initial MVP we will just use the first board in the request to
	// get the container MD from, but this will need to be solved long term.
	board, gcsPath, err := gcsInfo(cmd.CtpReq)
	if err != nil {
		return err
	}

	build := getBuildFromGCSPath(gcsPath)

	var buildContainerMetadata map[string]*buildapi.ContainerImageInfo
	if cmd.IsAlRun {
		step.SetSummaryMarkdown("skipping; AL run doesn't require building container metadata")
	} else {
		buildContainerMetadata, err = common.FetchImageData(ctx, board, gcsPath)
		if err != nil {
			logging.Infof(ctx, fmt.Sprintf("failed to fetch container image data from %s, will continue without build containers. err: %s", gcsPath, err))
			step.SetSummaryMarkdown("container metadata download failed: perhaps metadata doesn't exist")
			return errors.Annotate(err, "failed to fetch container image data: ").Err()
		}
	}

	logging.Infof(ctx, "ctpreq:", cmd.CtpReq)

	// Grab correct firestore db name to be used
	firestoreDBName := common.TestPlatformFireStore
	if cmd.IsAlRun && cmd.IsPartnerRun {
		firestoreDBName = common.PartnerTestPlatformFireStore
	}

	defK := common.MakeDefaultFilters(ctx, cmd.CtpReq.GetSuiteRequest(), cmd.Experiments)
	finalMetadataMap := createContainerImagesInfoMap(ctx, cmd.CtpReq, buildContainerMetadata, cmd.CredsFile, cmd.CTPversion, defK, build, firestoreDBName)
	logging.Infof(ctx, "FINALMAP:", finalMetadataMap)

	cmd.ContainerMetadataMap = finalMetadataMap

	// Write to log
	mapData, err := json.MarshalIndent(finalMetadataMap, "", "\t")
	if err != nil {
		logging.Infof(
			ctx,
			"error during writing container metadata map to log: %s",
			err.Error())
	}
	step.Log("Final container metadata map").Write(mapData)

	// -- Create ctp filters from default and input filters --

	ctpFilters, err := common.ConstructCtpFilters(ctx, defK, finalMetadataMap, append(cmd.CtpReq.GetKarbonFilters(), cmd.CtpReq.GetKoffeeFilters()...), build)
	if err != nil {
		logging.Infof(ctx, "Err in ctpFilters.")

		return errors.Annotate(err, "failed to create filters: ").Err()
	}

	if cmd.IsAlRun && cmd.IsPartnerRun {
		for _, filter := range ctpFilters {
			// TODO (cdelagarza): remove this custom filter check once they are upreved
			if filter.GetContainerInfo().GetContainer().GetName() != "cros-test-finder" && filter.GetContainerInfo().GetContainer().GetName() != "pre-process-filter" {
				filter.GetContainerInfo().BinaryArgs = append(filter.GetContainerInfo().GetBinaryArgs(), "-firestore", firestoreDBName)
			}
		}
	}

	filterData, err := json.MarshalIndent(ctpFilters, "", "\t")
	if err != nil {
		logging.Infof(
			ctx,
			"error during writing ctp filters to log: %s",
			err.Error())
	}
	step.Log("Final Ctp filters list").Write(filterData)

	step.Log("CTPv2 Build").Write([]byte(fmt.Sprintf("%v", build)))
	// -- Create container info queue --

	containerInfoList := list.New()

	for _, filter := range ctpFilters {
		containerInfoList.PushBack(CtpFilterToContainerInfo(filter, build))
	}
	step.Log("Container Info queue").Write(common.ListToJson(containerInfoList))

	cmd.ContainerInfoQueue = containerInfoList

	return nil
}

func NewPrepareFilterContainersInfoCmd() *PrepareFilterContainersInfoCmd {
	abstractCmd := interfaces.NewAbstractCmd(PrepareFilterContainersCmdType)
	abstractSingleCmdByNoExecutor := &interfaces.AbstractSingleCmdByNoExecutor{AbstractCmd: abstractCmd}
	return &PrepareFilterContainersInfoCmd{AbstractSingleCmdByNoExecutor: abstractSingleCmdByNoExecutor}
}

func gcsInfo(req *testapi.CTPRequest) (string, string, error) {
	board := getFirstBoardFromLegacy(req.GetScheduleTargets())
	if board == "" {
		return "", "", errors.New("no board provided in legacy request")
	}

	gcsPath := getFirstGcsPathFromLegacy(req.GetScheduleTargets())
	if gcsPath == "" {
		return "", "", errors.New("no gcsPath provided in legacy request")
	}

	return board, gcsPath, nil
}

func getFirstBoardFromLegacy(targs []*testapi.ScheduleTargets) string {
	board := ""
	variant := ""

	if len(targs) == 0 || len(targs[0].GetTargets()) == 0 {
		return board
	}

	// TODO (azrahman): add support for multi-dut.
	currentTarg := targs[0].GetTargets()[0]

	switch hw := currentTarg.HwTarget.Target.(type) {
	case *testapi.HWTarget_LegacyHw:
		board = hw.LegacyHw.Board
		variant = hw.LegacyHw.GetVariant()
	}

	if variant != "" {
		return fmt.Sprintf("%s-%s", board, variant)
	}

	return board
}

func getFirstGcsPathFromLegacy(schedTargs []*testapi.ScheduleTargets) string {
	targs := schedTargs[0].GetTargets()
	if len(targs) == 0 {
		return ""
	}

	switch sw := targs[0].SwTarget.SwTarget.(type) {
	case *testapi.SWTarget_LegacySw:
		return sw.LegacySw.GetGcsPath()
	default:
		return ""
	}
}

func createContainerImagesInfoMap(
	ctx context.Context,
	req *testapi.CTPRequest,
	buildContMetadata map[string]*buildapi.ContainerImageInfo,
	creds, ctpVersion string,
	defaultFilterNames []string,
	build int,
	firestoreDBName string) (bcm map[string]*buildapi.ContainerImageInfo) {
	// In case of any overlap of container metadata between input and build metadata,
	// the input metadata will be prioritized.
	bcm = make(map[string]*buildapi.ContainerImageInfo)
	for k, v := range buildContMetadata {
		bcm[k] = v
	}

	// Write in the default filter's container image info.
	// Overwrite any of the build's metadata.
	defaultFilters := common.GetDefaultFilterContainerImageInfosMap(ctx, creds, ctpVersion, defaultFilterNames, buildContMetadata, build, firestoreDBName)
	for defaultFilterName, defaultFilter := range defaultFilters {
		bcm[defaultFilterName] = defaultFilter
	}

	for _, filter := range req.GetKarbonFilters() {
		container := filter.GetContainerInfo().GetContainer()
		filterName := container.GetName()
		bcm[filterName] = container
	}

	return
}

// CtpFilterToContainerInfo creates container info from provided ctp filter.
func CtpFilterToContainerInfo(ctpFilter *api.CTPFilter, build int) *data.ContainerInfo {
	contName := ctpFilter.GetContainerInfo().GetContainer().GetName()
	// TODO (azrahman): remove this once container creation is more generic.
	if contName == common.TtcpContainerName {
		return &data.ContainerInfo{
			ImageKey:  contName,
			Request:   common.CreateTTCPContainerRequest(ctpFilter),
			ImageInfo: ctpFilter.GetContainerInfo().GetContainer(),
		}
	} else {
		return &data.ContainerInfo{
			ImageKey:  contName,
			Request:   common.CreateContainerRequest(ctpFilter, build),
			ImageInfo: ctpFilter.GetContainerInfo().GetContainer(),
		}
	}
}
