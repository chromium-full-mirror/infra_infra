// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package experimental

import (
	"context"
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/maruel/subcommands"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"

	"go.chromium.org/infra/cmd/shivas/site"
	"go.chromium.org/infra/cmd/shivas/utils"
)

// ChangelogCmd lists the changes made to a particular entity
var ChangelogCmd = &subcommands.Command{
	UsageLine: "changelog",
	ShortDesc: "list changelog for any entity",
	LongDesc: `List the changelog associated with the given key
	./shivas changelog -key <dut-name> -limit 100
	./shivas changelog -key <asset-tag> -limit 1000
	./shivas changelog -key <asset-tag>`,
	CommandRun: func() subcommands.CommandRun {
		c := &changelogRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.Flags.StringVar(&c.key, "key", "", "a key to query")
		c.Flags.IntVar(&c.limit, "limit", 100, "limit the number of entries. 0 lists everything")
		return c
	},
}

type changelogRun struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags

	key   string
	limit int
}

func (c *changelogRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

func (c *changelogRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) (err error) {
	ctx := cli.GetContext(a, c, env)

	if c.key == "" {
		return fmt.Errorf("Must specify a key for listing")
	}

	authOpts, err := c.authFlags.Options()
	if err != nil {
		return err
	}

	cl, err := auth.NewAuthenticator(ctx, auth.SilentLogin, authOpts).Client()
	if err != nil {
		return err
	}
	client, err := bigquery.NewClient(ctx, "unified-fleet-system", option.WithHTTPClient(cl))
	if err != nil {
		return err
	}

	logs, err := queryChangelog(ctx, client, c.key, c.limit)
	if err != nil {
		return err
	}
	utils.PrettyPrintListOfStruct(logs)
	return nil
}

// queryChangelog does a BQ query and returns the list of changelogs or error on any errors
func queryChangelog(ctx context.Context, client *bigquery.Client, key string, limit int) ([]*changelog, error) {
	query := `SELECT change_event.name AS Name, change_event.event_label AS Label,
	change_event.new_value AS NewVal, change_event.old_value AS OldVal,
	change_event.comment AS Comment, change_event.update_time AS Stamp,
	change_event.user_email AS Email FROM ` + "`unified-fleet-system.ufs.change_events` " +
		`WHERE change_event.name LIKE "%` + key + `%" ORDER BY change_event.update_time DESC`
	if limit != 0 {
		// Add the correct limit
		query = query + fmt.Sprintf(" LIMIT %d", limit)
	}
	q := client.Query(query)
	it, err := q.Read(ctx)
	if err != nil {
		fmt.Println(q)
		return nil, fmt.Errorf("Failed to query BQ. %w", err)
	}
	log := make([]*changelog, 0, limit)
	for {
		var i changelog
		err := it.Next(&i)
		if err == iterator.Done {
			break
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		log = append(log, &i)
	}
	return log, nil

}

// changelog is a row structure for changelog
type changelog struct {
	Name    string
	Label   string
	NewVal  string
	OldVal  string
	Comment string
	Email   string
	Stamp   time.Time
}
