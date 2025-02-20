// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/golang/protobuf/jsonpb"

	"go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
)

// FetchContainerMetadataCmd represents build input validation command.
type FetchContainerMetadataCmd struct {
	*interfaces.AbstractSingleCmdByNoExecutor
	ExecutorConfig interfaces.ExecutorConfigInterface

	// Deps (optional)
	Board  string
	Bucket string
	Number string

	// Updates
	Containers map[string]*api.ContainerImageInfo
	ImagePath  string
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *FetchContainerMetadataCmd) ExtractDependencies(ctx context.Context, ski interfaces.StateKeeperInterface) error {
	var err error
	switch sk := ski.(type) {
	case *data.PreLocalTestStateKeeper:
		err = cmd.extractDepsFromPreLocalTestStateKeeper(ctx, sk)

	default:
		return fmt.Errorf("StateKeeper '%T' is not supported by cmd type %s.", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during extracting dependencies for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *FetchContainerMetadataCmd) extractDepsFromPreLocalTestStateKeeper(ctx context.Context, sk *data.PreLocalTestStateKeeper) error {
	if sk.Args.BuildBoard == "" {
		logging.Infof(ctx, "Warning: cmd %q missing non-critical dependency: BuildBoard", cmd.GetCommandType())
	}
	if sk.Args.BuildBucket == "" {
		logging.Infof(ctx, "Warning: cmd %q missing non-critical dependency: BuildBucket", cmd.GetCommandType())
	}
	if sk.Args.BuildNumber == "" {
		logging.Infof(ctx, "Warning: cmd %q missing non-critical dependency: BuildNumber", cmd.GetCommandType())
	}
	cmd.Board = sk.Args.BuildBoard
	cmd.Bucket = sk.Args.BuildBucket
	cmd.Number = sk.Args.BuildNumber

	return nil
}

// Execute executes the command.
func (cmd *FetchContainerMetadataCmd) Execute(ctx context.Context) error {
	var err error
	step, ctx := build.StartStep(ctx, "Fetch containers metadata from GS")
	defer func() { step.End(err) }()

	if cmd.Board != "" && cmd.Bucket != "" && cmd.Number != "" {
		err = cmd.fetchImageData(ctx)
	}

	return err

}

// UpdateStateKeeper updates the state keeper with info from the cmd.
func (cmd *FetchContainerMetadataCmd) UpdateStateKeeper(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.PreLocalTestStateKeeper:
		err = cmd.updatePreLocalTestStateKeeper(ctx, sk)
	}

	if err != nil {
		return errors.Annotate(err, "error during updating for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *FetchContainerMetadataCmd) updatePreLocalTestStateKeeper(
	ctx context.Context,
	sk *data.PreLocalTestStateKeeper) error {

	if cmd.Containers != nil {
		sk.ContainerImages = cmd.Containers
	}
	if cmd.ImagePath != "" {
		sk.ImagePath = cmd.ImagePath
	}

	return nil
}

func NewFetchContainerMetadataCmd() *FetchContainerMetadataCmd {
	abstractCmd := interfaces.NewAbstractCmd(FetchContainerMetadataCmdType)
	abstractSingleCmdByNoExecutor := &interfaces.AbstractSingleCmdByNoExecutor{AbstractCmd: abstractCmd}
	return &FetchContainerMetadataCmd{AbstractSingleCmdByNoExecutor: abstractSingleCmdByNoExecutor}
}

func (cmd *FetchContainerMetadataCmd) fetchImageData(ctx context.Context) error {
	image_data_template := "gs://chromeos-image-archive/%s-%s/%s*/metadata/containers.jsonpb"
	template := fmt.Sprintf(
		image_data_template,
		cmd.Board,
		cmd.Bucket,
		cmd.Number,
	)

	gsutil := exec.CommandContext(ctx, "gsutil", "ls", "-l", template)
	sort := exec.CommandContext(ctx, "sort", "-k", "2")

	gPipe, err := gsutil.StdoutPipe()
	if err != nil {
		return err
	}

	sort.Stdin = gPipe

	if err := gsutil.Start(); err != nil {
		return err
	}
	imageDataRaw, err := sort.Output()
	if err != nil {
		return err
	}

	regContainerEx := regexp.MustCompile(`gs://.*.jsonpb`)
	containerImages := regContainerEx.FindAllStringSubmatch(string(imageDataRaw), -1)

	if len(containerImages) == 0 {
		return fmt.Errorf("Could not find any container images with given build %s-%s/%s", cmd.Board, cmd.Bucket, cmd.Number)
	}
	archivePath := containerImages[len(containerImages)-1][0]
	cmd.ImagePath = strings.Split(archivePath, "metadata")[0]

	cat := exec.CommandContext(ctx, "gsutil", "cat", archivePath)

	catOut, err := cat.Output()
	if err != nil {
		return err
	}

	reader := strings.NewReader(string(catOut))

	metadata := &api.ContainerMetadata{}
	unmarshaler := jsonpb.Unmarshaler{}
	unmarshaler.Unmarshal(reader, metadata)
	images := metadata.Containers[cmd.Board].Images

	cmd.Containers = make(map[string]*api.ContainerImageInfo)
	for _, image := range images {
		cmd.Containers[image.Name] = &api.ContainerImageInfo{
			Digest:     image.Digest,
			Repository: image.Repository,
			Name:       image.Name,
			Tags:       image.Tags,
		}
	}

	return nil
}
