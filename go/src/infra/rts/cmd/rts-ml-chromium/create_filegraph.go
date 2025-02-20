// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bufio"
	"context"
	"os"
	"path/filepath"

	"cloud.google.com/go/bigquery"
	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/data/text"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/rts/filegraph/git"
	"go.chromium.org/infra/rts/internal/chromium"
)

func cmdFileGraph(authOpt *auth.Options) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: `create-filegraph -checkout <path>`,
		ShortDesc: "create a model to be used by select subcommand",
		LongDesc:  "Create a model to be used by select subcommand",
		CommandRun: func() subcommands.CommandRun {
			r := &fileGraphRun{authOpt: authOpt}
			r.Flags.StringVar(&r.modelDir, "model-dir", "", text.Doc(`
				Path to the directory where to write the model files.
				The directory will be created if it does not exist.
			`))

			r.Flags.StringVar(&r.checkout, "checkout", "", "Path to a src.git checkout")
			r.Flags.IntVar(&r.loadOptions.MaxCommitSize, "fg-max-commit-size", 100, text.Doc(`
				Maximum number of files touched by a commit.
				Commits that exceed this limit are ignored.
				The rationale is that large commits provide a weak signal of file
				relatedness and are expensive to process, O(N^2).
			`))
			return r
		},
	}
}

type fileGraphRun struct {
	baseCommandRun
	modelDir string

	checkout    string
	loadOptions git.LoadOptions
	fg          *git.Graph

	authOpt  *auth.Options
	bqClient *bigquery.Client
}

func (r *fileGraphRun) validateFlags() error {
	switch {
	case r.modelDir == "":
		return errors.New("-model-dir is required")
	case r.checkout == "":
		return errors.New("-checkout is required")
	default:
		return nil
	}
}

func (r *fileGraphRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, r, env)
	if len(args) != 0 {
		return r.done(errors.New("unexpected positional arguments"))
	}
	if err := r.validateFlags(); err != nil {
		return r.done(err)
	}

	var err error
	if r.bqClient, err = chromium.NewBQClient(ctx, auth.NewAuthenticator(ctx, auth.InteractiveLogin, *r.authOpt)); err != nil {
		return r.done(errors.Annotate(err, "failed to create BigQuery client").Err())
	}

	return r.done(r.writeFileGraphModel(ctx))
}

// writeFileGraphModel writes the file graph model to the model dir.
func (r *fileGraphRun) writeFileGraphModel(ctx context.Context) error {
	dir := filepath.Join(r.modelDir, "git-file-graph")

	// Ensure model dir exists.
	if err := os.MkdirAll(dir, 0777); err != nil {
		return errors.Annotate(err, "failed to create model dir at %q", dir).Err()
	}

	var err error
	if r.fg, err = git.Load(ctx, r.checkout, r.loadOptions); err != nil {
		return err
	}

	if err := r.writeFileGraph(ctx, filepath.Join(dir, "graph.fg")); err != nil {
		return errors.Annotate(err, "failed to write file graph").Err()
	}

	return nil
}

// writeFileGraph writes the graph file.
func (r *fileGraphRun) writeFileGraph(ctx context.Context, fileName string) error {
	f, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer f.Close()
	bufW := bufio.NewWriter(f)
	if err := r.fg.Write(bufW); err != nil {
		return err
	}
	return bufW.Flush()
}
