// Copyright 2021 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	resultpb "go.chromium.org/luci/resultdb/proto/v1"
	sinkpb "go.chromium.org/luci/resultdb/sink/proto/v1"
)

// ensureArgsValid checks the payload to ensure it looks like a `go test`
// invocation and that it has the `-json` flag.
func (r *goRun) ensureArgsValid(args []string) ([]string, error) {
	// Scan the arguments.
	var (
		testArgIndex = -1
		jsonFlagSeen = false
	)
	for i, t := range args {
		switch t {
		case "test":
			testArgIndex = i
		case "-json":
			jsonFlagSeen = true
		}
	}
	if testArgIndex != -1 && !jsonFlagSeen {
		// If there's a 'test' argument and no '-json' flag, automatically add one.
		args = append(args[:testArgIndex+1], append([]string{"-json"}, args[testArgIndex+1:]...)...)
		return args, nil
	}
	if cmd := strings.Join(args, " "); !strings.Contains(cmd, "test") && !strings.Contains(cmd, "json") {
		// Since the command line doesn't even mention "test" nor "json", probably safe enough to fail here.
		return nil, errors.Reason("Expected command to be an invocation of `go test -json` or equivalent: %q", args).Err()
	}
	// Otherwise it might be something that will emit JSON events in https://go.dev/cmd/test2json format,
	// such as 'GOROOT/src/run.bash -json' or 'sh -c "something && go test -json ./..."'.
	//
	// Use the arguments as is and let it fail at a higher level if the caller made a mistake.
	return args, nil
}

func (r *goRun) generateTestResults(ctx context.Context, data []byte) ([]*sinkpb.TestResult, error) {
	if r.DumpJSONFile != "" {
		if err := os.WriteFile(r.DumpJSONFile, data, 0o666); err != nil {
			return nil, errors.Annotate(err, "dumping Go test JSON to a file").Err()
		}
	}
	ordered := goTestJSONToPackageRecords(ctx, data, r.CopyTestOutput, r.VerboseTestOutput)

	ret := make([]*sinkpb.TestResult, 0, 8)
	for _, record := range ordered {
		ret = append(ret, record.toTestProtos(ctx)...)
	}
	return ret, nil
}

// goTestJSONToPackageRecords parses one line at a time from the given output,
// which is expected to be the one produced by `go test -json <package>`.
// It converts each line to GoTestEvent and ingests it into a PackageRecord.
// copyTestOutput optionally specifies where to write a copy of test output.
// The resulting PackageRecord(s) are returned to the caller as a slice in the
// same order as they were initially seen.
func goTestJSONToPackageRecords(ctx context.Context, data []byte, copyTestOutput io.Writer, verboseTestOutput bool) []*PackageRecord {
	var ordered []*PackageRecord
	byID := make(map[string]*PackageRecord)       // Map key is the 'Package' field.
	buildRecords := make(map[string]*BuildRecord) // Map key is the 'ImportPath' field.

	// Set up the test renderer, which will render the go test -json events
	// to copyTestOutput.
	var rn *GoTestRenderer
	var renderFailed bool
	if copyTestOutput != nil {
		rn = NewGoTestRenderer(copyTestOutput, verboseTestOutput)
		defer func() {
			if err := rn.Close(renderFailed); err != nil {
				logging.Warningf(ctx, "failed to finish test output rendering: %v", err)
			}
		}()
	}

	// Ensure that the scanner below returns the last line in the output.
	if !bytes.HasSuffix(data, []byte{'\n'}) {
		data = append(data, '\n')
	}
	lines := bufio.NewScanner(bytes.NewReader(data))
	// Iterate over output, parsing an event from each line and making the
	// appropriate record ingest it.
	for lines.Scan() {
		l := lines.Bytes()
		if len(l) == 0 {
			continue
		}
		tEvt, err := parseRow(l)
		if err != nil {
			logging.Warningf(ctx, "cannot parse row %q, %s", string(l), err)
			continue
		}
		if tEvt.IsBuildEvent() {
			br := buildRecords[tEvt.ImportPath]
			if br == nil {
				br = new(BuildRecord)
				buildRecords[tEvt.ImportPath] = br
			}
			br.ingest(tEvt)
		} else {
			pr := byID[tEvt.Package]
			if pr == nil {
				pr = &PackageRecord{
					PackageName: tEvt.Package,
					TestsByName: make(map[string]*TestRecord),
				}
				ordered = append(ordered, pr)
				byID[pr.PackageName] = pr
			}
			pr.ingest(tEvt, buildRecords)
		}

		// Pass events to the renderer, if available. If we ever fail to render,
		// stop rendering, otherwise we'll likely be emitting a lot of error lines
		// for no reason.
		if rn != nil && !renderFailed {
			if err := rn.Ingest(tEvt); err != nil {
				logging.Warningf(ctx, "failed to render test output: %v", err)
				renderFailed = true
			}
		}
	}

	// By now, records in the buildRecords map have likely been reported
	// as part of test events that had a corresponding FailedBuild field.
	// However, in some cases there may be build events without a corresponding
	// test event (for example, if there are build warnings and no test failure).
	//
	// So, if we get this far and there are unreported build records, add them
	// to the closest fit package record we find, creating new package records
	// if there's no exact match.
	for importPath, br := range buildRecords {
		if br.Reported {
			// Already reported, nothing to do.
			continue
		}

		pr := byID[importPath]
		if pr == nil {
			pr = &PackageRecord{
				PackageName: importPath,
				TestsByName: make(map[string]*TestRecord),
			}
			ordered = append(ordered, pr)
			byID[pr.PackageName] = pr

			// By default, treat build-output with no build-fail
			// as a passing result. This corresponds to behavior
			// like compiler warnings.
			pr.Result = "pass"
		}
		if pr.Result == "pass" && br.Fail {
			// If the package record was passing but we observed
			// a build-fail event, change it to a failing result.
			pr.Result = "fail"
		}
		pr.Output.WriteString(br.Output.String())
	}

	return ordered
}

func parseRow(s []byte) (*GoTestEvent, error) {
	new := &GoTestEvent{}
	return new, json.Unmarshal(s, new)
}

// GoTestEvent represents each JSON object produced by `go test -json`.
//
// It corresponds to either the TestEvent struct, or
// the BuildEvent struct; IsBuildEvent reports which.
//
// See https://go.dev/cmd/test2json#hdr-Output_Format
// and https://go.dev/cmd/go#hdr-Build__json_encoding.
type GoTestEvent struct {
	Time    time.Time // encodes as an RFC3339-format string
	Action  string
	Package string
	Test    string
	Elapsed float64 // seconds
	Output  string

	// FailedBuild is set for Action == "fail" if the test failure was caused
	// by a build failure. It contains the package ID of the package that failed
	// to build. This matches the ImportPath field of the "go list" output,
	// as well as the BuildEvent.ImportPath field as emitted by "go build -json".
	//
	// This field is populated in Go 1.24 and newer, unless the user opts out of
	// the new behavior using the GODEBUG setting gotestjsonbuildtext.
	// See https://go.dev/doc/go1.24#go-command.
	FailedBuild string

	// ImportPath holds the package ID of the package being built.
	// This field exists only in BuildEvent objects.
	ImportPath string
}

// IsBuildEvent reports whether this event is a test event, or build event.
//
// As of Go 1.24, the two possible build events are build-output and build-fail.
// More build events are not expected to be added soon, but if so, they'll also
// have a "build-" prefix.
func (e *GoTestEvent) IsBuildEvent() bool { return strings.HasPrefix(e.Action, "build-") }

// PackageRecord represents the results of a single package.
type PackageRecord struct {
	PackageName string // Import path of the go package.
	Result      string // Out of a subset of the values for GoTestEvent.Action as applicable.
	Started     time.Time
	Elapsed     float64         // seconds
	Output      strings.Builder // Output for the package, excluding output attributed to individual tests.

	TestsByName  map[string]*TestRecord
	OrderedTests []*TestRecord
}

// TestRecord represents the results of a single test.
// Several test events will apply to each TestRecord.
type TestRecord struct {
	TestName    string // TestName is the name of the test within the package.
	PackageName string // Import path of the Go package that the test is a part of.
	Result      string // Out of a subset of the values for GoTestEvent.Action as applicable.
	Started     time.Time
	Elapsed     float64 // seconds
	Output      strings.Builder
}

// ingest updates the fields of the package record according to the contents of
// the given test event.
//
// Whenever a test event has a non-empty FailedBuild field, ingest consumes the
// corresponding BuildRecord and deletes it from buildRecords.
func (pr *PackageRecord) ingest(te *GoTestEvent, buildRecords map[string]*BuildRecord) {
	if te.Test == "" {
		switch te.Action {
		// Action string values from https://go.dev/cmd/test2json.
		case "start":
			pr.Started = te.Time
		case "output":
			pr.Output.WriteString(te.Output)
		case "pass", "bench", "fail", "skip":
			pr.Result = te.Action
			if te.Elapsed > 0 {
				pr.Elapsed = te.Elapsed
			}
		case "pause", "cont":
			// Ignore.
		default:
			// Ignore.
		}

		// Include information about the build failure that caused this test failure.
		if te.FailedBuild != "" {
			br, ok := buildRecords[te.FailedBuild]
			if !ok {
				// This should not happen under normal circumstances.
				// Report some details about it if it happens anyway.
				br = new(BuildRecord)
				fmt.Fprintf(&br.Output, "unexpected case: encountered a TestEvent with FailedBuild field %q, but no corresponding BuildEvent\n", te.FailedBuild)
			}
			pr.Output.WriteString(br.Output.String())
			br.Reported = true
		}

	} else {
		// Record for a specific test.
		testRecord := pr.TestsByName[te.Test]
		if testRecord == nil {
			testRecord = &TestRecord{TestName: te.Test}
			pr.OrderedTests = append(pr.OrderedTests, testRecord)
			pr.TestsByName[testRecord.TestName] = testRecord
		}
		testRecord.ingest(te)
	}
}

func (pr *PackageRecord) toTestProtos(ctx context.Context) []*sinkpb.TestResult {
	anyTestFailed := false
	for _, tr := range pr.OrderedTests {
		if tr.Result == "fail" {
			anyTestFailed = true
		}
	}

	// Include a result for the package, which relates to the package
	// setup/teardown that is not covered by the individual test results.
	packageResult := &sinkpb.TestResult{}
	packageResult.TestId = pr.PackageName
	switch pr.Result {
	case "pass", "bench":
		packageResult.Status = resultpb.TestStatus_PASS
		packageResult.Expected = true
	case "fail":
		if anyTestFailed {
			// Package may only be reporting fail because one of the tests failed.
			// Report 'skip' to signify we don't know whether the package setup/teardown
			// passed or failed.
			// This is different to how go represents package failures but is intended
			// to avoid creating two 'failing' results in response to one test failure.
			packageResult.Status = resultpb.TestStatus_SKIP
			packageResult.Expected = true
		} else {
			// Package failed in setup/teardown.
			packageResult.Status = resultpb.TestStatus_FAIL
		}
	case "skip":
		packageResult.Status = resultpb.TestStatus_SKIP
		packageResult.Expected = true
	case "":
		// A test interrupted by SIGTERM, SIGABORT, SIGKILL will usually
		// have its status unset.
		packageResult.Status = resultpb.TestStatus_ABORT
	}

	if pr.Output.Len() > 0 {
		a := sinkpb.Artifact{}
		a.Body = &sinkpb.Artifact_Contents{
			Contents: []byte(pr.Output.String()),
		}
		packageResult.Artifacts = map[string]*sinkpb.Artifact{"output": &a}
		packageResult.SummaryHtml = `<p>Result only captures package setup and teardown. Tests within the package have their own result.</p>` +
			`<p><text-artifact artifact-id="output"></p>`
	}
	packageResult.Duration = durationpb.New(time.Duration(int64(pr.Elapsed * float64(time.Second))))
	if pr.Started != (time.Time{}) {
		packageResult.StartTime = timestamppb.New(pr.Started)
	}

	results := make([]*sinkpb.TestResult, 0, len(pr.OrderedTests)+1)
	results = append(results, packageResult)

	packagePassed := pr.Result == "pass" || pr.Result == "bench"
	for _, tr := range pr.OrderedTests {
		results = append(results, tr.toProto(ctx, packagePassed))
	}
	return results
}

// ingest updates the fields of the test record according to the contents of
// the given test event.
// Output events from a specific test will be associated with the corresponding
// test record.
// Tests running in parallel that choose not to use t.Log/t.Error and instead
// write to stdout/stderr directly may result in output being associated with
// the wrong test. See https://go.dev/issue/23036#issuecomment-355669573.
func (tr *TestRecord) ingest(te *GoTestEvent) {
	if tr.PackageName == "" {
		tr.PackageName = te.Package
	}

	switch te.Action {
	// Action string values from https://go.dev/cmd/test2json.
	case "run":
		tr.Started = te.Time
	case "output":
		tr.Output.WriteString(te.Output)
	case "pass", "bench", "fail", "skip":
		tr.Result = te.Action
		if te.Elapsed > 0 {
			tr.Elapsed = te.Elapsed
		}
	case "pause", "cont":
		// Ignore.
	default:
		// Ignore.
	}
}

func (tr *TestRecord) toProto(ctx context.Context, packagePassed bool) *sinkpb.TestResult {
	result := &sinkpb.TestResult{}

	testID := fmt.Sprintf("%s.%s", tr.PackageName, tr.TestName)
	result.TestId = testID

	switch tr.Result {
	case "pass", "bench":
		result.Status = resultpb.TestStatus_PASS
		result.Expected = true
	case "fail":
		result.Status = resultpb.TestStatus_FAIL
	case "skip":
		result.Status = resultpb.TestStatus_SKIP
		result.Expected = true
	case "":
		// It has been observed that test2json may fail to parse the status
		// of a test when multiple tests run in parallel in the same package
		// and produce certain output (such as goconvey output). In those
		// cases it's okay to mark the tests as passing if the whole
		// package passed.
		if packagePassed {
			logging.Warningf(ctx,
				"Status for test %s is missing from the list of test events. Setting to `pass` because package passed.", testID)
			result.Status = resultpb.TestStatus_PASS
			result.Expected = true
		} else {
			// A test interrupted by SIGTERM, SIGABORT, SIGKILL will usually
			// have its status unset.
			result.Status = resultpb.TestStatus_ABORT
		}
	}
	if tr.Output.Len() > 0 {
		a := sinkpb.Artifact{}
		a.Body = &sinkpb.Artifact_Contents{
			Contents: []byte(tr.Output.String()),
		}
		result.Artifacts = map[string]*sinkpb.Artifact{"output": &a}
		result.SummaryHtml = `<p><text-artifact artifact-id="output"></p>`
	}
	result.Duration = durationpb.New(time.Duration(int64(tr.Elapsed * float64(time.Second))))
	result.StartTime = timestamppb.New(tr.Started)
	return result
}

// BuildRecord represents the results of building a single package.
//
// A single build record aggregates information from several build
// events, grouping that information by the Go package import path.
type BuildRecord struct {
	Output strings.Builder
	Fail   bool // Whether a build-fail event was seen for this import path.

	// Reported tracks whether this build record has been reported
	// as part of at least one test event, before reaching the end
	// of the event stream.
	Reported bool
}

// ingest ingests te, which must be a build event,
// and updates the build record br accordingly.
func (br *BuildRecord) ingest(te *GoTestEvent) {
	switch te.Action {
	case "build-output":
		br.Output.WriteString(te.Output)
	case "build-fail":
		br.Fail = true
	default:
		// Ignore.
		//
		// In the unlikely case a new build event is added,
		// it's likely safe to ignore it by default, until
		// result_adapter gets updated to handle it better.
	}
}

// GoTestRenderer takes a go test -json event stream and renders it as text.
//
// It supports two modes: verbose and non-verbose mode. They correspond
// roughly to the output of "go test" with and without -v respectively.
type GoTestRenderer struct {
	w            io.Writer
	testOut      map[string]*pkg         // Map key is the 'Package' field.
	buildRecords map[string]*BuildRecord // Map key is the 'ImportPath' field.
	pkgs         []string
	verbose      bool
}

func NewGoTestRenderer(w io.Writer, verbose bool) *GoTestRenderer {
	return &GoTestRenderer{
		w:            w,
		testOut:      make(map[string]*pkg),
		buildRecords: make(map[string]*BuildRecord),
		verbose:      verbose,
	}
}

// Ingest consumes the next event from a go test -json event stream.
func (r *GoTestRenderer) Ingest(ev *GoTestEvent) error {
	// If we see any output from a package, record that
	// we've seen that package.
	if ev.Package != "" && r.testOut[ev.Package] == nil {
		r.testOut[ev.Package] = newPkg()
		r.pkgs = append(r.pkgs, ev.Package)
	}

	// If this is a build event, ingest it as such.
	//
	// Most build events will be consumed by later
	// test events with matching FailedBuild field.
	if ev.IsBuildEvent() {
		br := r.buildRecords[ev.ImportPath]
		if br == nil {
			br = new(BuildRecord)
			r.buildRecords[ev.ImportPath] = br
		}
		br.ingest(ev)
		return nil
	}

	// If we get this far, this is a test event.
	// Ingest it.
	switch ev.Action {
	case "run":
		if ev.Test != "" {
			r.testOut[ev.Package].tests[ev.Test] = new(lines)
		}

	case "output":
		if ev.Test == "" {
			// Top-level package output.
			//
			// Ignore just "PASS" in non-verbose mode because that's
			// omitted from the standard "go test" output. The next line
			// will be the "ok" line we do want to print.
			if !r.verbose && ev.Output == "PASS\n" {
				break
			}
			r.testOut[ev.Package].extra.add(ev.Output)
			break
		}
		// Lines starting with "=== " are progress
		// updates only shown in verbose mode (like
		// "=== RUN"). These are never indented.
		if !r.verbose && strings.HasPrefix(ev.Output, "=== ") {
			break
		}
		// Accumulate output in case this test fails.
		if lines, ok := r.testOut[ev.Package].tests[ev.Test]; ok {
			lines.add(ev.Output)
		} else {
			if _, err := fmt.Fprintf(r.w, "\"output\" event from unexpected test: %+v\n", ev); err != nil {
				return err
			}
		}

	case "fail":
		if ev.Test == "" {
			// Package failed.
			r.testOut[ev.Package].done = true
			r.testOut[ev.Package].failed = true

			// Include information about the build failure that caused this test failure.
			if ev.FailedBuild != "" {
				br, ok := r.buildRecords[ev.FailedBuild]
				if !ok {
					// This should not happen under normal circumstances.
					// Report some details about it if it happens anyway.
					br = new(BuildRecord)
					fmt.Fprintf(&br.Output, "unexpected case: encountered a TestEvent with FailedBuild field %q, but no corresponding BuildEvent\n", ev.FailedBuild)
				}

				// Report the build record no more than once in the text.
				// This matches the behavior of 'go test' (without -json).
				if !br.Reported {
					r.testOut[ev.Package].extra.add(br.Output.String())
					br.Reported = true
				}
			}

			break
		}
		// Leave failed tests in the map.

	case "pass", "skip":
		if ev.Test == "" {
			// Package passed, so mark it done.
			r.testOut[ev.Package].done = true
			break
		}
		if !r.verbose {
			// The test passed, so delete accumulated output in non-verbose mode.
			delete(r.testOut[ev.Package].tests, ev.Test)
		}
	}

	// Flush completed tests.
	return r.flushCompleted()
}

func (r *GoTestRenderer) flushCompleted() error {
	for len(r.pkgs) > 0 && r.testOut[r.pkgs[0]].done {
		pkg := r.testOut[r.pkgs[0]]
		delete(r.testOut, r.pkgs[0])
		r.pkgs = r.pkgs[1:]
		if err := pkg.emitTests(r.w, r.verbose); err != nil {
			return err
		}
	}
	return nil
}

func (r *GoTestRenderer) Close(renderFailed bool) error {
	if renderFailed {
		// If rendering test output already failed,
		// don't try to render more test output.
		return nil
	}

	// By now, records in the buildRecords map have likely been reported
	// as part of test events that had a corresponding FailedBuild field.
	// However, in some cases there may be build events without a corresponding
	// test event (for example, if there are build warnings and no test failure).
	//
	// So, if we get this far and there are unreported build records, add them
	// to the closest fit package record we find, creating new package records
	// if there's no exact match.
	for importPath, br := range r.buildRecords {
		if br.Reported {
			// Already reported, nothing to do.
			continue
		}

		if r.testOut[importPath] == nil {
			r.testOut[importPath] = newPkg()
			r.pkgs = append(r.pkgs, importPath)

			// By default, treat build-output with no build-fail
			// as a passing result. This corresponds to behavior
			// like compiler warnings.
			r.testOut[importPath].done = true
			r.testOut[importPath].failed = false
		}
		if !r.testOut[importPath].failed && br.Fail {
			// If the package record was passing but we observed
			// a build-fail event, change it to a failing result.
			r.testOut[importPath].failed = true
		}
		r.testOut[importPath].extra.add(br.Output.String())
	}

	// Flush completed packages.
	if err := r.flushCompleted(); err != nil {
		return err
	}

	// Report incomplete packages.
	if len(r.testOut) != 0 {
		if _, err := fmt.Fprintln(r.w, "packages neither passed nor failed:"); err != nil {
			return err
		}
		for pkgName, pkg := range r.testOut {
			if _, err := fmt.Fprintln(r.w, pkgName); err != nil {
				return err
			}
			if err := pkg.emitTests(r.w, r.verbose); err != nil {
				return err
			}
		}
	}

	return nil
}

// pkg records test output from a package.
type pkg struct {
	// tests records output lines for in-flight and failed tests.
	tests map[string]*lines

	// extra records package-level test output.
	extra lines

	// done is set when all tests from this package are done, and
	// failed indicates if a completed package failed.
	done, failed bool
}

func newPkg() *pkg {
	return &pkg{tests: make(map[string]*lines)}
}

func (p *pkg) emitTests(w io.Writer, verbose bool) error {
	// Sort tests.
	tests := make([]string, 0, len(p.tests))
	for k := range p.tests {
		tests = append(tests, k)
	}
	sort.Strings(tests)

	// Emit each test.
	for _, test := range tests {
		if err := p.tests[test].emit(w, verbose); err != nil {
			return err
		}
	}

	// Emit package-level output.
	if verbose || p.failed {
		// Emit everything in the case of a failure or in verbose mode.
		return p.extra.emit(w, verbose)
	}
	// Prune everything except the "ok" when passing in non-verbose mode.
	var pruned lines
	for _, line := range p.extra.lines {
		if isOkLine(line) {
			pruned.add(line)
			break
		}
	}
	return pruned.emit(w, verbose)
}

type lines struct {
	lines []string
}

func (l *lines) add(line string) {
	l.lines = append(l.lines, line)
}

func (l *lines) emit(w io.Writer, verbose bool) error {
	lines := l.lines
	// The last line could be a (possibly indented) "--- FAIL". In
	// non-verbose mode, this is printed *before* the test log.
	if !verbose && len(lines) > 0 && isFailLine(lines[len(lines)-1]) {
		if _, err := io.WriteString(w, lines[len(lines)-1]); err != nil {
			return err
		}
		lines = lines[:len(lines)-1]
	}
	for _, line := range lines {
		if _, err := io.WriteString(w, line); err != nil {
			return err
		}
	}
	return nil
}

func isFailLine(line string) bool {
	// The line may be indented.
	line = strings.TrimLeft(line, " ")
	return strings.HasPrefix(line, "--- FAIL: ")
}

func isOkLine(line string) bool {
	return strings.HasPrefix(line, "ok")
}
