// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package satlabcommands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/timestamp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/satlab/common/paths"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
	"go.chromium.org/infra/cros/satlab/common/utils/misc"
	multiCmdExcutor "go.chromium.org/infra/cros/satlab/satlabrpcserver/utils/executor"
)

// Decision is a classification of a line in a file.
// Lines may be kept, modified, or deleted.
// Functions that process lines of text are split conceptually
// into a decision which classifies lines and a transformation
// which only applies to selected lines.
type Decision int

const (
	Unknown Decision = iota
	Keep
	Reject
	// Modify is used only by replacing things.
	Modify
)

// OSVersion contains the OS version information
// from the command `get_host_os_version`
type OSVersion struct {
	// Version the ChromeOS verison
	Version     string
	Track       string
	Description string
}

// GetHostIdentifier gets the host identifier value.
//
// Note that this command always returns the identifier in lowercase.
func GetDockerHostBoxIdentifier(ctx context.Context, executor executor.IExecCommander) (string, error) {
	out, err := executor.CombinedOutput(exec.CommandContext(ctx, paths.GetHostIdentifierScript))
	// Immediately normalize the satlab prefix to lowercase. It will save a lot of
	// trouble later.
	return strings.ToLower(misc.TrimOutput(out)), errors.Annotate(err, "get host identifier").Err()
}

// parseOutput parse the raw data "<value>\\n"
// removing the `\"` and `\\n`
func parseOutput(s string) string {
	return strings.Trim(s, "\n \"")
}

// GetSatlabVersion gets the Satlab version from docker container `compose` label.
func GetSatlabVersion(ctx context.Context, executor executor.IExecCommander) (string, error) {
	// We want to get the env variables from `docker inspect`
	// The output is like this
	// ```
	// KEY=VALUE
	// KEY=VALUE
	// ...
	// ```
	out, err := executor.CombinedOutput(
		exec.CommandContext(
			ctx,
			paths.DockerPath,
			"inspect",
			"--format={{range .Config.Env}}{{println .}}{{end}}",
			"compose",
		),
	)
	if err != nil {
		return "", nil
	}
	rawData := strings.Fields(string(out))
	for _, row := range rawData {
		if r, ok := strings.CutPrefix(row, "BUILD_VERSION="); ok {
			return r, nil
		}
	}
	return "", errors.New("can't find the version")
}

// GetOsVersion gets the OS GetOsVersion
func GetOsVersion(ctx context.Context, executor executor.IExecCommander) (*OSVersion, error) {
	out, err := executor.CombinedOutput(exec.CommandContext(ctx, paths.GetOSVersionScript))
	if err != nil {
		return nil, err
	}
	rawData := strings.Split(string(out), "\n")
	resp := OSVersion{}
	for _, r := range rawData {
		row := strings.Split(r, ":")
		if len(row) == 2 {
			if strings.ToLower(row[0]) == "version" {
				resp.Version = parseOutput(row[1])
			}
			if strings.ToLower(row[0]) == "track" {
				resp.Track = parseOutput(row[1])
			}
			if strings.ToLower(row[0]) == "description" {
				resp.Description = parseOutput(row[1])
			}
		}
	}
	return &resp, nil
}

// GetHostIP gets the host ip.
func GetHostIP(ctx context.Context, executor executor.IExecCommander) (string, error) {
	out, err := executor.CombinedOutput(exec.CommandContext(ctx, paths.GetHostIPScript))
	if err != nil {
		return "", err
	}
	return parseOutput(string(out)), nil
}

// GetMacAddress gets hostname and mac address of satlab.
func GetMacAddress(ctx context.Context, executor executor.IExecCommander) (string, error) {
	if _, err := os.Stat(paths.GetHostMACScript); errors.Is(err, os.ErrNotExist) {
		// TODO: this if statement can be removed once we fully roll out crrev.com/c/6072857
		hostIP, err := GetHostIP(ctx, executor)
		if err != nil {
			return "", err
		}

		multipleCmdsExecutor := multiCmdExcutor.New(
			exec.CommandContext(
				ctx,
				paths.DockerPath,
				"exec",
				"dhcp",
				"ip",
				"route",
				"show",
			),
			exec.CommandContext(ctx, paths.Grep, hostIP),
		)
		hostIPInfo, err := multipleCmdsExecutor.Exec(executor)
		if err != nil {
			return "", err
		}

		hostIPInfoArr := strings.Split(string(hostIPInfo), " ")
		if len(hostIPInfoArr) < 3 {
			return "", errors.New("Can not get network interface control name.")
		}

		NICIndex := 2
		NICName := hostIPInfoArr[NICIndex]

		cmd := fmt.Sprintf(paths.NetInfoPathTemplate, NICName)
		out, err := executor.CombinedOutput(
			exec.CommandContext(
				ctx,
				paths.DockerPath,
				"exec",
				"dhcp",
				"cat",
				cmd,
			),
		)
		if err != nil {
			return "", err
		}
		return parseOutput(string(out)), nil
	}

	out, err := executor.CombinedOutput(exec.CommandContext(ctx, paths.GetHostMACScript))
	if err != nil {
		return "", err
	}
	return parseOutput(string(out)), nil
}

// GetSatlabStartTime gets start time and uptime of satlab.
func GetSatlabStartTime(ctx context.Context, executor executor.IExecCommander) (*timestamp.Timestamp, error) {
	out, err := executor.CombinedOutput(
		exec.CommandContext(
			ctx,
			paths.DockerPath,
			"inspect",
			"--format='{{.State.StartedAt}}'",
			"compose",
		),
	)
	if err != nil {
		return nil, err
	}

	timeObj, err := time.Parse(time.RFC3339Nano, strings.Trim(string(out), "\n \"'"))
	if err != nil {
		return nil, err
	}

	return timestamppb.New(timeObj), nil
}

// IsUpdateAvailable checks if new versions of satlab is available.
func IsUpdateAvailable(ctx context.Context, executor executor.IExecCommander) (bool, error) {
	out, err := executor.CombinedOutput(exec.CommandContext(ctx, paths.IsUpdateAvailablePath))
	if err != nil {
		return false, errors.Annotate(err, "get is update available").Err()
	}
	return strconv.ParseBool(misc.TrimOutput(out))
}
