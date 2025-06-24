// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package heuristics

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"strings"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/errors"
)

// LooksLikeSatlabRemoteAccessContainer determines whether the container we are running on looks like
// a satlab remote access container.
func LooksLikeSatlabRemoteAccessContainer() (bool, error) {
	_, err := os.Stat("/usr/local/bin/get_host_identifier")
	if err == nil {
		// Happy path, we're in a satlab remote access container.
		return true, nil
	}
	if ok := errors.Is(err, fs.ErrNotExist); !ok {
		// Semi-happy path, we successfully determined that we're not in a satlab remote access container.
		return false, nil
	}
	return false, errors.Fmt("looks like satlab remote access container: %w", err)
}

// LooksLikeSatlabDevice returns whether a hostname or botID appears to be a satlab-managed device.
// This function exists so that we use the same heuristic everywhere when identifying satlab devices.
func LooksLikeSatlabDevice(hostname string) bool {
	h := strings.TrimPrefix(hostname, "crossk-")
	return strings.HasPrefix(h, "satlab")
}

// LooksLikeLabstation returns whether a hostname or botID appears to be a labstation or not.
// This function exists so that we always use the same heuristic everywhere when identifying labstations.
func LooksLikeLabstation(hostname string) bool {
	return strings.Contains(hostname, "labstation")
}

// LooksLikeHeader heuristically determines whether a CSV line looks like
// a CSV header for the MCSV format.
func LooksLikeHeader(rec []string) bool {
	if len(rec) == 0 {
		return false
	}
	return strings.EqualFold(rec[0], "name")
}

// HwSwarmingBotIDPrefixes includes all possible prefix for bots for DUTs
var HwSwarmingBotIDPrefixes = []string{"crossk-", "cros-"}

// NormalizeBotNameToDeviceName takes a bot name or a DUT name and normalizes it to a DUT name.
// The prefix "crossk-" or "cros-" is suspicious and should be removed. The suffix ".cros" is also suspicious and should be removed.
func NormalizeBotNameToDeviceName(hostname string) string {
	res := hostname
	for _, p := range HwSwarmingBotIDPrefixes {
		if strings.HasPrefix(hostname, p) {
			res = strings.TrimPrefix(hostname, p)
			break
		}
	}
	return strings.TrimSuffix(res, ".cros")
}

// looksLikeValidPool heuristically checks a string to see if it looks like a valid pool.
// A heuristically valid pool name contains only a-z, A-Z, 0-9, -, and _ .
// A pool name cannot begin with - and 0-9 .
var LooksLikeValidPool = regexp.MustCompile(`\A[A-Za-z_][-A-Za-z0-9_]*\z`).MatchString

// NormalizeTextualData lowercases data and removes leading and trailing whitespace.
func NormalizeTextualData(data string) string {
	return strings.ToLower(strings.TrimSpace(data))
}

// NormalizeServoNameToDeviceName converts a servo name (ending in "-servo") to a device name by stripping
// the "-servo" suffix, which is a longstanding convention.
func NormalizeServoNameToDeviceName(servo string) string {
	return strings.TrimSuffix(servo, "-servo")
}

// LooksLikeFieldMask checks whether a given string looks like a field mask.
var LooksLikeFieldMask = regexp.MustCompile(`\A[a-z][A-Za-z0-9\.]*\z`).MatchString

// TaskType is the high level type of a task such as repair or audit. It indicates what
// implementation (legacy or paris) and what level of stability (prod vs canary) should be used.
type TaskType int

const (
	// ProdTaskType refers to the paris task on the prod cipd label.
	ProdTaskType TaskType = 100
	// LatestTaskType refers to the paris task on the latest cipd label.
	LatestTaskType TaskType = 200
)

// TruncateErrorString truncates a string to 1400 characters, a number that can safely be stored in both datastore
// and bigquery.
// Returns the truncated string.
func TruncateErrorString(msg string) string {
	// We take 3 characters for "...".
	// Characters at the end of the string tend to be more informative than those at the beginning,
	// so of our (1400-3) character budget, we spend 20% on the prefix and 80% on the suffix.
	fPrefixLen := (1400 - 3) * 0.2
	prefixLen := int(fPrefixLen)
	suffixLen := 1400 - 3 - prefixLen
	if len(msg)+3 < 1400 {
		return msg
	}
	prefix := msg[0:prefixLen]
	suffix := msg[(len(msg) - suffixLen):]
	return strings.ToValidUTF8(fmt.Sprintf("%s...%s", prefix, suffix), "")
}

// ParseUsingCommand is a test helper for subcommands.
func ParseUsingCommand(c *subcommands.Command, args []string, validate func(subcommands.CommandRun) error) (*flag.FlagSet, error) {
	runner := c.CommandRun()
	flagSet := runner.GetFlags()
	if err := flagSet.Parse(args); err != nil {
		return nil, err
	}
	if err := validate(runner); err != nil {
		return nil, err
	}
	return flagSet, nil
}
