// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package parser

import (
	"regexp"

	"infra/cros/satlab/common/utils/errors"
)

var boardAndModelRegex = regexp.MustCompile(`^buildTargets/(?P<Board>\w+)/models/(?P<Model>\w+)$`)
var fwBuildVersionRegex = regexp.MustCompile(`\w+\.(?P<BuildVersion>\d+\.\d+\.\d+)`)
var fwImageBuildVersionRegex = regexp.MustCompile(`\/R\d+-(?P<BuildVersion>\d+\.\d+\.\d+)`)
var milestoneRegex = regexp.MustCompile(`milestones/(?P<Milestone>\d+)$`)

type BoardAndModelPair struct {
	Board string
	Model string
}

// ExtractBoardAndModelFrom extract board and model information from the given string.
//
// string s the string we want to get the information from.
func ExtractBoardAndModelFrom(s string) (*BoardAndModelPair, error) {
	if !boardAndModelRegex.MatchString(s) {
		return nil, errors.NotMatch
	}

	matches := boardAndModelRegex.FindStringSubmatch(s)
	boardIndex := boardAndModelRegex.SubexpIndex("Board")
	modelIndex := boardAndModelRegex.SubexpIndex("Model")

	return &BoardAndModelPair{Board: matches[boardIndex], Model: matches[modelIndex]}, nil
}

// ExtractMilestoneFrom extract the milestone information from the given string.
//
// string s the string we want to get the information from.
func ExtractMilestoneFrom(s string) (string, error) {
	if !milestoneRegex.MatchString(s) {
		return "", errors.NotMatch
	}

	matches := milestoneRegex.FindStringSubmatch(s)
	index := milestoneRegex.SubexpIndex("Milestone")

	return matches[index], nil
}

// ExtractFwImageBuildVersionFrom extract the firmware image
// build version information from the given string.
//
// string s the string we want to get the information from.
func ExtractFwImageBuildVersionFrom(s string) (string, error) {
	if !fwImageBuildVersionRegex.MatchString(s) {
		return "", errors.NotMatch
	}

	matches := fwImageBuildVersionRegex.FindStringSubmatch(s)
	index := fwImageBuildVersionRegex.SubexpIndex("BuildVersion")

	return matches[index], nil
}

// ExtractFwBuildVersionFrom extract the firmware
// build version information from the given string.
//
// string s the string we want to get the information from.
func ExtractFwBuildVersionFrom(s string) (string, error) {
	if !fwBuildVersionRegex.MatchString(s) {
		return "", errors.NotMatch
	}

	matches := fwBuildVersionRegex.FindStringSubmatch(s)
	index := fwBuildVersionRegex.SubexpIndex("BuildVersion")

	return matches[index], nil
}
