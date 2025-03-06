// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fake

import (
	"context"
	"fmt"
	"io/ioutil"

	"google.golang.org/grpc"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/proto/git"
	"go.chromium.org/luci/common/proto/gitiles"
)

// GitClient mocks the git.ClientInterface
type GitClient struct {
}

// GitTilesClient mocks the gitiles.GitilesClient
type GitTilesClient struct {
}

// GetFile mocks git.ClientInterface.GetFile()
func (gc *GitClient) GetFile(ctx context.Context, path string) (string, error) {
	if path == "test_git_path" {
		return GitData("../frontend/fake/dhcp_test.conf")
	} else if path == "test_enc_git_path" {
		return GitData("../frontend/fake/bots.cfg")
	} else if path == "test_security_git_path" {
		return GitData("../frontend/fake/ufs_security.cfg")
	} else if path == "test_maintenance_git_path" {
		return GitData("../frontend/fake/ufs_maintenance_config.cfg")
	}
	return "", errors.Reason("Unspecified mock path %s", path).Err()
}

// SwitchProject mocks git.ClientInterface.SwitchProject()
func (gc *GitClient) SwitchProject(ctx context.Context, project string) error {
	return nil
}

// Log mocks gitiles.GitilesClient.Log()
func (gc *GitTilesClient) Log(ctx context.Context, req *gitiles.LogRequest, opts ...grpc.CallOption) (res *gitiles.LogResponse, err error) {
	return &gitiles.LogResponse{
		Log: []*git.Commit{
			{Id: fmt.Sprintf("%s-%s", req.Project, req.Committish)},
		},
	}, nil
}

func (gc *GitTilesClient) DownloadFile(ctx context.Context, req *gitiles.DownloadFileRequest, opts ...grpc.CallOption) (*gitiles.DownloadFileResponse, error) {
	if req.Path == "test_device_config" {
		return GitilesData("../frontend/fake/device_config.cfg")
	}
	if req.Path == "test_security_git_path" {
		if req.Committish == "5201756875e0405c5c44d0e6d97de653b0d6cfca" {
			return GitilesData("../frontend/fake/ufs_security.cfg")
		} else {
			return nil, errors.Reason("unknown commitsh %s", req.Committish).Err()
		}
	}

	return nil, errors.Reason("unspecified mock path %s", req.Path).Err()
}

// GitData mocks a git file content based on a given filepath
func GitData(path string) (string, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func GitilesData(path string) (*gitiles.DownloadFileResponse, error) {
	content, err := GitData(path)
	return &gitiles.DownloadFileResponse{
		Contents: content,
	}, err
}
