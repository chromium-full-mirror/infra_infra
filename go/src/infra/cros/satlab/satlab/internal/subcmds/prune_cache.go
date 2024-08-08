// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subcmds

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/maruel/subcommands"

	"infra/cros/satlab/common/utils/misc"
)

const containerVolume = "default_cache"
const imageName = "us-docker.pkg.dev/chromeos-partner-moblab/satlab/nginx:stable"

var PruneCmd = &subcommands.Command{
	UsageLine: `prune_cache`,
	ShortDesc: `Clears cache memory downloaded by the Artifact Downloader`,
	CommandRun: func() subcommands.CommandRun {
		c := &prune_base{}
		return c
	},
}

// prune_base is a placeholder for prune_cache command.
type prune_base struct {
	subcommands.CommandRunBase
}

// Cleaning cache memory for Artifact downloader.
func cleanVolume() error {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return err
	}

	// Shell command to clean folder contents.
	command := []string{"sh", "-c", "rm -rf /_data/*"}

	// Pull nginx image.
	if _, err := cli.ImagePull(context.Background(), imageName, types.ImagePullOptions{}); err != nil {
		return err
	}

	// Creating a container and binding it to data folder present in default_cache.
	resp, err := cli.ContainerCreate(context.Background(),
		&container.Config{
			Image: imageName,
			Cmd:   command,
		},
		&container.HostConfig{
			Binds: []string{fmt.Sprintf("%s:/_data", containerVolume)},
		},
		nil, nil, "",
	)
	if err != nil {
		return err
	}

	// Running a created container.
	if err := cli.ContainerStart(context.Background(), resp.ID, types.ContainerStartOptions{}); err != nil {
		return err
	}

	// Waiting for the process to complete.
	statusCh, errCh := cli.ContainerWait(context.Background(), resp.ID,
		container.WaitConditionNotRunning,
	)
	select {
	case <-statusCh:
	case err := <-errCh:
		if err != nil {
			return err
		}
	}

	// Removing the container.
	if err := cli.ContainerRemove(context.Background(), resp.ID, types.ContainerRemoveOptions{}); err != nil {
		return err
	}
	return nil
}

// Run runs the command, asks for a confirmation prints the message and returns an exit status.
func (c *prune_base) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	r, err := misc.AskConfirmation("This will clean up default_cache, do you want to continue?")
	if err != nil {
		fmt.Printf("Error occurred while reading input: %s.\n", err)
		return 1
	}
	if r {
		if err := cleanVolume(); err != nil {
			fmt.Printf("Error creating Docker Client: %s.\n", err)
			return 1
		}
	}
	return 0
}
