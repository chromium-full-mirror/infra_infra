// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Given a cloud project id and a regex for allowed pubsub topics (optional)
// listen to all of them and write out messages to stdout as they exist.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cloud.google.com/go/pubsub"
	"github.com/maruel/subcommands"
	"google.golang.org/api/iterator"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/hardcoded/chromeinfra"
)

func Application() *cli.Application {
	return &cli.Application{
		Name:  "pubsublistener",
		Title: `Listen to PubSub`,
		Context: func(ctx context.Context) context.Context {
			return ctx
		},
		Commands: []*subcommands.Command{
			subcommands.CmdHelp,
			ListenCmd,
		},
	}
}

var ListenCmd = &subcommands.Command{
	UsageLine: "listen",
	ShortDesc: "list pubsub topics",
	LongDesc:  "list all the pubsub topics for a given project",
	CommandRun: func() subcommands.CommandRun {
		c := &listenCmd{}
		c.authFlags.Register(&c.Flags, DefaultAuthOptions)
		return c
	},
}

// DefaultAuthOptions is an auth.Options struct prefilled with chrome-infra
// defaults.
var DefaultAuthOptions = chromeinfra.SetDefaultAuthOptions(auth.Options{
	Scopes:     GetAuthScopes(DefaultAuthScopes),
	SecretsDir: SecretsDir(),
})

// SecretsDir customizes the location for auth-related secrets.
func SecretsDir() string {
	configDir := os.Getenv("XDG_CACHE_HOME")
	if configDir == "" {
		configDir = filepath.Join(os.Getenv("HOME"), ".cache")
	}
	return filepath.Join(configDir, "pubsublistener", "auth")
}

// GetAuthScopes get environment scopes if set
// Otherwise, return default scopes
func GetAuthScopes(defaultScopes []string) []string {
	e := os.Getenv("OAUTH_SCOPES")
	if e != "" {
		return strings.Split(e, "|")
	}
	return defaultScopes
}

// DefaultAuthScopes is the default scopes for shivas login
var DefaultAuthScopes = []string{auth.OAuthScopeEmail, "https://www.googleapis.com/auth/spreadsheets", "https://www.googleapis.com/auth/cloud-platform"}

type listenCmd struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
}

func (c *listenCmd) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

func (c *listenCmd) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	ctx := context.Background()
	for _, project := range args {
		client, err := pubsub.NewClient(ctx, project)
		if err != nil {
			return err
		}
		topics := client.Topics(ctx)
		for {
			topic, err := topics.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return err
			}
			fmt.Fprintf(a.GetOut(), "%s\n", topic)
		}
	}
	return nil
}

func main() {
	os.Exit(subcommands.Run(Application(), nil))
}
