// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cli defines different commands for the test_plan tool.
package cli

import (
	"context"
	"errors"

	"cloud.google.com/go/bigquery"
	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/data/text"

	"go.chromium.org/infra/cros/internal/testplan/coveragerules"
)

func CmdChromeosCoverageRulesUpdateRun(authOpts auth.Options) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "chromeos-coverage-rules-update -table project.dataset.table -generateddir ./generated",
		ShortDesc: text.Doc(`
			Reads all of the generated CoverageRule jsonprotos in a directory
			and uploads them to a BigQuery table.
		`),
		LongDesc: text.Doc(`
			Reads all of the generated CoverageRule jsonprotos in a directory
			and uploads them to a BigQuery table.

			Each CoverageRule is converted to a CoverageRuleBqRow proto for
			upload to BigQuery.  The currently generated CoverageRules in the local
			checkout are used, i.e. any local changes will be reflected in the
			uploaded rows.

			Note that previous rows for a given directory are not deleted, but
			each execution uses the same partition_time field in every row.
			Thus, queries will often want to use a view of the table that only
			shows rows from the most recent partition.
		`),
		CommandRun: func() subcommands.CommandRun {
			r := &chromeosCoverageRulesUpdateRun{}
			r.addSharedFlags(authOpts)
			r.addBigQueryFlags()

			r.Flags.StringVar(&r.generatedDir,
				"generateddir",
				"",
				text.Doc(`
					Directory containing lists of CoverageRules to upload. Every
					file in the dir is assumed to be a JSON list of CoverageRule
					jsonprotos (note that there is no proto message representing
					a list of CoverageRules, i.e. the generated files should not
					actually be valid jsonproto, but they should be a JSON list
					of valid jsonproto). If any file in -generateddir isn't a
					list of CoverageRules, parsing will fail and no rows will be
					uploaded.
				`),
			)

			return r
		},
		Advanced: true,
	}
}

type chromeosCoverageRulesUpdateRun struct {
	bqUpdateRun
	generatedDir string
}

func (r *chromeosCoverageRulesUpdateRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, r, env)
	return errToCode(a, r.run(ctx))
}

func (r *chromeosCoverageRulesUpdateRun) validateFlags(ctx context.Context) error {
	if r.generatedDir == "" {
		return errors.New("-generateddir must be set")
	}

	return nil
}

func (r *chromeosCoverageRulesUpdateRun) run(ctx context.Context) error {
	if err := r.validateFlags(ctx); err != nil {
		return err
	}

	client, table, err := r.getClientAndTable(ctx)
	if err != nil {
		return err
	}

	rows, err := coveragerules.ReadGenerated(ctx, r.generatedDir)
	if err != nil {
		return err
	}

	schema, err := coveragerules.GenerateCoverageRuleBqRowSchema()
	if err != nil {
		return err
	}

	return ensureTableAndUploadRows(ctx, client, table,
		&bigquery.TableMetadata{
			Schema: schema,
			TimePartitioning: &bigquery.TimePartitioning{
				Expiration: r.expiration,
			},
		},
		rows)
}
