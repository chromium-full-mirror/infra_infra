// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package build provides a way to update the buildbucket Build proto during
// execution.
package build

import (
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cmd/cros_test_platform/internal/execution/testrunner"
)

// RequestStepUpdater provides methods to update a step corresponding to the
// execution of a request.
type RequestStepUpdater struct {
	build       *bbpb.Build
	step        *bbpb.Step
	invocations []*InvocationStepUpdater
	finalized   bool
}

// NewRequestStep creates a new step corresponding to a request.
//
// NewRequestStep returns a RequestStepUpdater that may be used to update this
// step as the execution proceeds.
func NewRequestStep(name string, build *bbpb.Build) *RequestStepUpdater {
	return &RequestStepUpdater{
		build: build,
		step:  appendNewStep(build, fmt.Sprintf("request %s", name)),
	}
}

// NewInvocationStep creates a new step for an invocation that is part of this
// request.
//
// NewInvocationStep returns an InvocationStepUpdater that may be used to update
// this invocation as the execution proceeds.
func (r *RequestStepUpdater) NewInvocationStep(name string) *InvocationStepUpdater {
	s := &InvocationStepUpdater{
		step: appendNewStep(r.build, fmt.Sprintf("%s|invocation %d. %s", r.step.Name, 1+len(r.invocations), name)),
	}
	r.invocations = append(r.invocations, s)
	return s
}

// Close updates the build to reflect that the execution of this request is
// complete.
//
// RequestStepUpdater should not be used once Close() has been called.
func (r *RequestStepUpdater) Close(status bbpb.Status, summaryMarkdown string) error {
	if r.finalized {
		return errors.New("RequestStepUpdater: finalized called more than once")
	}
	for _, i := range r.invocations {
		if !i.closed {
			i.close(status)
		}
	}
	closeStep(r.step, status)

	if summaryMarkdown != "" {
		r.step.SummaryMarkdown = summaryMarkdown + "\n"
	}
	r.finalized = true
	return nil
}
func (r *RequestStepUpdater) DisplayExceptionExpiredSummary(expirationDate time.Time) {
	r.step.SummaryMarkdown = fmt.Sprintf("SuiteLimits exception expired on %s", expirationDate.UTC().Format(time.RFC822))
}

func (r *RequestStepUpdater) DisplayExceptionSummary(expirationDate time.Time) {
	r.step.SummaryMarkdown = fmt.Sprintf("SuiteLimits Exception granted. Expires %s", expirationDate.UTC().Format(time.RFC822))
}

// InvocationStepUpdater provides methods to update a step corresponding to the
// execution of an invocation.
type InvocationStepUpdater struct {
	step   *bbpb.Step
	tasks  []*testrunner.Build
	closed bool
}

// NotifyNewTask notifies the InvocationStepUpdater of the creation of a new
// task for an invocation.
func (i *InvocationStepUpdater) NotifyNewTask(task *testrunner.Build) {
	i.tasks = append(i.tasks, task)
	i.step.SummaryMarkdown = i.summary()
}

// MarkCompleted closes the invocation step.
func (i *InvocationStepUpdater) MarkCompleted() {
	i.close(bbpb.Status_SUCCESS)
}

const (
	// Include a leading newline to separate from the step name.
	latestAttemptTemplate     = "*    [latest attempt](%s)"
	previousAttemptsTemplate  = "*    previous failed attempts: %s"
	ExceededExecutionTimeText = "*    SUITE EXECUTION TIME LIMIT EXCEEDED: go/suitelimits-faqs"
)

func (i *InvocationStepUpdater) AddCancelledSummary() {
	i.step.SummaryMarkdown = ExceededExecutionTimeText + "\n" + i.step.SummaryMarkdown
	i.close(bbpb.Status_FAILURE)
}

func (i *InvocationStepUpdater) summary() string {
	ts := i.tasks
	if len(ts) == 0 {
		return "No tasks created"
	}
	s := []string{fmt.Sprintf(latestAttemptTemplate, ts[len(ts)-1].TaskURL())}
	ts = ts[0 : len(ts)-1]
	if len(ts) > 0 {
		ls := make([]string, len(ts))
		for c, t := range ts {
			ls[c] = fmt.Sprintf("[%d](%s)", c+1, t.TaskURL())
		}
		s = append(s, fmt.Sprintf(previousAttemptsTemplate, strings.Join(ls, ", ")))
	}
	return strings.Join(s, "\n")
}

func (i *InvocationStepUpdater) close(status bbpb.Status) {
	i.closed = true
	closeStep(i.step, status)
}

func appendNewStep(build *bbpb.Build, name string) *bbpb.Step {
	step := &bbpb.Step{
		Name:      name,
		Status:    bbpb.Status_STARTED,
		StartTime: timestamppb.New(time.Now()),
	}
	build.Steps = append(build.Steps, step)
	return step
}

func closeStep(s *bbpb.Step, status bbpb.Status) {
	s.EndTime = timestamppb.New(time.Now())
	s.Status = status
}
