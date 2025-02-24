// Copyright 2018 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/proto"

	findingspb "go.chromium.org/luci/common/proto/findings"

	tricium "go.chromium.org/infra/tricium/api/v1"
	"go.chromium.org/infra/tricium/functions/shellcheck/runner"
)

const (
	analyzerName   = "shellcheck"
	bundledBinPath = "shellcheck_subtool/bin/shellcheck"
)

var (
	runnerLogger = log.New(os.Stderr, "shellcheck", log.LstdFlags)
)

func main() {
	inputDir := flag.String("input", "", "Path to root of Tricium input")
	outputDir := flag.String("output", "", "Path to root of Tricium output")
	shellCheckPath := flag.String("shellcheck_path", "", "Path to shellcheck binary")

	exclude := flag.String("exclude", "", "Exclude warnings (see shellcheck")
	shell := flag.String("shell", "", "Specify dialect (see shellcheck")
	enable := flag.String("enable", "", "Enable optional checks (see shellcheck)")

	// This is needed until/unless crbug.com/863106 is fixed.
	pathFilters := flag.String("path_filters", "", "Patterns to filter file list")

	flag.Parse()
	if flag.NArg() != 0 {
		log.Panicf("Unexpected argument")
	}

	r := &runner.Runner{
		Path:    *shellCheckPath,
		Dir:     *inputDir,
		Exclude: *exclude,
		Enable:  *enable,
		Shell:   *shell,
		Logger:  runnerLogger,
	}

	if r.Path == "" {
		// No explicit shellcheck_bin; try to find one.
		r.Path = findShellCheckBin()
		if r.Path == "" {
			log.Panic("Couldn't find shellcheck bin!")
		}
		// Validate that the found binary is a supported version of shellcheck.
		version, err := r.Version()
		if err != nil {
			log.Panicf("Error checking shellcheck version: %v", err)
		}
		if !strings.HasPrefix(version, "0.") || version < "0.6" {
			log.Panicf("Found shellcheck with unsupported version %q", version)
		}
	}

	run(r, *inputDir, *outputDir, *pathFilters)
}

func run(r *runner.Runner, inputDir, outputDir, pathFilters string) {
	// Read Tricium input FILES data.
	input := &tricium.Data_Files{}
	if err := tricium.ReadDataType(inputDir, input); err != nil {
		log.Panicf("Failed to read FILES data: %v", err)
	}
	log.Printf("Read FILES data.")

	filtered, err := tricium.FilterFiles(input.Files, strings.Split(pathFilters, ",")...)
	if err != nil {
		log.Panicf("Failed to filter files: %v", err)
	}

	// Run shellcheck on input files.
	paths := make([]string, len(filtered))
	for i, f := range filtered {
		paths[i] = f.Path
	}

	var warns []runner.Warning
	if len(paths) > 0 {
		warns, err = r.Warnings(paths...)
		if err != nil {
			log.Panicf("Error running shellcheck: %v", err)
		}
	} else {
		log.Printf("No files to check.")
	}

	// Convert shellcheck warnings into findings
	findings := make([]*findingspb.Finding, len(warns))
	for i, warn := range warns {
		findings[i] = &findingspb.Finding{
			Category: analyzerName,
			Message:  fmt.Sprintf("error code: SC%d\n\n%s\n\n%s", warn.Code, warn.Message, warn.WikiURL()),
			Location: &findingspb.Location{
				FilePath: warn.File,
				Range: &findingspb.Location_Range{
					StartLine: warn.Line,
					EndLine:   warn.EndLine,
					// shellcheck uses 1-based columns, but finding needs 0-based columns.
					StartColumn: warn.Column - 1,
					EndColumn:   warn.EndColumn - 1,
				},
			},
		}
		switch strings.ToLower(warn.Level) {
		case "error":
			findings[i].SeverityLevel = findingspb.Finding_SEVERITY_LEVEL_ERROR
		case "warning":
			findings[i].SeverityLevel = findingspb.Finding_SEVERITY_LEVEL_WARNING
		case "info", "style":
			findings[i].SeverityLevel = findingspb.Finding_SEVERITY_LEVEL_INFO
		default:
			findings[i].SeverityLevel = findingspb.Finding_SEVERITY_LEVEL_WARNING
		}
	}

	if err := writeFindingsOutput(findings, outputDir); err != nil {
		log.Fatalf("Failed to write findings output: %v", err)
	}

	// TODO: b/388321980 - stop writing tricium data after deprecation
	if err := writeTriciumResults(findings, outputDir); err != nil {
		log.Fatalf("Failed to write Tricium results: %v", err)
	}
}

func findShellCheckBin() string {
	// Look for bundled shellcheck next to this executable.
	ex, err := os.Executable()
	if err == nil {
		bundledPath := filepath.Join(filepath.Dir(ex), bundledBinPath)
		if path, err := exec.LookPath(bundledPath); err == nil {
			return path
		}
	}
	// Look in PATH.
	if path, err := exec.LookPath("shellcheck"); err == nil {
		return path
	}
	return ""
}

func writeFindingsOutput(findings []*findingspb.Finding, outputDir string) error {
	findingsData, err := proto.Marshal(&findingspb.Findings{Findings: findings})
	if err != nil {
		return fmt.Errorf("failed to marshal the findings: %w", err)
	}
	if err := os.MkdirAll(outputDir, 0777); err != nil {
		return fmt.Errorf("failed to create output dir %s: %w", outputDir, err)
	}
	path := filepath.Join(outputDir, "findings.out")
	if err := os.WriteFile(path, findingsData, 0644); err != nil {
		return fmt.Errorf("failed to write the findings to %s: %w", path, err)
	}
	log.Printf("Wrote findings data to %s", path)
	return nil
}

// TODO(crbug/388321980) : stop writing tricium data after deprecation
func writeTriciumResults(findings []*findingspb.Finding, outputDir string) error {
	results := &tricium.Data_Results{
		Comments: make([]*tricium.Data_Comment, len(findings)),
	}

	for i, finding := range findings {
		results.Comments[i] = &tricium.Data_Comment{
			Category:  finding.GetCategory(),
			Message:   finding.GetMessage(),
			Path:      finding.GetLocation().GetFilePath(),
			StartLine: finding.GetLocation().GetRange().GetStartLine(),
			EndLine:   finding.GetLocation().GetRange().GetEndLine(),
			StartChar: finding.GetLocation().GetRange().GetStartColumn(),
			EndChar:   finding.GetLocation().GetRange().GetEndColumn(),
		}
		switch finding.GetSeverityLevel() {
		case findingspb.Finding_SEVERITY_LEVEL_INFO:
			results.Comments[i].Message = "INFO: " + results.Comments[i].Message
		case findingspb.Finding_SEVERITY_LEVEL_WARNING:
			results.Comments[i].Message = "WARNING: " + results.Comments[i].Message
		case findingspb.Finding_SEVERITY_LEVEL_ERROR:
			results.Comments[i].Message = "ERROR: " + results.Comments[i].Message
		}
	}
	path, err := tricium.WriteDataType(outputDir, results)
	if err != nil {
		return err
	}
	log.Printf("Wrote RESULTS data to %s", path)
	return nil
}
