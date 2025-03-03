// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package git

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/rts/filegraph"
	"go.chromium.org/infra/rts/internal/gitutil"
)

// BenchmarkE2E measures performance of this package end to end:
//   - Builds the graph from a git checkout
//   - Serializes it.
//   - Deserializes it.
//   - Runs queries from some files
//
// Uses the git checkout at path in the FILEGRAPH_BENCH_CHECKOUT environment
// variable. If it is not defined, then uses current repo (luci-go.git).
func BenchmarkE2E(b *testing.B) {
	ctx := context.Background()
	repoDir := benchRepoDir(b)

	var g *Graph

	// First, build the graph from scratch.
	b.Run("Build", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			g = &Graph{}
			if err := g.Update(ctx, repoDir, "refs/heads/main", UpdateOptions{}); err != nil {
				b.Fatal(err)
			}
		}
		b.StopTimer()
		printStats(g, b)
	})

	// Serialize it.
	buf := &bytes.Buffer{}
	b.Run("Write", func(b *testing.B) {
		for range b.N {
			buf.Reset()
			if err := g.Write(buf); err != nil {
				b.Fatal(err)
			}
		}
	})

	// Deserialize it.
	b.Run("Read", func(b *testing.B) {
		for range b.N {
			r := bufio.NewReader(bytes.NewReader(buf.Bytes()))
			if err := g.Read(r); err != nil {
				b.Fatal(err)
			}
		}
	})

	// Run queries for each top-level file.
	for _, n := range g.root.children {
		n := n
		if len(n.children) > 0 {
			continue
		}
		b.Run("Query-"+strings.TrimPrefix(n.name, "//"), func(b *testing.B) {
			for range b.N {
				q := filegraph.Query{Sources: []filegraph.Node{n}, EdgeReader: &EdgeReader{}}
				q.Run(func(*filegraph.ShortestPath) bool {
					return true
				})
			}
		})
	}
}

func TestE2E(t *testing.T) {
	t.Parallel()

	repoDir := benchRepoDir(t)
	ftt.Run(`E2E`, t, func(t *ftt.Test) {
		ctx := context.Background()

		// Build the graph from scratch.
		g := &Graph{}
		err := g.Update(ctx, repoDir, "refs/remotes/origin/main", UpdateOptions{})
		assert.Loosely(t, err, should.BeNil)

		// Ensure each file in the repo is also present in the graph.
		gitListFiles(ctx, t, repoDir, "origin/main", func(file string) {
			n := g.node("//" + file)
			t.Log(file)
			assert.Loosely(t, n, should.NotBeNil)
		})
	})
}

func gitListFiles(ctx context.Context, t *ftt.Test, dir, ref string, callback func(file string)) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "ls-files", ref)
	stdout, err := cmd.StdoutPipe()
	assert.Loosely(t, err, should.BeNil)
	assert.Loosely(t, cmd.Start(), should.BeNil)

	scan := bufio.NewScanner(stdout)
	for scan.Scan() {
		callback(scan.Text())
	}
	assert.Loosely(t, scan.Err(), should.BeNil)
	assert.Loosely(t, cmd.Wait(), should.BeNil)
}

func printStats(g *Graph, b *testing.B) {
	nodes := 0
	edges := 0
	g.root.visit(func(n *node) bool {
		nodes++
		edges += len(n.edges)
		return true
	})
	b.Logf("%d nodes, %d edges", nodes, edges)
}

func benchRepoDir(b testing.TB) string {
	if repoDir := os.Getenv("FILEGRAPH_BENCH_CHECKOUT"); repoDir != "" {
		return repoDir
	}

	cwd, err := os.Getwd()
	if err != nil {
		b.Fatal(err)
	}
	repoDir, err := gitutil.TopLevel(cwd)
	if err != nil {
		b.Fatal(err)
	}
	return repoDir
}
