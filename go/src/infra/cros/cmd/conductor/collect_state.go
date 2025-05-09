// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	pb "go.chromium.org/chromiumos/infra/proto/go/chromiumos"
	bbpb "go.chromium.org/luci/buildbucket/proto"
)

type Clock interface {
	Now() int64
}

type RealClock struct{}

func (c *RealClock) Now() int64 {
	return time.Now().Unix()
}

type Rule struct {
	rule              *pb.RetryRule
	builderNameRe     []*regexp.Regexp
	summaryMarkdownRe []*regexp.Regexp
	totalRetries      uint32
	retriesByBuild    map[string]uint32
}

// CollectState tracks state for a conductor collect.
type CollectState struct {
	stdoutLog *log.Logger
	stderrLog *log.Logger

	rules             []*Rule
	startTime         int64
	clock             Clock
	initialBuildCount int
}

// LogOut logs to stdout.
func (c *CollectState) LogOut(format string, a ...any) {
	if c.stdoutLog != nil {
		c.stdoutLog.Printf(format, a...)
	}
}

// LogErr logs to stderr.
func (c *CollectState) LogErr(format string, a ...any) {
	if c.stderrLog != nil {
		c.stderrLog.Printf(format, a...)
	}
}

type collectStateOpts struct {
	config            *pb.CollectConfig
	stdoutLog         *log.Logger
	stderrLog         *log.Logger
	initialBuildCount int
}

// initCollectState_inner returns a new CollectState based on the specified config.
func initCollectState_inner(opts *collectStateOpts, clock Clock) *CollectState {
	c := &CollectState{
		clock:             clock,
		stdoutLog:         opts.stdoutLog,
		stderrLog:         opts.stderrLog,
		startTime:         clock.Now(),
		initialBuildCount: opts.initialBuildCount,
	}
	c.rules = c.initRules(opts.config)
	return c
}

// initCollectState returns a new CollectState based on the specified config.
func initCollectState(opts *collectStateOpts) *CollectState {
	clock := &RealClock{}
	return initCollectState_inner(opts, clock)
}

// initCollectStateTest returns a new CollectState based on the specified
// config that uses the specified clock.
func initCollectStateTest(opts *collectStateOpts, clock Clock) *CollectState {
	return initCollectState_inner(opts, clock)
}

// initRules initializes the rules (and associated state) for the collect state.
func (c *CollectState) initRules(config *pb.CollectConfig) []*Rule {
	rules := []*Rule{}
	for i, rule := range config.GetRules() {
		r := &Rule{
			rule:           rule,
			totalRetries:   0,
			retriesByBuild: map[string]uint32{},
		}
		skipRule := false
		builderNameRe := []*regexp.Regexp{}
		// We don't want to crash out a whole build due to bad config, so if
		// we can't build a rule for any reason, we'll omit it (and log that
		// we're doing so) -- no retries is closer to baseline behavior.
		for _, re := range rule.GetBuilderNameRe() {
			exp, err := regexp.Compile(re)
			if err != nil {
				skipRule = true
				c.LogErr("Could not compile regexp `%s`, skipping rule %d", re, i)
				break
			}
			builderNameRe = append(builderNameRe, exp)
		}
		if skipRule {
			continue
		}
		r.builderNameRe = builderNameRe

		summaryMarkdownRe := []*regexp.Regexp{}
		for _, re := range rule.GetSummaryMarkdownRe() {
			exp, err := regexp.Compile(re)
			if err != nil {
				skipRule = true
				c.LogErr("Could not compile regexp `%s`, skipping rule %d", re, i)
				break
			}
			summaryMarkdownRe = append(summaryMarkdownRe, exp)
		}
		if skipRule {
			continue
		}
		r.summaryMarkdownRe = summaryMarkdownRe

		rules = append(rules, r)
	}
	return rules
}

func matchesAny(term string, res []*regexp.Regexp) bool {
	for _, re := range res {
		if re.MatchString(term) {
			return true
		}
	}
	return false
}

func extractCheckpointStatus(build *bbpb.Build, checkpoint pb.RetryStep) *string {
	prop, ok := build.GetOutput().GetProperties().GetFields()["retry_summary"]
	if !ok {
		return nil
	}
	retrySummary := prop.GetStructValue().AsMap()
	status, ok := retrySummary[checkpoint.String()]
	if !ok {
		return nil
	}
	statusString, ok := status.(string)
	if !ok {
		return nil
	}
	return &statusString
}

// matches evaluates whether the given build result matches the rule.
func (r *Rule) matches(build *bbpb.Build) bool {
	if len(r.rule.GetStatus()) > 0 {
		status := build.GetStatus()
		statusMatch := false
		for _, ruleStatus := range r.rule.GetStatus() {
			if status == bbpb.Status(ruleStatus) {
				statusMatch = true
				break
			}
		}
		if !statusMatch {
			return false
		}
	}

	builderName := build.GetBuilder().GetBuilder()
	if len(r.builderNameRe) > 0 {
		matches := matchesAny(builderName, r.builderNameRe)
		if !matches {
			return false
		}
	}

	summaryMarkdown := build.GetSummaryMarkdown()
	if len(r.summaryMarkdownRe) > 0 {
		matches := matchesAny(summaryMarkdown, r.summaryMarkdownRe)
		if !matches {
			return false
		}
	}

	if r.rule.GetFailedCheckpoint() != pb.RetryStep_UNDEFINED {
		checkpointStatus := extractCheckpointStatus(build, r.rule.GetFailedCheckpoint())
		if checkpointStatus == nil {
			return false
		} else if *checkpointStatus != "FAILED" && *checkpointStatus != "STARTED" {
			// When builds fail particularly ungracefully they may not have a
			// chance to update the retry summary. Consider "STARTED" a failed
			// status (for a build that is no longer running).
			return false
		}
	}

	if r.rule.GetBeforeCheckpoint() != pb.RetryStep_UNDEFINED {
		checkpointStatus := extractCheckpointStatus(build, r.rule.GetBeforeCheckpoint())
		if checkpointStatus != nil {
			return false
		}
	}
	return true
}

// canRetry evaluates whether a retry is permitted by all matching rules.
func (c *CollectState) canRetry(build *bbpb.Build, originalBBID string) bool {
	buildName := build.GetBuilder().GetBuilder()
	currentTime := c.clock.Now()
	matchesRules := []string{}
	foundSufficient := false
	for i, rule := range c.rules {
		if !rule.matches(build) {
			continue
		}
		if !rule.rule.GetInsufficient() {
			foundSufficient = true
		}
		matchesRules = append(matchesRules, fmt.Sprintf("%d", i))
	}
	buildStr := fmt.Sprintf("(%s, %d, %s)", buildName, build.GetId(), build.GetStatus())
	if len(matchesRules) == 0 {
		c.LogOut("Build %s does not match any rules, not evaluating for retry", buildStr)
		return false
	}

	c.LogOut("Build %s matches rules %s, evaluating for retry", buildStr, strings.Join(matchesRules, ","))
	if !foundSufficient {
		c.LogOut("Build %s only matches insufficient rules, not retrying.", buildStr)
		return false
	}
	for i, rule := range c.rules {
		if !rule.matches(build) {
			continue
		}
		if rule.rule.GetMaxRetries() > 0 {
			if rule.totalRetries >= uint32(rule.rule.GetMaxRetries()) {
				c.LogOut("Rule %d has used %d/%d total retries, not retrying.",
					i, rule.totalRetries, rule.rule.GetMaxRetries())
				return false
			}
		}
		if rule.rule.GetMaxRetriesPerBuild() > 0 {
			buildRetries, ok := rule.retriesByBuild[originalBBID]
			if ok && buildRetries >= uint32(rule.rule.GetMaxRetriesPerBuild()) {
				c.LogOut("Rule %d has used %d/%d total retries for build %s, not retrying.",
					i, buildRetries, rule.rule.GetMaxRetriesPerBuild(), buildName)
				return false
			}
		}
		if rule.rule.GetCutoffSeconds() > 0 {
			if c.startTime+int64(rule.rule.GetCutoffSeconds()) < currentTime {
				c.LogOut("Rule %d only allows retries %d seconds into the collection"+
					" (we're at %d seconds), not retrying.",
					i, rule.rule.GetCutoffSeconds(), currentTime-c.startTime)
				return false
			}
		}
		if rule.rule.GetCutoffPercent() > 0 {
			if float32(rule.totalRetries)/float32(c.initialBuildCount) >= rule.rule.GetCutoffPercent() {
				c.LogOut("Rule %d will only retry %2.2f%% builds, not retrying.", i, rule.rule.GetCutoffPercent()*100)
				return false
			}
		}
		if rule.rule.GetBuildRuntimeCutoff() > 0 {
			if build.GetStartTime() == nil {
				c.LogOut("Build never started; build runtime cutoff does not apply.")
			} else {
				if build.GetEndTime() == nil {
					c.LogOut("Build doesn't have an end time recorded, not retrying.")
					return false
				}
				buildRuntime := build.GetEndTime().Seconds - build.GetStartTime().Seconds
				if buildRuntime > int64(rule.rule.GetBuildRuntimeCutoff()) {
					c.LogOut("Rule %d will only retry %d seconds into a build (we're at %d), not retrying.", i, rule.rule.GetBuildRuntimeCutoff(), buildRuntime)
					return false
				}
			}
		}

		if !rule.rule.GetInsufficient() {
			foundSufficient = true
		}
	}
	return true
}

// recordRetry records that the build was retried.
func (c *CollectState) recordRetry(build *bbpb.Build, originalBBID string) {
	for _, rule := range c.rules {
		if rule.matches(build) {
			rule.totalRetries++
			rule.retriesByBuild[originalBBID]++
		}
	}
}
