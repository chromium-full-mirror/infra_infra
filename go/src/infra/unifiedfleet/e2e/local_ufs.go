// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build linux && (integration || e2e)

// Package e2e_test runs the service via docker compose and executes end to end
// or integration tests.
// It may run multiple instance of docker-compose which each has a individual
// network, so please ensure your docker daemon config file
// (/etc/docker/daemon.json) has enough size of address pool (suggest no less
// than 32 networks.)
package e2e_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/modules/compose"
	"github.com/testcontainers/testcontainers-go/wait"
)

type LocalUFSEnv struct {
	comp      compose.ComposeStack
	shivasCtr *testcontainers.DockerContainer
}

func NewLocalUFSEnv(ctx context.Context) (*LocalUFSEnv, error) {
	composeFilePath := "./docker-compose.yml"
	comp, err := compose.NewDockerCompose(composeFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create compose stack: %w", err)
	}

	uid, gid := getCurrentUserIDs()
	err = comp.
		WaitForService("proxy_and_shivas", wait.ForListeningPort("8800/tcp")).
		WaitForService("ufs-service", wait.ForListeningPort("8800/tcp")).
		WaitForService("dumper", wait.ForListeningPort("8800/tcp")).
		WithEnv(map[string]string{"CURRENT_UID": fmt.Sprintf("%d:%d", uid, gid)}).
		Up(ctx)
	if err != nil {
		_ = comp.Down(ctx, compose.RemoveOrphans(true))
		return nil, fmt.Errorf("failed to start compose stack: %w", err)
	}
	shivas, err := comp.ServiceContainer(ctx, "proxy_and_shivas")
	if err != nil {
		_ = comp.Down(ctx, compose.RemoveOrphans(true))
		return nil, fmt.Errorf("failed to get shivas container: %w", err)
	}
	return &LocalUFSEnv{comp: comp, shivasCtr: shivas}, nil
}

func (u *LocalUFSEnv) Shivas(ctx context.Context, cmd []string, options ...tcexec.ProcessOption) (int, string, error) {
	fullCmd := formatShivasCmd(cmd)
	log.Printf("exec shivas cmd %q w/ opts: %#v", strings.Join(fullCmd, " "), options)
	options = append(options, tcexec.Multiplexed())
	code, reader, err := u.shivasCtr.Exec(ctx, fullCmd, options...)
	output, err := io.ReadAll(reader)
	if err != nil {
		return 0, "", fmt.Errorf("shivas exec: %w", err)
	}
	return code, string(output), nil
}

func (u *LocalUFSEnv) ShivasStdin(ctx context.Context, cmd []string, stdin string, options ...tcexec.ProcessOption) (int, string, error) {
	fullCmd := formatShivasCmd(cmd)
	log.Printf("exec shivas cmd %q w/ opts: %#v", strings.Join(fullCmd, " "), options)

	provider, err := testcontainers.NewDockerProvider()
	if err != nil {
		return 0, "", fmt.Errorf("shivas: failed to get Docker provider: %w", err)
	}
	defer provider.Close()

	processOptions := tcexec.NewProcessOptions(fullCmd)

	// processing all the options in a first loop because for the multiplexed option
	// we first need to have a containerExecCreateResponse
	for _, o := range options {
		o.Apply(processOptions)
	}
	processOptions.ExecConfig.AttachStdin = true

	// cli := dockerProvider.Client()
	cli := provider.Client()
	if cli == nil {
		return 0, "", fmt.Errorf("shivas: docker client is nil")
	}
	c := u.shivasCtr
	response, err := cli.ContainerExecCreate(ctx, c.ID, processOptions.ExecConfig)
	if err != nil {
		return 0, "", fmt.Errorf("container exec create: %w", err)
	}

	hijack, err := cli.ContainerExecAttach(ctx, response.ID, container.ExecAttachOptions{})
	if err != nil {
		return 0, "", fmt.Errorf("container exec attach: %w", err)
	}

	processOptions.Reader = hijack.Reader

	// second loop to process the multiplexed option, as now we have a reader
	// from the created exec response.
	for _, o := range options {
		o.Apply(processOptions)
	}

	go func() {
		_, err := io.WriteString(hijack.Conn, stdin)
		if err != nil {
			log.Printf("failed to write to container stdin: %v", err)
		}
		hijack.CloseWrite()
	}()

	var exitCode int
	for {
		execResp, err := cli.ContainerExecInspect(ctx, response.ID)
		if err != nil {
			return 0, "", fmt.Errorf("container exec inspect: %w", err)
		}

		if !execResp.Running {
			exitCode = execResp.ExitCode
			break
		}

		time.Sleep(100 * time.Millisecond)
	}
	output, err := io.ReadAll(processOptions.Reader)
	if err != nil {
		return 0, "", fmt.Errorf("shivas exec: %w", err)
	}

	return exitCode, string(output), nil
}

func WithNamespace(ns string) tcexec.ProcessOption {
	return tcexec.ProcessOptionFunc(func(opts *tcexec.ProcessOptions) {
		opts.ExecConfig.Env = append(opts.ExecConfig.Env, fmt.Sprintf("SHIVAS_NAMESPACE=%s", ns))
	})
}

var WithBrowser = WithNamespace("browser")

func WithStdin() tcexec.ProcessOption {
	return tcexec.ProcessOptionFunc(func(opts *tcexec.ProcessOptions) {
		opts.ExecConfig.AttachStdin = true
	})
}

func (u *LocalUFSEnv) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := u.comp.Down(ctx, compose.RemoveOrphans(true)); err != nil {
		return fmt.Errorf("failed to tear down compose stack: %v", err)
	}
	return nil
}

func formatShivasCmd(cmd []string) []string {
	return append([]string{"/shivas"}, cmd...)
}

// getCurrentUserIDs returns the current user's UID and GID as integers.
// It fails the test if any errors occur.
func getCurrentUserIDs() (int, int) {

	currentUser, err := user.Current()
	if err != nil {
		log.Fatalf("Error getting current user: %v", err)
	}

	userID, err := strconv.Atoi(currentUser.Uid)
	if err != nil {
		log.Fatalf("Error converting UID to integer: %v", err)
	}

	groupID, err := strconv.Atoi(currentUser.Gid)
	if err != nil {
		log.Fatalf("Error converting GID to integer: %v", err)
	}

	return userID, groupID
}
