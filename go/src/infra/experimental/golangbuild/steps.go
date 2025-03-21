// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/luciexe/build"
	resultpb "go.chromium.org/luci/resultdb/proto/v1"
	sinkpb "go.chromium.org/luci/resultdb/sink/proto/v1"
)

// cmdStepRun calls Run on the provided command and wraps it in a build step.
//
// It overwrites cmd.Stdout and cmd.Stderr to redirect into step logs.
// It runs the command with the environment from the context, so change
// the context's environment to alter the command's environment.
func cmdStepRun(ctx context.Context, stepName string, cmd *exec.Cmd, infra bool, logExtraFiles ...string) (err error) {
	step, ctx, startStepErr := cmdStartStep(ctx, stepName, cmd)
	defer func() {
		if infra {
			err = infraWrap(err) // Failure is deemed to be an infrastructure failure.
		}
		step.End(err)
	}()
	if startStepErr != nil {
		return startStepErr
	}

	// Combine output because it's annoying to pick one of stdout and stderr
	// in the UI and be wrong.
	output := step.Log("output")
	cmd.Stdout = output
	cmd.Stderr = output

	// Run the command.
	cmdErr := cmd.Run()
	if cmdErr != nil {
		cmdErr = fmt.Errorf("failed to run %s: %w", stepName, cmdErr)
		cmdErr = attachLinks(cmdErr, fmt.Sprintf("%s output", stepName), output.UILink())
	}

	// Log extra files.
	for _, filename := range logExtraFiles {
		name := filepath.Base(filename)
		f, err := os.Open(filename)
		if err != nil {
			log := step.Log("file: " + name + " (error)")
			_, _ = io.WriteString(log, err.Error())
			continue
		}
		log := step.Log("file: " + name)
		if _, err := io.Copy(log, f); err != nil {
			log := step.Log("file: " + name + " (error)")
			_, _ = io.WriteString(log, err.Error())
		}
		_ = f.Close()
	}
	return cmdErr
}

// cmdStepOutput calls Output on the provided command and wraps it in a build step.
//
// It overwrites cmd.Stdout and cmd.Stderr to redirect into step logs.
// It runs the command with the environment from the context, so change
// the context's environment to alter the command's environment.
func cmdStepOutput(ctx context.Context, stepName string, cmd *exec.Cmd, infra bool) (output []byte, err error) {
	step, ctx, startStepErr := cmdStartStep(ctx, stepName, cmd)
	defer func() {
		if infra {
			err = infraWrap(err) // Failure is deemed to be an infrastructure failure.
		}
		step.End(err)
	}()
	if startStepErr != nil {
		return nil, startStepErr
	}

	// Make sure we log stderr.
	stderr := step.Log("stderr")
	cmd.Stderr = stderr

	// Run the command and capture stdout.
	output, err = cmd.Output()

	// Log stdout before we do anything else.
	stdout := step.Log("stdout")
	stdout.Write(output)

	// Check for errors.
	if err != nil {
		err = fmt.Errorf("failed to run %s: %w", stepName, err)
		err = attachLinks(err,
			fmt.Sprintf("%s stdout", stepName), stdout.UILink(),
			fmt.Sprintf("%s stderr", stepName), stderr.UILink(),
		)
		return output, err
	}
	return output, nil
}

// cmdStepTest calls CombinedOutput on the provided command, wraps it in a build step, and uploads
// the command result as a test result to ResultDB.
//
// It overwrites cmd.Stdout and cmd.Stderr to redirect into step logs.
// It runs the command with the environment from the context, so change
// the context's environment to alter the command's environment.
func cmdStepTest(ctx context.Context, spec *buildSpec, stepName, testID string, cmd *exec.Cmd) (err error) {
	step, ctx, startStepErr := cmdStartStep(ctx, stepName, cmd)
	defer func() {
		step.End(err)
	}()
	if startStepErr != nil {
		return startStepErr
	}

	// Run the command and capture the output.
	start := time.Now()
	output, cmdErr := cmd.CombinedOutput()
	dur := time.Since(start)

	// Log the combined before we do anything else.
	log := step.Log("output")
	_, _ = log.Write(output)

	// Spruce up the error.
	if cmdErr != nil {
		cmdErr = fmt.Errorf("failed to run %s: %w", stepName, cmdErr)
		cmdErr = attachTestsFailed(cmdErr)
		cmdErr = attachLinks(cmdErr,
			fmt.Sprintf("%s output", stepName), log.UILink(),
		)
	}

	// Set up a test result in a file.
	status := resultpb.TestStatus_PASS
	if cmdErr != nil {
		status = resultpb.TestStatus_FAIL
	}
	tr := &sinkpb.TestResult{
		TestId:    testID,
		Expected:  cmdErr == nil,
		Status:    status,
		StartTime: timestamppb.New(start),
		Duration:  durationpb.New(dur),
		Artifacts: map[string]*sinkpb.Artifact{
			"output": {Body: &sinkpb.Artifact_Contents{Contents: output}},
		},
		SummaryHtml: `<p><text-artifact artifact-id="output"></p>`,
	}
	trMsg, err := protojson.Marshal(tr)
	if err != nil {
		log := step.Log("test result marshalling error")
		_, _ = io.WriteString(log, err.Error())
		return cmdErr
	}
	resultFile, err := writeTempFile(fmt.Sprintf("%s-test-result-", stepName), trMsg)
	if err != nil {
		log := step.Log("result file creation error")
		_, _ = io.WriteString(log, err.Error())
		return cmdErr
	}

	// Send off the test result.
	//
	// Note: This seems really roundabout, but there isn't an easier way to just send
	// tests results through the Sink API which is much nicer to work with than the
	// ResultDB API directly. We could set up a sink server and talk to ourselves,
	// but that requires a ton of boilerplate (most of which is basically what 'rdb stream'
	// already does).
	//
	// TODO(mknyszek): This is actually really gross. The only alternative I can think
	// of is to provide a mode to result_adapter to not actually require a command, or
	// to add a mode to rdb to directly ingest results from a file.

	trArgs := []string{"stream"}
	trArgs = append(trArgs, spec.rdbStreamArgs(ctx)...)
	trArgs = append(trArgs, toolPath(ctx, "result_adapter"), "native", "-result-file", resultFile, "--")
	// We need *any* dummy command here. Let's pick something we know we have and that we know won't do anything bad.
	// TODO(mknyszek): Do something better here. Ideally we'd just "echo 'This is a dummy command.'" or something,
	// but I'm not actually sure how to do that portably.
	trArgs = append(trArgs, toolPath(ctx, "rdb"), "-help")
	trCmd := exec.Command(toolPath(ctx, "rdb"), trArgs...)

	// N.B. Do not propagate any errors produced in trying to send the test off.
	// The step will still appear as failed in the UI, but it won't abort the
	// whole run, which is preferable.
	_ = cmdStepRun(ctx, fmt.Sprintf("upload test result for %s", stepName), trCmd, true)

	return cmdErr
}

// cmdStartStep sets up a command step.
//
// It overwrites cmd.Stdout and cmd.Stderr to redirect into step logs.
// It runs the command with the environment from the context, so change
// the context's environment to alter the command's environment.
func cmdStartStep(ctx context.Context, stepName string, cmd *exec.Cmd) (*build.Step, context.Context, error) {
	step, ctx := build.StartStep(ctx, stepName)

	// Log the full command we're executing.
	//
	// Put each env var on its own line to actually make this readable.
	envs := cmd.Env
	if envs == nil {
		envs = os.Environ()
	}
	var fullCmd bytes.Buffer
	for _, env := range envs {
		fullCmd.WriteString(env)
		fullCmd.WriteString("\n")
	}
	if cmd.Dir != "" {
		fullCmd.WriteString("PWD=")
		fullCmd.WriteString(cmd.Dir)
		fullCmd.WriteString("\n")
	}
	fullCmd.WriteString(cmd.String())
	if _, err := io.Copy(step.Log("command"), &fullCmd); err != nil {
		return step, ctx, infraWrap(err)
	}
	return step, ctx, nil
}

// infraErrorf formats according to a format specifier and
// returns an error with INFRA_FAILURE status attached.
func infraErrorf(format string, args ...any) error {
	return build.AttachStatus(fmt.Errorf(format, args...), bbpb.Status_INFRA_FAILURE, nil)
}

// infraWrap attaches an INFRA_FAILURE status to err and returns the result.
// This is a no-op if the error is nil.
func infraWrap(err error) error {
	return build.AttachStatus(err, bbpb.Status_INFRA_FAILURE, nil)
}

// endStep calls step.End with *errp.
// errp must be non-nil, but *errp may be a nil error.
func endStep(step *build.Step, errp *error) {
	step.End(*errp)
}

// endInfraStep modifies *errp to attach an INFRA_FAILURE status
// and calls step.End with *errp.
// errp must be non-nil, but *errp may be a nil error.
func endInfraStep(step *build.Step, errp *error) {
	*errp = infraWrap(*errp) // Any failure is an infrastructure failure.
	step.End(*errp)
}

// attachLinks attaches name/url pairs as links to the error.
//
// These can later be retrieved via extractLinks.
//
// Passing a nil error will result in no links attached, and
// will return another nil error.
//
// TODO(mknyszek): Consider producing a non-nil error anyway
// to avoid losing information and also to catch bugs where
// we attach information to a nil error. The downside of doing
// so is that we might end up accidentally producing a non-nil
// error for a nil error, causing a spurious failure.
func attachLinks(err error, links ...string) error {
	if len(links)%2 != 0 {
		panic("attachLinks requires name/URL pairs")
	}
	if err == nil {
		return err
	}
	el, ok := err.(*errLinks)
	if !ok {
		el = &errLinks{err: err}
	}
	for i := 0; i < len(links); i += 2 {
		el.links = append(el.links, link{
			name: links[i+0],
			url:  links[i+1],
		})
	}
	return el
}

// extractLinks aggregates the links of all errLinks in the
// error chain.
//
// Accepts a nil error, but returns no links.
func extractLinks(err error) []link {
	// Walk the error chain and extract links.
	e := err
	var links []link
	for e != nil {
		// Check if there are any links to unwrap.
		if el, ok := e.(*errLinks); ok {
			links = append(links, el.links...)
		}

		// Walk errors.Join errors.
		w, ok := e.(interface{ Unwrap() []error })
		if ok {
			for _, err := range w.Unwrap() {
				links = append(links, extractLinks(err)...)
			}
			break
		}

		// Otherwise, just try to unwrap.
		e = errors.Unwrap(e)
	}
	return links
}

// errLinks is an error with arbitrary links (name/URL pairs) attached.
// *errLinks implements error. *errLinks is unwrappable.
type errLinks struct {
	err   error // Must be non-nil.
	links []link
}

func (e *errLinks) Error() string {
	return e.err.Error()
}

func (e *errLinks) Unwrap() error {
	return e.err
}

// link is a hyperlink: a URL wth a name.
type link struct {
	name, url string
}

// attachTestsFailed marks the error as having failing tests.
//
// Accepts a nil error, but also returns a nil error in that case.
func attachTestsFailed(err error) error {
	if err == nil || errorTestsFailed(err) {
		return err
	}
	return &errTestsFailed{err}
}

// errorTestsFailed reports whether the error contains an errTestsFailed marker in its chain.
func errorTestsFailed(err error) bool {
	e := err
	for e != nil {
		// Check if there are any links to unwrap.
		if _, ok := e.(*errTestsFailed); ok {
			return true
		}

		// Walk errors.Join errors.
		w, ok := e.(interface{ Unwrap() []error })
		if ok {
			for _, err := range w.Unwrap() {
				if errorTestsFailed(err) {
					return true
				}
			}
			break
		}

		// Otherwise, just try to unwrap.
		e = errors.Unwrap(e)
	}
	return false
}

// errTestsFailed is an error that marks that tests failed.
// *errTestsFailed implements error. *errTestsFailed is unwrappable.
type errTestsFailed struct {
	err error // Must be non-nil
}

func (e *errTestsFailed) Error() string {
	return e.err.Error()
}

func (e *errTestsFailed) Unwrap() error {
	return e.err
}

type topLevelLogger struct {
	state *build.State
	links []link
}

// withTopLevelLogger installs a topLevelLogger in a new context.Context based on ctx.
func withTopLevelLogger(ctx context.Context, st *build.State) context.Context {
	return context.WithValue(ctx, topLevelLoggerKey{}, &topLevelLogger{state: st})
}

type topLevelLoggerKey struct{}

// topLevelLog creates a new top-level log entry and registers it with the topLevelLogger
// in ctx.
func topLevelLog(ctx context.Context, name string) *build.Log {
	logger, _ := ctx.Value(topLevelLoggerKey{}).(*topLevelLogger)
	if logger == nil {
		panic("topLevelLog called without topLevelLogger in context")
	}
	log := logger.state.Log(name)
	logger.links = append(logger.links, link{name: name, url: log.UILink()})
	return log
}

// topLevelLogLinks returns UI links to all the top-level logs that have been accumulated in ctx.
func topLevelLogLinks(ctx context.Context) []link {
	logger, _ := ctx.Value(topLevelLoggerKey{}).(*topLevelLogger)
	if logger == nil {
		panic("topLevelLog called without topLevelLogger in context")
	}
	return logger.links
}

func writeTempFile(pattern string, data []byte) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	name := f.Name()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(name)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}
