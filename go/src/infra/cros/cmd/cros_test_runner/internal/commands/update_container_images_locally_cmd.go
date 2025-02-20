// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/cros_test_runner/data"
)

// UpdateContainerImagesLocallyCmd represents build input validation command.
type UpdateContainerImagesLocallyCmd struct {
	*interfaces.AbstractSingleCmdByNoExecutor
	ExecutorConfig interfaces.ExecutorConfigInterface

	// Const Deps
	UpdateableContainers map[string]struct{}

	// Deps
	ContainerKeysRequestedForUpdate []string
	ContainersAvailable             map[string]*api.ContainerImageInfo
	Chroot                          string

	// Updates
	Containers map[string]*api.ContainerImageInfo
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *UpdateContainerImagesLocallyCmd) ExtractDependencies(ctx context.Context, ski interfaces.StateKeeperInterface) error {
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

func (cmd *UpdateContainerImagesLocallyCmd) extractDepsFromPreLocalTestStateKeeper(ctx context.Context, sk *data.PreLocalTestStateKeeper) error {
	if sk.Args.Chroot == "" {
		return fmt.Errorf("Cmd %q missing dependency: Chroot", cmd.GetCommandType())
	}
	cmd.Chroot = sk.Args.Chroot
	if sk.ContainerImages == nil {
		return fmt.Errorf("Cmd %q missing dependency: ContainerImages", cmd.GetCommandType())
	}
	cmd.ContainersAvailable = sk.ContainerImages
	cmd.ContainerKeysRequestedForUpdate = sk.ContainerKeysRequestedForUpdate

	cmd.UpdateableContainers = map[string]struct{}{
		"cros-test":        {},
		"cros-test-finder": {},
		"cros-dut":         {},
		"cros-provision":   {},
	}

	return nil
}

// Execute executes the command.
func (cmd *UpdateContainerImagesLocallyCmd) Execute(ctx context.Context) error {
	var err error
	step, ctx := build.StartStep(ctx, "Build Containers Locally")
	defer func() { step.End(err) }()

	cmd.Containers = cmd.ContainersAvailable
	for _, containerKey := range cmd.ContainerKeysRequestedForUpdate {
		_, ok := cmd.UpdateableContainers[containerKey]
		if !ok {
			err = fmt.Errorf("Container '%s' is not a supported container for local updates.", containerKey)
			return err
		}
		container, ok := cmd.Containers[containerKey]
		if !ok {
			err = fmt.Errorf("Container '%s' could not be found, aborting local update.", containerKey)
			return err
		}

		imageBase := fmt.Sprintf("%s/%s/%s:%s", container.Repository.Hostname, container.Repository.Project, container.Name, container.Tags[0])
		if err := UpdateContainer(ctx, cmd.Chroot, imageBase, containerKey); err != nil {
			err = fmt.Errorf("Container '%s' failed to update locally, %w", containerKey, err)
			return err
		}
		container.Tags[0] = container.Tags[0] + "_localchange"
	}

	return nil

}

// UpdateStateKeeper updates the state keeper with info from the cmd.
func (cmd *UpdateContainerImagesLocallyCmd) UpdateStateKeeper(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.PreLocalTestStateKeeper:
		err = cmd.updatePreLocalTestStateKeeper(ctx, sk)

	default:
		return fmt.Errorf("StateKeeper '%T' is not supported by cmd type %s.", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during updating for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *UpdateContainerImagesLocallyCmd) updatePreLocalTestStateKeeper(
	ctx context.Context,
	sk *data.PreLocalTestStateKeeper) error {

	sk.ContainerImages = cmd.Containers

	return nil
}

func NewUpdateContainerImagesLocallyCmd() *UpdateContainerImagesLocallyCmd {
	abstractCmd := interfaces.NewAbstractCmd(UpdateContainerImagesLocallyCmdType)
	abstractSingleCmdByNoExecutor := &interfaces.AbstractSingleCmdByNoExecutor{AbstractCmd: abstractCmd}
	return &UpdateContainerImagesLocallyCmd{AbstractSingleCmdByNoExecutor: abstractSingleCmdByNoExecutor}
}

type containerTarget struct {
	Src    string
	Dst    string
	DelDst string
}

// UpdateContainer updates the container locally
func UpdateContainer(ctx context.Context, chroot, imageBase, service string) error {
	targets := []*containerTarget{
		{
			Src:    fmt.Sprintf("%s/usr/bin/%s", chroot, service),
			Dst:    "usr/bin",
			DelDst: fmt.Sprintf("usr/bin/%s", service),
		},
	}

	if err := updateBinary(ctx, chroot, service); err != nil {
		return err
	}

	if err := updateImage(ctx, imageBase, targets, service == "cros-test-finder"); err != nil {
		return err
	}

	return nil
}

// updateBinary emerges the local changes into a binary
func updateBinary(ctx context.Context, chroot, service string) error {
	workon := exec.Command("cros_sdk", "cros-workon", "--host", "start", service)
	workon.Dir = chroot
	if err := workon.Run(); err != nil {
		return fmt.Errorf("Workon failed, %w", err)
	}

	emerge := exec.Command("cros_sdk", "sudo", "emerge", service)
	emerge.Dir = chroot
	if err := emerge.Run(); err != nil {
		return fmt.Errorf("Emerge failed. %w", err)
	}
	return nil
}

// createCleanedImage calls into the container to remove the old binary/state
func createCleanedImage(ctx context.Context, image, tempName string, targets []*containerTarget, sudo bool) error {
	args := []string{"run", "-d", "--name", tempName, image}
	if sudo {
		args = append(args, "sudo")
	}
	args = append(args, "rm", "-r")
	for _, target := range targets {
		args = append(args, target.DelDst)
	}
	// docker run -d --name <name> <image> sudo rm -r [delDst]
	logging.Infof(ctx, "Running: docker %s", args)
	create := exec.Command("docker", args...)
	if err := create.Run(); err != nil {
		return fmt.Errorf("docker run failed for %s, %w", image, err)
	}

	return nil
}

// updateImage ensures there is a clean image, places a binary inside the image, then commits the image with the suffix "_localchange"
func updateImage(ctx context.Context, image string, targets []*containerTarget, sudo bool) error {
	timeName := fmt.Sprint(time.Now().UnixNano())
	createCleanedImage(ctx, image, timeName, targets, sudo)

	for _, target := range targets {
		if err := copyIntoDocker(ctx, target, timeName); err != nil {
			logging.Infof(ctx, "copy failed, will respin, %s", err)
			timeName, err = respinImage(ctx, target, timeName)
			if err != nil {
				return err
			}
			if err := copyIntoDocker(ctx, target, timeName); err != nil {
				return fmt.Errorf("Failed to copy twice, %w", err)
			}
		}
	}

	logging.Infof(ctx, "Copy into container succeeded, committing container.")
	args := []string{"commit", timeName, image + "_localchange"}
	commit := exec.Command("docker", args...)
	if err := commit.Run(); err != nil {
		return fmt.Errorf("Failed to commit, %w", err)
	}

	return nil
}

// copyIntoDocker copies the local binary into the container based on the Dst location
func copyIntoDocker(ctx context.Context, target *containerTarget, tempName string) error {
	args := []string{"cp", target.Src, fmt.Sprintf("%s:%s", tempName, target.Dst)}
	cp := exec.Command("docker", args...)
	if out, err := cp.Output(); err != nil {
		logging.Infof(ctx, "docker cp failed, retrying, %s, %s", err, string(out))
		cp = exec.Command("docker", args...)
		if out2, err := cp.Output(); err != nil {
			return fmt.Errorf("docker cp 2nd try failed, %w, %s", err, string(out2))
		}
	}

	return nil
}

// respinImage commits the failed image then recleans the image in the event of a flake
func respinImage(ctx context.Context, target *containerTarget, tempName string) (string, error) {
	args := []string{"commit", tempName, tempName + "_failedcopy"}
	commit := exec.Command("docker", args...)
	sha, err := commit.Output()
	if err != nil {
		return "", fmt.Errorf("Respin failed, docker commit failed, %w", err)
	}
	timeName := fmt.Sprint(time.Now().UnixNano())
	newArgs := []string{"run", "-d", "--name", timeName, string(sha), "sudo", "rm", "-r", target.DelDst}
	respin := exec.Command("docker", newArgs...)
	if err := respin.Run(); err != nil {
		return "", fmt.Errorf("Failed to respin, %w", err)
	}

	return timeName, nil
}
