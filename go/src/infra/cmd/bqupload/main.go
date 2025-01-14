// Copyright 2018 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Command bqupload inserts rows in a BigQuery table.
//
// It is a lightweight alternative to 'bq insert' command from gcloud SDK.
//
// Inserts the records formatted as newline delimited JSON from file into
// the specified table. If file is not specified, reads from stdin. If there
// were any insert errors it prints the errors to stderr.
//
// Usage:
//
//	bqupload <project>.<dataset>.<table> [<file>]
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"cloud.google.com/go/bigquery"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/option"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/flag/stringmapflag"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/hardcoded/chromeinfra"
)

const (
	userAgent = "bqupload v1.7"
	// The bigquery API imposes a hard limit of 50,000 rows. We use a much lower
	// default limit to also make it less likely that the total payload size
	// exceeds the maximum, and to limit the blast radius when a batch fails to
	// upload.
	defaultBatchSize = 500
	// crbug/1491321 - http2 concurrent request limits
	// While using http2 in Golang, try to keep max concurrent requests to 100.
	maxConcurrentInserts = 100
)

func usage() {
	fmt.Fprintf(os.Stderr,
		`%s

Usage: bqupload <project>.<dataset>.<table> [<file>]

Inserts the records formatted as newline delimited JSON from file into
the specified table. If file is not specified, reads from stdin. If there
were any insert errors it prints the errors to stderr.

Optional flags:
`, userAgent)
	flag.PrintDefaults()
}

func main() {
	flag.CommandLine.Usage = usage
	if err := run(gologger.StdConfig.Use(context.Background())); err != nil {
		fmt.Fprintf(os.Stderr, "bqupload: %s\n", err)
		os.Exit(1)
	}
}

type uploadOpts struct {
	project string
	dataset string
	table   string
	columns stringmapflag.Value

	input        io.Reader
	auth         oauth2.TokenSource
	insertIDBase string

	ignoreUnknownValues bool
	skipInvalidRows     bool
	jsonList            bool
	batchSize           int
}

func run(ctx context.Context) error {
	// BQ options.
	bqOpts := uploadOpts{}
	flag.BoolVar(&bqOpts.ignoreUnknownValues, "ignore-unknown-values", false,
		"Ignore any values in a row that are not present in the schema.")
	flag.BoolVar(&bqOpts.skipInvalidRows, "skip-invalid-rows", false,
		"Attempt to insert any valid rows, even if invalid rows are present.")
	flag.IntVar(&bqOpts.batchSize, "batch-size", defaultBatchSize,
		"Number of rows per insert batch.")
	flag.BoolVar(&bqOpts.jsonList, "json-list", false,
		"Instead of looking for newline delimited rows, looks for a JSON list of rows.")
	flag.Var(&bqOpts.columns, "column",
		`Parse all the rows as usual, but add or replace these columns in each row. The
		value is parsed as JSON. Can be specified multiple times to set/replace multiple columns.`)

	// Auth options.
	defaults := chromeinfra.DefaultAuthOptions()
	defaults.Scopes = []string{
		"https://www.googleapis.com/auth/bigquery",
		"https://www.googleapis.com/auth/userinfo.email",
	}
	authFlags := authcli.Flags{}
	authFlags.Register(flag.CommandLine, defaults)

	flag.Parse()

	// Parse positional flags.
	args := flag.Args()
	if len(args) == 0 || len(args) > 2 {
		usage()
		os.Exit(2)
	}

	var err error
	bqOpts.project, bqOpts.dataset, bqOpts.table, err = parseTableRef(args[0])
	if err != nil {
		return err
	}

	bqOpts.input = os.Stdin
	if len(args) > 1 {
		f, err := os.Open(args[1])
		if err != nil {
			return err
		}
		defer f.Close()
		bqOpts.input = f
	}

	// Prepare random prefix to use for insert IDs uploaded by this process.
	rnd := make([]byte, 12)
	if _, err = rand.Read(rnd); err != nil {
		return err
	}
	bqOpts.insertIDBase = base64.RawURLEncoding.EncodeToString(rnd)

	// Get oauth2.TokenSource based on parsed auth flags.
	authOpts, err := authFlags.Options()
	if err != nil {
		return err
	}
	authenticator := auth.NewAuthenticator(ctx, auth.SilentLogin, authOpts)
	bqOpts.auth, err = authenticator.TokenSource()
	if err != nil {
		if err == auth.ErrLoginRequired {
			fmt.Fprintf(os.Stderr, "You need to login first by running:\n")
			fmt.Fprintf(os.Stderr, "  luci-auth login -scopes %q\n", strings.Join(defaults.Scopes, " "))
		}
		return err
	}

	// Report who we are running as, helps when debugging permissions. Carry on
	// on errors (there shouldn't be any anyway).
	email, err := authenticator.GetEmail()
	if err != nil {
		logging.Warningf(ctx, "Can't get an email of the active account - %s", err)
	} else {
		logging.Infof(ctx, "Running as %s", email)
	}

	return upload(ctx, &bqOpts)
}

func parseTableRef(ref string) (project, dataset, table string, err error) {
	chunks := strings.Split(ref, ".")
	if len(chunks) != 3 {
		err = fmt.Errorf("table reference should have form <project>.<dataset>.<table>, got %q", ref)
		return
	}
	return chunks[0], chunks[1], chunks[2], nil
}

func upload(ctx context.Context, opts *uploadOpts) error {
	client, err := bigquery.NewClient(ctx, opts.project,
		option.WithTokenSource(opts.auth),
		option.WithUserAgent(userAgent))
	if err != nil {
		return err
	}
	defer client.Close()

	inserter := client.Dataset(opts.dataset).Table(opts.table).Inserter()
	inserter.IgnoreUnknownValues = opts.ignoreUnknownValues
	inserter.SkipInvalidRows = opts.skipInvalidRows

	// Prepare column overrides.
	overrides := make(map[string]bigquery.Value, len(opts.columns))
	for key, value := range opts.columns {
		var val bigquery.Value
		d := json.NewDecoder(strings.NewReader(value))
		d.UseNumber()
		if err := d.Decode(&val); err != nil {
			return errors.Annotate(err, "parsing -column %q value", key).Err()
		}
		overrides[key] = val
	}

	// Note: we may potentially read rows from 'input' and upload them at the same
	// time for true streaming uploads in case 'input' is stdin and it's produced
	// on the fly. This is not trivial though and isn't needed yet, so we read
	// everything at once.
	rows, err := readInput(opts.input, opts.insertIDBase, opts.jsonList, overrides)
	if err != nil {
		return err
	}

	logging.Infof(ctx,
		"Inserting %d rows into table `%s.%s.%s`",
		len(rows), opts.project, opts.dataset, opts.table)

	if err := doInsert(ctx, os.Stderr, opts, inserter, rows); err != nil {
		return err
	}

	logging.Infof(ctx, "Done")
	return nil
}

// For testability.
type bqInserter interface {
	Put(ctx context.Context, src interface{}) error
}

func doInsert(ctx context.Context, stderr io.Writer, opts *uploadOpts, inserter bqInserter, rows []*tableRow) error {
	var mu sync.Mutex
	var multiErr bigquery.PutMultiError
	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(maxConcurrentInserts)
	for i := 0; i < len(rows); i += opts.batchSize {
		i := i
		eg.Go(func() error {
			end := i + opts.batchSize
			if end > len(rows) {
				end = len(rows)
			}
			batch := rows[i:end]
			if err := inserter.Put(egCtx, batch); err != nil {
				if merr, ok := err.(bigquery.PutMultiError); ok {
					mu.Lock()
					multiErr = append(multiErr, merr...)
					mu.Unlock()
				} else {
					// Unrecognized errors should be considered fatal.
					return err
				}
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	if len(multiErr) > 0 {
		fmt.Fprintf(stderr, "Failed to upload some rows:\n")
		for _, rowErr := range multiErr {
			for _, valErr := range rowErr.Errors {
				fmt.Fprintf(stderr, "row %d: %s\n", rowErr.RowIndex, valErr)
			}
		}
		return multiErr
	}
	return nil
}

func overrideColumns(row map[string]bigquery.Value, columns map[string]bigquery.Value) map[string]bigquery.Value {
	for k, v := range columns {
		row[k] = v
	}
	return row
}

func readInput(r io.Reader, insertIDBase string, jsonList bool, overrides map[string]bigquery.Value) (rows []*tableRow, err error) {
	if jsonList {
		var target []map[string]bigquery.Value
		d := json.NewDecoder(r)
		d.UseNumber()
		if err := d.Decode(&target); err != nil {
			return nil, err
		}
		rows = make([]*tableRow, len(target))
		for i, row := range target {
			rows[i] = &tableRow{overrideColumns(row, overrides), fmt.Sprintf("%s:%d", insertIDBase, i)}
		}
		return
	}

	buf := bufio.NewReaderSize(r, 32768)

	lineNo := 0
	for {
		lineNo++

		line, err := buf.ReadBytes('\n')
		switch {
		case err != nil && err != io.EOF:
			return nil, err // a fatal error
		case err == io.EOF && len(line) == 0:
			return rows, nil // read past the last line
		}

		if line = bytes.TrimSpace(line); len(line) != 0 {
			row, err := parseRow(line, fmt.Sprintf("%s:%d", insertIDBase, len(rows)))
			if err != nil {
				return nil, fmt.Errorf("bad input line %d: %s", lineNo, err)
			}
			row.data = overrideColumns(row.data, overrides)
			rows = append(rows, row)
		}
	}
}

// tableRow implements bigquery.ValueSaver.
type tableRow struct {
	data     map[string]bigquery.Value
	insertID string
}

func parseRow(data []byte, insertID string) (*tableRow, error) {
	row := make(map[string]bigquery.Value)
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if err := d.Decode(&row); err != nil {
		return nil, fmt.Errorf("bad JSON - %s", err)
	}
	return &tableRow{row, insertID}, nil
}

func (r *tableRow) Save() (map[string]bigquery.Value, string, error) {
	return r.data, r.insertID, nil
}
