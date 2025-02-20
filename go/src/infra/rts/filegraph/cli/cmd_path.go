// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cli

import (
	"bufio"
	"fmt"
	"os"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/data/text"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/rts/filegraph"
	"go.chromium.org/infra/rts/filegraph/git"
)

var cmdPath = &subcommands.Command{
	UsageLine: `path [flags] SOURCE_FILE TARGET_FILE`,
	ShortDesc: "print the shortest path from SOURCE_FILE to TARGET_FILE",
	LongDesc: text.Doc(`
		Print the shortest path from SOURCE_FILE to TARGET_FILE

		Each output line has format "<distance> (+<delta>) <filename>",
		where the filename is forward-slash-separated and has "//" prefix.
		Output example:
			0.00 (+0.00) //source_file.cc
			1.00 (+1.00) //intermediate_file.cc
			3.00 (+2.00) //target_file.cc

		Both files must be in the same git repository. When a filegraph is
		provided these files need to be in the filegraph name and present in
		the filegraph itself.
	`),
	CommandRun: func() subcommands.CommandRun {
		r := &pathRun{}
		r.Flags.StringVar(&r.filegraph, "filegraph", "", text.Doc(`
			Path to a pre generated filegraph. When set this will use files
			within the provided filegraph, not files in the repo.`))
		r.gitGraph.RegisterFlags(&r.Flags)
		return r
	},
}

type pathRun struct {
	baseCommandRun
	gitGraph
	filegraph string
}

func (r *pathRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, r, env)

	if len(args) != 2 {
		return r.done(errors.Reason("usage: filegraph path SOURCE_FILE TARGET_FILE").Err())
	}

	var startNode filegraph.Node
	var endNode filegraph.Node
	if r.filegraph == "" {
		nodes, err := r.loadSyncedNodes(ctx, args[0], args[1])
		if err != nil {
			return r.done(err)
		}
		startNode = nodes[0]
		endNode = nodes[1]
	} else {
		f, err := os.Open(r.filegraph)
		if err != nil {
			return r.done(err)
		}
		defer f.Close()
		r.Graph = &git.Graph{}
		r.Read(bufio.NewReader(f))
		startNode = r.Graph.Node(args[0])
		if startNode == nil {
			return r.done(errors.Reason("source file not found in the filegraph").Err())
		}
		endNode = r.Graph.Node(args[1])
		if endNode == nil {
			return r.done(errors.Reason("target file not found in the filegraph").Err())
		}
	}

	shortest := r.query(startNode).ShortestPath(endNode)
	if shortest == nil {
		return r.done(errors.New("not reachable"))
	}

	prevDist := 0.0
	for _, sp := range shortest.Path() {
		fmt.Printf("%.2f (+%.2f) %s\n", sp.Distance, sp.Distance-prevDist, sp.Node.Name())
		prevDist = sp.Distance
	}
	return 0
}
