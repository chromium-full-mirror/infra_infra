// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gitiles

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/api/gitiles"
	"go.chromium.org/luci/common/errors"
	gitilespb "go.chromium.org/luci/common/proto/gitiles"

	"infra/chromium/bootstrapper/clients/gob"
)

// Client provides the gitiles-oriented operations required for bootstrapping.
type Client struct {
	clients map[string]GitilesClient
	factory GitilesClientFactory
}

// GitilesClient provides a subset of the generated gitiles RPC client.
type GitilesClient interface {
	Log(context.Context, *gitilespb.LogRequest, ...grpc.CallOption) (*gitilespb.LogResponse, error)
	DownloadFile(context.Context, *gitilespb.DownloadFileRequest, ...grpc.CallOption) (*gitilespb.DownloadFileResponse, error)
	DownloadDiff(context.Context, *gitilespb.DownloadDiffRequest, ...grpc.CallOption) (*gitilespb.DownloadDiffResponse, error)
}

// Enforce that the GitilesClient interface is a subset of the generated client
// interface.
var _ GitilesClient = (gitilespb.GitilesClient)(nil)

// GitilesClientFactory creates clients for accessing each necessary gitiles
// instance.
type GitilesClientFactory func(ctx context.Context, host string) (GitilesClient, error)

var ctxKey = "infra/chromium/bootstrapper/clients/gitiles.GitilesClientFactory"

// UseGitilesClientFactory returns a context that causes new Client instances to
// use the given factory when getting gitiles clients.
func UseGitilesClientFactory(ctx context.Context, factory GitilesClientFactory) context.Context {
	return context.WithValue(ctx, &ctxKey, factory)
}

// NewClient returns a new gitiles client.
//
// If ctx is a context returned from UseGitilesClientFactory, then the returned
// client will use the factory that was passed to UseGitilesClientFactory when
// creating gitiles clients. Otherwise, a factory that creates gitiles clients
// using gitiles.NewRESTClient and http.DefaultClient will be used.
func NewClient(ctx context.Context) *Client {
	factory, _ := ctx.Value(&ctxKey).(GitilesClientFactory)
	if factory == nil {
		factory = func(ctx context.Context, host string) (GitilesClient, error) {
			authClient, err := auth.NewAuthenticator(ctx, auth.SilentLogin, auth.Options{Scopes: []string{gitiles.OAuthScope}}).Client()
			if err != nil {
				return nil, fmt.Errorf("could not initialize auth client: %w", err)
			}
			return gitiles.NewRESTClient(authClient, host, true)
		}
	}
	return &Client{map[string]GitilesClient{}, factory}
}

func (c *Client) gitilesClientForHost(ctx context.Context, host string) (GitilesClient, error) {
	if client, ok := c.clients[host]; ok {
		return client, nil
	}
	client, err := c.factory(ctx, host)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, errors.Reason("returned client for %s is nil", host).Err()
	}
	c.clients[host] = client
	return client, nil
}

// FetchLatestRevision returns the git commit hash for the latest change to the
// given ref of the given project on the given host.
func (c *Client) FetchLatestRevision(ctx context.Context, host, project, ref string) (string, error) {
	return c.FetchLatestRevisionForPath(ctx, host, project, ref, "")
}

// FetchLatestRevisionForPath returns the git commit hash for the latest change to the given path on
// the given ref of the given project on the given host.
func (c *Client) FetchLatestRevisionForPath(ctx context.Context, host, project, ref, path string) (string, error) {
	gitilesClient, err := c.gitilesClientForHost(ctx, host)
	if err != nil {
		return "", err
	}
	request := &gitilespb.LogRequest{
		Project:    project,
		Committish: ref,
		PageSize:   1,
		Path:       path,
	}

	var response *gitilespb.LogResponse
	err = gob.Execute(ctx, "Log", func() error {
		var err error
		response, err = gitilesClient.Log(ctx, request)
		return err
	})
	if err != nil {
		return "", err
	}

	return response.Log[0].GetId(), nil
}

// GetParentRevision returns the git commit hash for the parent revision of a given commt of the
// given project on the given host.
func (c *Client) GetParentRevision(ctx context.Context, host, project, revision string) (string, error) {
	gitilesClient, err := c.gitilesClientForHost(ctx, host)
	if err != nil {
		return "", err
	}
	request := &gitilespb.LogRequest{
		Project:    project,
		Committish: revision,
		PageSize:   2,
	}

	var response *gitilespb.LogResponse
	err = gob.Execute(ctx, "Log", func() error {
		var err error
		response, err = gitilesClient.Log(ctx, request)
		return err
	})
	if err != nil {
		return "", err
	}

	return response.Log[1].GetId(), nil
}

// DownloadFile returns the contents of the file at the given path at the given
// revision of the given project on the given host.
func (c *Client) DownloadFile(ctx context.Context, host, project, revision, path string) (string, error) {
	gitilesClient, err := c.gitilesClientForHost(ctx, host)
	if err != nil {
		return "", err
	}
	request := &gitilespb.DownloadFileRequest{
		Project:    project,
		Committish: revision,
		Path:       path,
	}

	var response *gitilespb.DownloadFileResponse
	err = gob.Execute(ctx, "DownloadFile", func() error {
		var err error
		response, err = gitilesClient.DownloadFile(ctx, request)
		return err
	})
	if err != nil {
		return "", err
	}

	return response.Contents, nil
}

// PARENT can be passed as the base argument to DownloadDiff to take the diff between a revision and
// its parent.
const PARENT = ""

// DownloadDiff returns the diff between a given revision and its parent.
func (c *Client) DownloadDiff(ctx context.Context, host, project, revision, base, path string) (string, error) {
	gitilesClient, err := c.gitilesClientForHost(ctx, host)
	if err != nil {
		return "", err
	}
	request := &gitilespb.DownloadDiffRequest{
		Project:    project,
		Committish: revision,
		Base:       base,
		Path:       path,
	}
	var response *gitilespb.DownloadDiffResponse
	err = gob.Execute(ctx, "DownloadDiff", func() error {
		var err error
		response, err = gitilesClient.DownloadDiff(ctx, request)
		return err
	})
	if err != nil {
		return "", err
	}
	return response.Contents, nil
}
