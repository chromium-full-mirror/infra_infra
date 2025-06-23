// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

// MatchCrossystemValueToExpectation reads value from crossystem and compared to expected value.
func MatchCrossystemValueToExpectation(ctx context.Context, run components.Runner, subcommand string, expectedValue string) error {
	out, err := run(ctx, time.Minute, "crossystem", subcommand)
	if err != nil {
		return errors.Annotate(err, "match crossystem value to expectation: fail read %s", subcommand).Err()
	}
	actualValue := strings.TrimSpace(out)
	if actualValue != expectedValue {
		return errors.Reason("match crossystem value to expectation: %q, found: %q", expectedValue, actualValue).Err()
	}
	return nil
}

// MatchSuffixValueToExpectation reads value from crossystem, split both read out and expected value with a given delimiter and then compare their suffix.
func MatchSuffixValueToExpectation(ctx context.Context, run components.Runner, subcommand string, expectedValue string, delimiter string) error {
	out, err := run(ctx, time.Minute, "crossystem", subcommand)
	if err != nil {
		return errors.Annotate(err, "match suffix value to expectation: fail read %s", subcommand).Err()
	}

	splittedOut := strings.SplitN(strings.TrimSpace(out), delimiter, 2)
	if len(splittedOut) != 2 {
		return errors.Reason("match suffix value to expectation: cannot split output %s with delimiter %s", out, delimiter).Err()
	}
	actual := splittedOut[1]
	if actual == "" {
		return errors.Reason("match suffix value to expectation: suffix from output value is empty after split.").Err()
	}
	log.Debugf(ctx, "Suffix found from splitted output value: %s", actual)

	splittedExpectedValue := strings.SplitN(expectedValue, delimiter, 2)
	if len(splittedExpectedValue) != 2 {
		return errors.Reason("match suffix value to expectation: cannot split expected value %s with delimiter %s", expectedValue, delimiter).Err()
	}
	expected := splittedExpectedValue[1]
	if expected == "" {
		return errors.Reason("match suffix value to expectation: suffix from expected value is empty after split.").Err()
	}
	log.Debugf(ctx, "Suffix found from splitted expected value: %s", expected)

	if actual != expected {
		return errors.Reason("match crossystem value to expectation: %q, found: %q", expected, actual).Err()
	}
	return nil
}

// UpdateCrossystem sets value of the subcommand to the value passed in.
//
// @params: check: bool value to check whether the crossystem command is being updated successfully.
func UpdateCrossystem(ctx context.Context, run components.Runner, cmd string, val string, check bool) error {
	if _, err := run(ctx, time.Minute, fmt.Sprintf("crossystem %s=%s", cmd, val)); err != nil {
		return errors.Annotate(err, "update crossystem value").Err()
	}
	if check {
		return errors.WrapIf(MatchCrossystemValueToExpectation(ctx, run, cmd, val), "update crossystem value")
	}
	return nil
}
