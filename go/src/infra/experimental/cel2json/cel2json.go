// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// cel2json is a simple utility for taking cel-encoded files
// and dumping them as json.
//
// This is useful for exploring the CEL syntax and for writing quick
// tools that use it.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/google/cel-go/cel"
	"google.golang.org/protobuf/encoding/protojson"
)

type celParseError struct {
	path   string
	issues *cel.Issues
}

func (e *celParseError) Error() string {
	return fmt.Sprintf("file %q has issues %v", e.path, e.issues)
}

// main runs the program.
func main() {
	err := doMain(os.Args[1:])
	if err != nil {
		fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

// fprintf is like fmt.Fprintf but exits the program if anything weird happens.
//
// Does not detect short writes.
func fprintf(w io.Writer, format string, a ...any) {
	_, err := fmt.Fprintf(w, format, a...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", "printf failed", err)
		os.Exit(2)
	}
}

// openPath opens a path, but interprets "-" as stdin, like cat does.
func openPath(path string) (io.Reader, error) {
	switch path {
	case "":
		return nil, errors.New(`"" is invalid`)
	case "-":
		return os.Stdin, nil
	default:
		return os.Open(path)
	}
}

// doMain processes a list of files one at a time.
func doMain(paths []string) error {
	if len(paths) == 0 {
		paths = []string{"-"}
	}
	for _, path := range paths {
		fh, err := openPath(path)
		if err != nil {
			return err
		}
		content, err := io.ReadAll(fh)
		if err != nil {
			return fmt.Errorf("reading file %q: %w", path, err)
		}
		env, err := cel.NewEnv()
		if err != nil {
			return fmt.Errorf("reading file %q: %w", path, err)
		}
		ast, issues := env.Parse(string(content))
		if issues != nil {
			return &celParseError{
				path:   path,
				issues: issues,
			}
		}
		expr, err := cel.AstToParsedExpr(ast)
		if err != nil {
			return fmt.Errorf("exporting file %q to proto: %w", path, err)
		}
		bytes, err := protojson.MarshalOptions{
			Indent: "  ",
		}.Marshal(expr)
		if err != nil {
			return fmt.Errorf("marshaling %q: %s", path, err)
		}
		fprintf(os.Stdout, "%s\n", string(bytes))
	}
	return nil
}
