// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cli defines different commands for the test_plan tool.
package cli

import (
	"context"
	"errors"
	"path/filepath"

	"cloud.google.com/go/bigquery"
	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/data/text"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/cros/internal/manifestutil"
	"go.chromium.org/infra/cros/internal/testplan/computemapping"
	"go.chromium.org/infra/tools/dirmd/cli/updater"
)

func CmdChromeosDirmdUpdateRun(authOpts auth.Options) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "chromeos-dirmd-update -table project.dataset.table -crossrcroot ~/chromiumos",
		ShortDesc: text.Doc(`
			Computes all DIR_METADATA mappings in a ChromeOS checkout and
			uploads them to a BigQuery table.
		`),
		LongDesc: text.Doc(`
			Computes all DIR_METADATA mappings in a ChromeOS checkout and
			uploads them to a BigQuery table.

			Each mapping is converted to a DirBQRow proto for upload to
			BigQuery. Mappings are computed using the local ChromeOS manifest
			and checkout, i.e. any local changes will be reflected in the
			uploaded rows.

			Note that previous rows for a given directory are not deleted, but
			each execution uses the same partition_time field in every row.
			Thus, queries will often want to use a view of the table that only
			shows rows from the most recent partition.
		`),
		CommandRun: func() subcommands.CommandRun {
			r := &chromeosDirmdUpdateRun{}
			r.addSharedFlags(authOpts)
			r.addBigQueryFlags()

			r.Flags.StringVar(
				&r.crosSrcRoot,
				"crossrcroot",
				"",
				text.Doc(`
					Path to the root of a ChromeOS checkout to use. The manifest
					and DIR_METADATA in the checkout will be used to compute the
					uploaded rows. Required.
				`),
			)

			return r
		},
		Advanced: true,
	}
}

type chromeosDirmdUpdateRun struct {
	bqUpdateRun
	crosSrcRoot string
}

func (r *chromeosDirmdUpdateRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, r, env)
	return errToCode(a, r.run(ctx))
}

func (r *chromeosDirmdUpdateRun) validateFlags(ctx context.Context) error {
	if r.crosSrcRoot == "" {
		return errors.New("-crossrcroot must be set")
	}

	return nil
}

func (r *chromeosDirmdUpdateRun) run(ctx context.Context) error {
	if err := r.validateFlags(ctx); err != nil {
		return err
	}

	client, table, err := r.getClientAndTable(ctx)
	if err != nil {
		return err
	}

	manifestPath := filepath.Join(r.crosSrcRoot, "manifest-internal", "default.xml")
	logging.Infof(ctx, "reading manifest from %q", manifestPath)
	manifest, err := manifestutil.LoadManifestFromFileWithIncludes(manifestPath)
	if err != nil {
		return err
	}

	logging.Infof(ctx, "computing mappings for all repos in the manifest")
	rows, err := computemapping.ToDirBQRows(ctx, r.crosSrcRoot, manifest)
	if err != nil {
		return err
	}

	schema, err := updater.GenerateDirBQRowSchema()
	if err != nil {
		return err
	}

	return ensureTableAndUploadRows(ctx, client, table,
		&bigquery.TableMetadata{
			Schema: schema,
			TimePartitioning: &bigquery.TimePartitioning{
				Expiration: r.expiration,
				Field:      "partition_time",
			},
		},
		rows,
	)
}
