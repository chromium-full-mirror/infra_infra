// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cmd

import (
	"context"
	"flag"

	"cloud.google.com/go/pubsub"
	"github.com/maruel/subcommands"
	"google.golang.org/api/option"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"

	"go.chromium.org/infra/cros/cmd/ipcpubsub/internal/site"
)

type baseRun struct {
	subcommands.CommandRunBase
	topic     string
	project   string
	authFlags authcli.Flags
}

func (r *baseRun) registerCommonFlags(fs *flag.FlagSet) {
	r.authFlags.Register(fs, site.DefaultAuthOptions)
	fs.StringVar(&r.topic, "topic", "", "Pubsub topic to use")
	fs.StringVar(&r.project, "project", "", "Pubsub project to use")
}

func (r *baseRun) createClient(ctx context.Context) (*pubsub.Client, error) {
	opts, err := r.authFlags.Options()
	if err != nil {
		return nil, err
	}
	authenticator := auth.NewAuthenticator(ctx, auth.SilentLogin, opts)
	ts, err := authenticator.TokenSource()
	if err != nil {
		return nil, err
	}

	client, err := pubsub.NewClient(ctx, r.project, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	return client, nil
}

func bytesMapToList(m map[string][]byte) [][]byte {
	l := make([][]byte, 0, len(m))
	for _, v := range m {
		l = append(l, v)
	}
	return l
}
