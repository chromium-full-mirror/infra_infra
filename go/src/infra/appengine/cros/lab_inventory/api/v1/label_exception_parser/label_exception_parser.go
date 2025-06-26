// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"go.chromium.org/luci/common/errors"
)

type arcTruth struct {
	Board string `json:"board"`
	Arc   string `json:"arc"`
}

type board struct {
	Board string `json:"board"`
}

func parseArc(path string) (map[string]bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Fmt("load arc json %s: %w", path, err)
	}
	arcs := make([]arcTruth, 0)
	if err := json.Unmarshal(b, &arcs); err != nil {
		return nil, errors.Fmt("parse arc json: %w", err)
	}

	arcBoards := make(map[string]bool, 0)
	for _, a := range arcs {
		if a.Arc == "True" {
			arcBoards[a.Board] = true
		}
	}
	return arcBoards, nil
}

func parseBoards(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Fmt("load existing boards json file %s: %w", path, err)
	}
	boards := make([]board, 0)
	if err := json.Unmarshal(b, &boards); err != nil {
		return nil, errors.Fmt("parse boards json file: %w", err)
	}
	bstrs := make([]string, len(boards))
	for i, b := range boards {
		bstrs[i] = b.Board
	}
	return bstrs, nil
}

func printBoardsWithoutArc(boards []string, arcBoards map[string]bool) {
	sorted := make([]string, 0)
	for _, b := range boards {
		if _, ok := arcBoards[b]; !ok {
			sorted = append(sorted, b)
		}
	}
	sort.Strings(sorted)
	for _, s := range sorted {
		fmt.Printf("\"%s\": true,\n", s)
	}
}

func main() {
	boards, err := parseBoards("./existing_boards.json")
	if err != nil {
		fmt.Println(err)
		return
	}
	arcBoards, err := parseArc("./arc.json")
	if err != nil {
		fmt.Println(err)
		return
	}
	printBoardsWithoutArc(boards, arcBoards)
}
