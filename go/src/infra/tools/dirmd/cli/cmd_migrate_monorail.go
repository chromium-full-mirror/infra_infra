// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/maruel/subcommands"
	"google.golang.org/protobuf/encoding/prototext"

	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/data/text"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/sync/parallel"

	"go.chromium.org/infra/tools/dirmd"
	dirmdpb "go.chromium.org/infra/tools/dirmd/proto"
)

// TODO(crbug.com/1505875) - Deprecate this once migration is complete.
func cmdMigrateMonorail() *subcommands.Command {
	return &subcommands.Command{
		UsageLine: `monorail DIR1 [DIR2...]`,
		ShortDesc: "migrate Monorail metadata in the given directories",
		LongDesc: text.Doc(`
			Migrate Monorail metadata in the given directories.

			Translate from Monorail to Buganizer definitions in the given directories
			and all subdirectories using the file provided.

			The file is expected to be a .textproto with the format defined in
			proto/component_def.proto.

			This command will be deprecated once the migration is complete.
			See http://crbug/1505875.
		`),
		CommandRun: func() subcommands.CommandRun {
			r := &migrateMonorailRun{}
			r.Flags.StringVar(&r.migrationFilepath, "file-path", "", "Absolute path to the file mapping.")
			r.Flags.StringVar(&r.rootFilePath, "root-path", "", "Absolute path to the root of the project. Used to find and update mixin metadata files.")
			return r
		},
	}
}

type migrateMonorailRun struct {
	baseCommandRun
	migrationFilepath string
	rootFilePath      string
}

func (r *migrateMonorailRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, r, env)

	if r.migrationFilepath == "" || r.rootFilePath == "" {
		return r.done(ctx, errors.Reason("-file-path and -root must be specified.").Err())
	}

	if !filepath.IsAbs(r.migrationFilepath) || !filepath.IsAbs(r.rootFilePath) {
		return r.done(ctx, errors.Reason("-file-path and -root must be defined as an absolute filepath.").Err())
	}

	return r.done(ctx, r.run(ctx, args))
}

func (r *migrateMonorailRun) run(ctx context.Context, dirs []string) error {
	contents, err := os.ReadFile(r.migrationFilepath)
	if err != nil {
		return err
	}

	cfg := &dirmdpb.ComponentsConfig{}
	err = prototext.Unmarshal(contents, cfg)
	if err != nil {
		return err
	}

	cm := make(map[string]int64, 0)
	for _, c := range cfg.ComponentDef {
		// Lowercase the component path for consistency
		cm[strings.ToLower(c.Path)] = c.BuganizerId
	}

	return parallel.WorkPool(16, func(workC chan<- func() error) {
		for _, dir := range dirs {
			err := filepath.Walk(dir, func(dir string, info os.FileInfo, err error) error {
				switch {
				case err != nil:
					return err
				case !info.IsDir():
					return nil
				}
				workC <- func() error {
					err = dirmd.MigrateMonorailMetadata(dir, cm, r.rootFilePath)
					if err != nil {
						logging.Warningf(ctx, "%s", err)
						return err
					}
					return nil
				}
				return nil
			})
			if err != nil {
				workC <- func() error {
					return err
				}
			}
		}
	})
}
