// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Firestore_cli is a simple CLI wrapper for upserting and fetching documents from the command line.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"

	"go.chromium.org/infra/cmdsupport/cmdlib"
)

func getCollection(ctx context.Context, common commonFlags) (*firestore.CollectionRef, error) {
	projectID := firestore.DetectProjectID

	if common.ProjectID != "" {
		projectID = common.ProjectID
	}

	c, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}

	return c.Collection(common.CollectionID), nil
}

type commonFlags struct {
	subcommands.CommandRunBase
	CollectionID string
	ProjectID    string
}

func (c *commonFlags) init(fs *flag.FlagSet) {
	fs.StringVar(&c.CollectionID, "collection", "", "firestore collection ID")
	fs.StringVar(&c.ProjectID, "project", "", "GCP project that contains the firestore collection")
}

func (c *commonFlags) parse() error {
	if c.CollectionID == "" {
		return fmt.Errorf("must supply collection ID")
	}

	return nil
}

var cmdSet = &subcommands.Command{
	UsageLine: "set <document-id>",
	ShortDesc: "upserts a document",
	LongDesc:  "Upserts a document to the provided firestore collection, if no document ID is provided a new one is created.",
	CommandRun: func() subcommands.CommandRun {
		c := &setRun{}
		c.init(&c.Flags)
		c.Flags.StringVar(&c.fromFile, "from-file", "", "upload the provided JSON file as a document.")
		return c
	},
}

type setRun struct {
	commonFlags
	fromFile string
	subcommands.CommandRunBase
}

// set creates or if the document already exists overrides a single document in a collection.
func (s *setRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, s, env)

	if err := s.parse(); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}

	c, err := getCollection(ctx, s.commonFlags)
	if err != nil {
		cmdlib.PrintError(a, fmt.Errorf("could not get collection: %w", err))
		return 1
	}

	in := os.Stdin

	if s.fromFile != "" {
		f, err := os.OpenFile(s.fromFile, os.O_RDONLY, 0755)
		if err != nil {
			cmdlib.PrintError(a, fmt.Errorf("could not open provided file %s: %w", s.fromFile, err))
			return 1
		}
		defer f.Close()
		in = f
	}

	doc := make(map[string]any)
	if err := json.NewDecoder(in).Decode(&doc); err != nil {
		cmdlib.PrintError(a, fmt.Errorf("could not decode input: %w", err))
		return 1
	}

	dr := c.NewDoc()

	if len(args) > 0 {
		if len(args) != 1 {
			cmdlib.PrintError(a, fmt.Errorf("set only takes one argument: <document-id>"))
			return 1
		}
		dr = c.Doc(args[0])
	}

	res, err := dr.Set(ctx, doc)
	if err != nil {
		cmdlib.PrintError(a, fmt.Errorf("could not set document: %w", err))
		return 1
	}

	b, err := json.MarshalIndent(struct {
		ID      string    `json:"id"`
		Updated time.Time `json:"updated_time"`
	}{dr.ID, res.UpdateTime}, "", "  ")
	if err != nil {
		cmdlib.PrintError(a, fmt.Errorf("could not marshal result: %w", err))
		return 1
	}

	fmt.Fprint(os.Stdout, string(b)+"\n")

	return 0
}

var cmdDelete = &subcommands.Command{
	UsageLine: "del <document-id>",
	ShortDesc: "deletes a document",
	LongDesc:  "Deletes a document from the firestore collection.",
	CommandRun: func() subcommands.CommandRun {
		c := &deleteRun{}
		c.init(&c.Flags)
		return c
	},
}

type deleteRun struct {
	commonFlags
	subcommands.CommandRunBase
}

// delete a single document.
func (d *deleteRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, d, env)

	if err := d.parse(); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}

	c, err := getCollection(ctx, d.commonFlags)
	if err != nil {
		cmdlib.PrintError(a, fmt.Errorf("could not get collection: %w", err))
		return 1
	}

	if len(args) != 1 {
		cmdlib.PrintError(a, fmt.Errorf("delete expects one argument: <document-id>"))
		return 1
	}
	dr := c.Doc(args[0])

	if _, err := dr.Delete(ctx); err != nil {
		cmdlib.PrintError(a, fmt.Errorf("could not delete document: %w", err))
		return 1
	}
	return 0
}

var cmdGet = &subcommands.Command{
	UsageLine: "get <document-id>",
	ShortDesc: "outputs a document to stdout",
	LongDesc:  "Outputs a document from the firestore collection in JSON to stdout.",
	CommandRun: func() subcommands.CommandRun {
		c := &getRun{}
		c.init(&c.Flags)
		return c
	},
}

type getRun struct {
	commonFlags
	subcommands.CommandRunBase
}

// get a single document.
func (g *getRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, g, env)

	if err := g.parse(); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}

	c, err := getCollection(ctx, g.commonFlags)
	if err != nil {
		cmdlib.PrintError(a, fmt.Errorf("could not get collection: %w", err))
		return 1
	}

	if len(args) != 1 {
		cmdlib.PrintError(a, fmt.Errorf("get expects one argument: <document-id>"))
		return 1
	}

	documentID := args[0]

	s, err := c.Doc(documentID).Get(ctx)
	if err != nil {
		cmdlib.PrintError(a, fmt.Errorf("could not get document %s: %w", documentID, err))
		return 1
	}

	doc := s.Data()
	if doc == nil {
		cmdlib.PrintError(a, fmt.Errorf("document %s/%s not found", c.ID, documentID))
		return 1
	}

	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		cmdlib.PrintError(a, fmt.Errorf("could not marshal result: %w", err))
		return 1
	}

	fmt.Fprint(os.Stdout, string(b)+"\n")
	return 0
}

func main() {
	app := &cli.Application{
		Name:  "firestorecli",
		Title: "helper CLI to do simple firestore operations",
		Commands: []*subcommands.Command{
			cmdDelete,
			cmdGet,
			cmdSet,
		},
	}

	os.Exit(subcommands.Run(app, nil))
}
