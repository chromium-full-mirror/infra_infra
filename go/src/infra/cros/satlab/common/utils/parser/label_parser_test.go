// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package parser

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	e "infra/cros/satlab/common/utils/errors"
)

func TestExtractBoardAndBoardShouldWork(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input  string
		output *BoardAndModelPair
		err    error
	}{
		{
			"buildTargets/b1/models/m1",
			&BoardAndModelPair{Board: "b1", Model: "m1"},
			nil,
		},
		{
			"buildTagets/b1/m/m1",
			nil,
			e.NotMatch,
		},
	}

	for _, tt := range cases {
		expected := tt.output
		actual, err := ExtractBoardAndModelFrom(tt.input)
		if diff := cmp.Diff(expected, actual); diff != "" {
			t.Errorf("unexpected diff: %s", diff)
		}

		if !errors.Is(err, tt.err) {
			t.Errorf("Expected: %v, got: %v", tt.err, err)
		}
	}
}

func TestExtractMilestoneShouldWork(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input  string
		output string
		err    error
	}{
		{
			"milestones/119",
			"119",
			nil,
		},
		{
			"milestone/119",
			"",
			e.NotMatch,
		},
	}

	for _, tt := range cases {
		expected := tt.output
		actual, err := ExtractMilestoneFrom(tt.input)
		if diff := cmp.Diff(expected, actual); diff != "" {
			t.Errorf("Expected: %v, unexpected diff: %s", expected, diff)
		}

		if !errors.Is(err, tt.err) {
			t.Errorf("Expected: %v, got: %v", tt.err, err)
		}
	}

}

func TestExtractFwImageBuildVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input  string
		output string
		err    error
	}{
		{
			"board-release/R111-16112.0.0/firmware_from_source.tar.bz2",
			"16112.0.0",
			nil,
		},
		{
			"board-firmware/R111-16112.0.0",
			"16112.0.0",
			nil,
		},
		{
			"firmware-boardA-11709.B-branch-firmware/R102-15749.126.0/boardB",
			"15749.126.0",
			nil,
		},
		{
			"firmware-board-15577.B-branch-firmware/R89-13527.574.0",
			"13527.574.0",
			nil,
		},
		{
			"board-release/R111/6112.0.0/firmware_from_source.tar.bz2",
			"",
			e.NotMatch,
		},
		{
			"board-firmware/R111-16112.0",
			"",
			e.NotMatch,
		},
		{
			"firmware-boardA-11709.B-branch-firmware/M102-15749.126.0/boardB",
			"",
			e.NotMatch,
		},
		{
			"firmware-board-15577.B-branch-firmware-R89-13527.574.0",
			"",
			e.NotMatch,
		},
	}

	for _, tt := range cases {
		expected := tt.output
		actual, err := ExtractFwImageBuildVersionFrom(tt.input)
		if diff := cmp.Diff(expected, actual); diff != "" {
			t.Errorf("Expected: %v, unexpected diff: %s", expected, diff)
		}

		if !errors.Is(err, tt.err) {
			t.Errorf("Expected: %v, got: %v", tt.err, err)
		}
	}
}

func TestExtractFwBuildVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input  string
		output string
		err    error
	}{
		{
			"Google_Chronicler.16112.0.0",
			"16112.0.0",
			nil,
		},
		{
			"Google_Craask.16112.0.0",
			"16112.0.0",
			nil,
		},
		{
			"board-release/R111/6112.0.0/firmware_from_source.tar.bz2",
			"",
			e.NotMatch,
		},
		{
			"Google_Craask.16112.0",
			"",
			e.NotMatch,
		},
		{
			"Google_Craask-15749.126.0",
			"",
			e.NotMatch,
		},
		{
			"Google_Chronicler.R89-13527.574.0",
			"",
			e.NotMatch,
		},
	}

	for _, tt := range cases {
		expected := tt.output
		actual, err := ExtractFwBuildVersionFrom(tt.input)
		if diff := cmp.Diff(expected, actual); diff != "" {
			t.Errorf("Expected: %v, unexpected diff: %s", expected, diff)
		}

		if !errors.Is(err, tt.err) {
			t.Errorf("Expected: %v, got: %v", tt.err, err)
		}
	}
}
