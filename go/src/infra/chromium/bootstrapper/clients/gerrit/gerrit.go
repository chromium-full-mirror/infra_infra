// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gerrit

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/api/gerrit"
	"go.chromium.org/luci/common/errors"
	gerritpb "go.chromium.org/luci/common/proto/gerrit"

	"go.chromium.org/infra/chromium/bootstrapper/clients/gob"
)

type Client struct {
	clients map[string]GerritClient
	factory GerritClientFactory
}

// GerritClient provides a subset of the generated gerrit RPC client.
type GerritClient interface {
	GetChange(ctx context.Context, in *gerritpb.GetChangeRequest, opts ...grpc.CallOption) (*gerritpb.ChangeInfo, error)
}

// Enforce that the GerritClient interface is a subset of the generated client
// interface.
var _ GerritClient = (gerritpb.GerritClient)(nil)

// GerritClientFactory creates clients for accessing each necessary gerrit
// instance.
type GerritClientFactory func(ctx context.Context, host string) (GerritClient, error)

var ctxKey = "infra/chromium/bootstrapper/clients/gerrit.GerritClientFactory"

// UseGerritClientFactory returns a context that causes new Client instances to
// use the given factory when getting gerrit clients.
func UseGerritClientFactory(ctx context.Context, factory GerritClientFactory) context.Context {
	return context.WithValue(ctx, &ctxKey, factory)
}

func NewClient(ctx context.Context) *Client {
	factory, _ := ctx.Value(&ctxKey).(GerritClientFactory)
	if factory == nil {
		factory = func(ctx context.Context, host string) (GerritClient, error) {
			authClient, err := auth.NewAuthenticator(ctx, auth.SilentLogin, auth.Options{Scopes: []string{gerrit.OAuthScope}}).Client()
			if err != nil {
				return nil, fmt.Errorf("could not initialize auth client: %w", err)
			}
			return gerrit.NewRESTClient(authClient, host, true)
		}
	}
	return &Client{
		clients: map[string]GerritClient{},
		factory: factory,
	}
}

func (c *Client) gerritClientForHost(ctx context.Context, host string) (GerritClient, error) {
	if client, ok := c.clients[host]; ok {
		return client, nil
	}
	client, err := c.factory(ctx, host)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, errors.Fmt("returned client for %s is nil", host)
	}
	c.clients[host] = client
	return client, nil
}

type ChangeInfo struct {
	// TargetRef is the ref that the change targets.
	TargetRef string
	// GitilesRevision is the revision in the corresponding gitiles repository containing the
	// commits in the patchset.
	GitilesRevision string
}

func (c *Client) GetChangeInfo(ctx context.Context, host, project string, change int64, patchset int32) (*ChangeInfo, error) {
	gerritClient, err := c.gerritClientForHost(ctx, host)
	if err != nil {
		return nil, err
	}

	info := &ChangeInfo{}
	err = gob.Execute(ctx, "GetChange", func() error {
		changeInfo, err := gerritClient.GetChange(ctx, &gerritpb.GetChangeRequest{
			Project: project,
			Number:  change,
			Options: []gerritpb.QueryOption{gerritpb.QueryOption_ALL_REVISIONS},
		})
		if err != nil {
			return err
		}
		for rev, revInfo := range changeInfo.Revisions {
			if revInfo.Number == patchset {
				info.TargetRef = changeInfo.Ref
				info.GitilesRevision = rev
				return nil
			}
		}
		// Occasionally the returned change information doesn't contain the patchset
		// corresponding to the build's gerrit change, presumably due to replication lag. In
		// that case return an error with codes.NotFound so that it will be retried.
		return status.Error(codes.NotFound, fmt.Sprintf("%s/c/%s/+/%d does not have patchset %d", host, project, change, patchset))
	})
	if err != nil {
		return nil, err
	}

	return info, nil
}
