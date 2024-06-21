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
	CtpReq     *testapi.CTPRequest
	CredsFile  string
	CTPversion string

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

	cmd.CTPversion = sk.CTPversion
	cmd.CredsFile = sk.DockerKeyFile
	cmd.CtpReq = sk.CtpReq
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

	buildContainerMetadata, err := common.FetchImageData(ctx, board, gcsPath)
	if err != nil {
		logging.Infof(ctx, fmt.Sprintf("failed to fetch container image data from %s, will continue without build containers. err: %s", gcsPath, err))
		step.SetSummaryMarkdown("container metadata download failed: perhaps metadata doesn't exist")
		return errors.Annotate(err, "failed to fetch container image data: ").Err()
	}
	logging.Infof(ctx, "ctpreq:", cmd.CtpReq)
	finalMetadataMap := createContainerImagesInfoMap(ctx, cmd.CtpReq, buildContainerMetadata, cmd.CredsFile, cmd.CTPversion)
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

	ctpFilters := make([]*api.CTPFilter, 0)
	defK := common.MakeDefaultFilters(ctx, cmd.CtpReq.GetSuiteRequest())

	karbonFilters, err := common.ConstructCtpFilters(ctx, defK, finalMetadataMap, cmd.CtpReq.GetKarbonFilters(), build)
	if err != nil {
		logging.Infof(ctx, "Err in karbonFilters.")

		return errors.Annotate(err, "failed to create karbon filters: ").Err()
	}
	logging.Infof(ctx, "Past karbonFilters. %s", karbonFilters)

	ctpFilters = append(ctpFilters, karbonFilters...)

	koffeeFilters, err := common.ConstructCtpFilters(ctx, common.DefaultKoffeeFilterNames, finalMetadataMap, cmd.CtpReq.GetKoffeeFilters(), build)
	if err != nil {
		return errors.Annotate(err, "failed to create koffee filters: ").Err()
	}
	logging.Infof(ctx, "Past koffeeFilters. %s", ctpFilters)

	ctpFilters = append(ctpFilters, koffeeFilters...)

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

func createContainerImagesInfoMap(ctx context.Context, req *testapi.CTPRequest, buildContMetadata map[string]*buildapi.ContainerImageInfo, creds, ctpVersion string) map[string]*buildapi.ContainerImageInfo {
	// In case of any overlap of container metadata between input and build metadata,
	// the input metadata will be prioritized.
	bcm := make(map[string]*buildapi.ContainerImageInfo)
	for k, v := range buildContMetadata {
		bcm[k] = v
	}

	// Add staging/prod containers from the firestore.
	firestoreFilters, err := common.FetchFiltersFromFirestore(ctx, creds, ctpVersion)
	if err != nil {
		logging.Infof(ctx, "%w", err)
	}
	for _, filter := range firestoreFilters {
		bcm[filter.GetContainerInfo().GetContainer().GetName()] = filter.GetContainerInfo().GetContainer()
	}

	// Combine karbon and koffee filters.
	// They use identical logic.
	for _, filter := range append(req.GetKarbonFilters(), req.GetKoffeeFilters()...) {
		container := filter.GetContainerInfo().GetContainer()
		// Overwrite container image info if present within filter input.
		// Else, check if map already contains image info for the filter.
		if hasValidDigest(container.Digest) || len(container.Tags) > 0 {
			bcm[container.GetName()] = container
		} else {
			if _, ok := bcm[container.GetName()]; !ok {
				logging.Infof(ctx, "container %s missing container info", container.GetName())
			}
		}
	}

	return bcm
}

// hasValidDigest ensures the digest starts with `sha256:` and
// the SHA value itself is 64 characters in length.
func hasValidDigest(digest string) bool {
	return strings.HasPrefix(digest, "sha256:") && len(digest) == len("sha256:")+64
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
