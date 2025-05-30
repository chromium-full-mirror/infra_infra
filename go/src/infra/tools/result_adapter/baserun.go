// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode"

	"github.com/armon/circbuf"
	"github.com/maruel/subcommands"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/luci/common/data/text"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/system/exitcode"
	"go.chromium.org/luci/common/system/signals"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/lucictx"
	sinkpb "go.chromium.org/luci/resultdb/sink/proto/v1"

	exceptionpb "go.chromium.org/infra/tools/result_adapter/proto"
)

// ExitCodeCommandFailure indicates that a given command failed due to internal errors
// or invalid input parameters.
const ExitCodeCommandFailure = 123

// baseRun provides common command run functionality.
type baseRun struct {
	subcommands.CommandRunBase

	// Flags.
	artifactDir                     string
	resultFile                      string
	testMetadataFile                string
	trimArtifactPrefix              string
	invocationLinkArtifact          string
	enableInvocationArtifactsUpload bool

	sinkCtx *lucictx.ResultSink
	sinkC   sinkpb.SinkClient

	// captureOutput indicates whether we should capture the complete output of the
	// test command we execute. If true, the data passed to the converter function
	// is guaranteed to contain the complete output of the test command, otherwise
	// it may contain only truncated subset, usable only for diagnostics. This option
	// is used by subcommands whose test commands do not write their results to a
	// file, like `go test`.
	//
	// This option is set by subcommands that embed baseRun.
	captureOutput bool
}

type converter func(ctx context.Context, data []byte) ([]*sinkpb.TestResult, error)

type StdoutBuffer interface {
	Write(data []byte) (int, error)
	Bytes() []byte
}

func (r *baseRun) RegisterGlobalFlags() {
	r.Flags.StringVar(&r.artifactDir, "artifact-directory", "", text.Doc(`
				Directory of the artifacts. Required.
			`))
	r.Flags.StringVar(&r.resultFile, "result-file", "", text.Doc(`
				Path to the result output file. Required.
			`))
	r.Flags.StringVar(&r.testMetadataFile, "test-metadata-file", "", text.Doc(`
				Path to the CFT test metadata output file. Optional.
			`))
	r.Flags.StringVar(&r.trimArtifactPrefix, "trim-artifact-prefix", "", text.Doc(`
				Artifact Path to be trimmed, e.g. absolute path inside a container. Optional.
			`))
	r.Flags.StringVar(&r.invocationLinkArtifact, "invocation-link-artifacts", "", text.Doc(`
				Name and URL pairs to insert as invocation level artifacts, e.g. stainless_logs=https://stainless/logs/1234,testhaus_logs=https://testhaus/logs/1234
				URLs must be url encoded, so '=', ',' and ' ' are not valid characters. Optional.
	`))
	r.Flags.BoolVar(&r.enableInvocationArtifactsUpload, "enable-invocation-artifacts-upload", false, text.Doc(`
				Whether to upload all invocation level artifacts to resultdb.
			`))
}

// validate validates the command has required flags.
func (r *baseRun) validate() (err error) {
	if r.resultFile == "" {
		return errors.Reason("-result-file is required").Err()
	}
	return nil
}

// initSinkClient initializes the result sink client.
func (r *baseRun) initSinkClient(ctx context.Context) (err error) {
	r.sinkCtx = lucictx.GetResultSink(ctx)
	if r.sinkCtx == nil {
		return errors.Reason("no result sink info found in $LUCI_CONTEXT").Err()
	}

	r.sinkC = sinkpb.NewSinkPRPCClient(&prpc.Client{
		Host:    r.sinkCtx.Address,
		Options: &prpc.Options{Insecure: true},
	})

	return nil
}

// runTestCmd waits for test cmd to complete.
func (r *baseRun) runTestCmd(ctx context.Context, args []string) (output []byte, err error) {

	// Subprocess exiting will unblock result_uploader and will stop soon.
	cmdCtx, cancelCmd := context.WithCancel(ctx)
	defer cancelCmd()
	defer signals.HandleInterrupt(func() {
		logging.Warningf(ctx, "result_uploader: interrupt signal received; killing the subprocess")
		cancelCmd()
	})()

	var stdoutBuf StdoutBuffer
	if r.captureOutput {
		stdoutBuf = new(bytes.Buffer)
	} else {
		// Create a ring buffer with size limited to last 10kb.
		stdoutBuf, _ = circbuf.NewBuffer(10_000)
	}
	cmd := exec.CommandContext(cmdCtx, args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	if r.captureOutput {
		cmd.Stdout = stdoutBuf
	} else {
		cmd.Stdout = io.MultiWriter(stdoutBuf, os.Stdout)
	}
	cmd.Stderr = os.Stderr

	// Launch the command w/o the result_sink section in lucictx, in case the test
	// framework has SinkAPI integrated. If a test binary was launched by result_adapter,
	// the test binary shouldn't be able to talk to the local SinkServer directly.
	exported, err := lucictx.Export(lucictx.SetResultSink(cmdCtx, nil))
	if err != nil {
		return nil, errors.Annotate(err, "failed to export a luci-context w/o result-sink").Err()
	}
	defer exported.Close()
	exported.SetInCmd(cmd)

	if err := cmd.Start(); err != nil {
		return nil, errors.Annotate(err, "cmd.start").Err()
	}

	err = cmd.Wait()
	return stdoutBuf.Bytes(), err
}

func (r *baseRun) done(err error) int {
	if err != nil {
		fmt.Fprintf(os.Stderr, "result_adapter: %s\n", err)
		return ExitCodeCommandFailure
	}
	return 0
}

func (r *baseRun) reportException(ctx context.Context, reportErr error, out []byte) {
	// Includes the stdout to stacktrace, as result_adapter exceptions are
	// generally caused by test execution errors.
	outString := strings.ToValidUTF8(string(out), string(unicode.ReplacementChar))
	stackTrace := strings.Split(outString+"\n"+errors.RenderStack(reportErr), "\n")
	exceptions := &exceptionpb.ExceptionOccurrences{
		Datapoints: []*exceptionpb.ExceptionOccurrence{
			{
				Name:         reportErr.Error(),
				Stacktrace:   stackTrace,
				OccurredTime: timestamppb.New(time.Now()),
			},
		},
	}
	// Convert exceptions to jsonpb
	exceptionsAnypb, err := anypb.New(exceptions)
	if err != nil {
		logging.Warningf(ctx, "Warning: failed to construct exceptions report.")
		return
	}

	jsonpb, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(exceptionsAnypb)
	if err != nil {
		logging.Warningf(ctx, "Warning: failed to convert exceptions as jsonpb.")
		return
	}
	exceptionsStruct := new(structpb.Struct)
	if err = exceptionsStruct.UnmarshalJSON(jsonpb); err != nil {
		logging.Warningf(ctx, "Warning: failed to construct exceptions struct.")
		return
	}

	// We are doing an overwrite for extended_properties.exception_occurrences.
	// However, we are not expecting exception_occurrences reported from multiple
	// different sources at the same time in the Chromium infra. That it should be
	// good to ignore the potential overwrite behavior for now.
	if _, err := r.sinkC.UpdateInvocation(ctx, &sinkpb.UpdateInvocationRequest{
		Invocation: &sinkpb.Invocation{
			ExtendedProperties: map[string]*structpb.Struct{
				"exception_occurrences": exceptionsStruct,
			},
		},
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: []string{"extended_properties.exception_occurrences"},
		},
	}); err != nil {
		logging.Warningf(ctx, "Warning: failed to report converter exceptions.")
	}
}

func (r *baseRun) run(ctx context.Context, args []string, f converter) (ret int) {
	if err := r.initSinkClient(ctx); err != nil {
		return r.done(err)
	}

	out, err := r.runTestCmd(ctx, args)
	ec, ok := exitcode.Get(err)
	if !ok {
		return r.done(errors.Annotate(err, "test command failed").Err())
	}

	// Setup auth header for ResultSink before potential uploads.
	ctx = metadata.AppendToOutgoingContext(ctx, "Authorization", "ResultSink "+r.sinkCtx.AuthToken)

	trs, err := f(ctx, out)
	switch {
	case err != nil:
		r.reportException(ctx, err, out)
		return r.done(err)
	case len(trs) == 0:
		return ec
	}

	// Try to upload invocation link artifacts.
	// Upload before test results so that the links are present even if something goes wrong in the test result upload.
	// We do not abort on error here, as we still want to upload the test results even if we can't upload the links.
	linkArtifacts := map[string]*sinkpb.Artifact{}
	for _, pair := range strings.Split(r.invocationLinkArtifact, ",") {
		name, url, found := strings.Cut(pair, "=")
		if !found {
			logging.Warningf(ctx, "Warning: no '=' in invocation-link-artifacts pair: %q, ignoring", pair)
			continue
		}
		linkArtifacts[name] = &sinkpb.Artifact{
			Body:        &sinkpb.Artifact_Contents{Contents: []byte(url)},
			ContentType: "text/x-uri",
		}
	}
	if _, err := r.sinkC.ReportInvocationLevelArtifacts(ctx, &sinkpb.ReportInvocationLevelArtifactsRequest{Artifacts: linkArtifacts}); err != nil {
		logging.Warningf(ctx, "Warning: failed to upload invocation level link artifacts: %q, err: %v", r.invocationLinkArtifact, err)
	}

	// Upload invocations level artifacts
	if r.enableInvocationArtifactsUpload {
		invArtifacts, err := invocationArtifacts(r.artifactDir, trs)
		logging.Infof(ctx, "Info: Uploading %d invocation level artifacts to resultdb from dir: %q", len(invArtifacts), r.artifactDir)
		if err != nil {
			logging.Warningf(ctx, "Warning: failed to prepare invocation level artifacts from dir: %q, err: %v", r.artifactDir, err)
		} else {
			if _, err := r.sinkC.ReportInvocationLevelArtifacts(ctx, &sinkpb.ReportInvocationLevelArtifactsRequest{Artifacts: invArtifacts}); err != nil {
				logging.Warningf(ctx, "Warning: failed to upload invocation level artifacts from dir: %q, err: %v", r.artifactDir, err)
			}
		}
	}

	if _, err := r.sinkC.ReportTestResults(ctx, &sinkpb.ReportTestResultsRequest{TestResults: trs}); err != nil {
		return r.done(err)
	}
	return ec
}
