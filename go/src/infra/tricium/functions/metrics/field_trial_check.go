// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

	findingspb "go.chromium.org/luci/common/proto/findings"
)

const (
	trialConfigStartPattern = "\": ["
	experimentStart         = "\"experiments\": ["
	manyExperimentsWarning  = `Due to infrastructure capacity limitations, only the first experiment listed in %s will be tested. It's ok to list the others as documentation, but they will not be tested. So, please make sure that the first-listed experiment is the one most likely to launch!`
)

// experiment contains all info about experiment to enable.
type experiment struct {
	Name     string   `json:"name"`
	Features []string `json:"enable_features"`
}

// fieldTrialConfig contains all info about a particular field trial testing configuration.
type fieldTrialConfig struct {
	Platforms   []string     `json:"platforms"`
	Experiments []experiment `json:"experiments"`
	ExpLineNum  int
}

// allConfigs contains all of the field trial configs.
// Each field trial test name can map to multiple experiments.
type allConfigs map[string][]*fieldTrialConfig

func analyzeFieldTrialTestingConfig(reader io.Reader, path string) []*findingspb.Finding {
	var buf bytes.Buffer
	// We need to use a TeeReader here since we will also be scanning the file.
	// A simple reader will consume all of the bytes in the file, leaving nothing to scan.
	tee := io.TeeReader(reader, &buf)
	configsBuf, err := io.ReadAll(tee)
	if err != nil {
		log.Panicf("Failed to read %s into buffer: %v", path, err)
	}
	var configs allConfigs
	if err := json.Unmarshal(configsBuf, &configs); err != nil {
		log.Panicf("Failed to unmarshal config JSON in file %s: %v", path, err)
	}
	getExperimentLineNums(bufio.NewScanner(&buf), configs)
	return checkExperiments(configs, path)
}

func getExperimentLineNums(scanner *bufio.Scanner, configs allConfigs) {
	lineNum := 1
	currName := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		firstWord := strings.Trim(line, trialConfigStartPattern)
		// If first word in line is a config name, set current name to the first word.
		if _, found := configs[firstWord]; found && strings.Contains(line, trialConfigStartPattern) {
			currName = firstWord
		} else if strings.HasPrefix(line, experimentStart) {
			// Set the experiment line number for the next config in array that does not have one.
			for _, config := range configs[currName] {
				if config.ExpLineNum == 0 {
					config.ExpLineNum = lineNum
					break
				}
			}
		}
		lineNum++
	}
}

func checkExperiments(configs allConfigs, path string) []*findingspb.Finding {
	var findings []*findingspb.Finding
	for name, configArr := range configs {
		for _, config := range configArr {
			if len(config.Experiments) > 1 {
				finding := &findingspb.Finding{
					Category:      category,
					Message:       fmt.Sprintf(manyExperimentsWarning, name),
					SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
					Location: &findingspb.Location{
						FilePath: path,
						Range: &findingspb.Location_Range{
							StartLine: int32(config.ExpLineNum),
							EndLine:   int32(config.ExpLineNum),
						},
					},
				}
				log.Printf("ADDING finding for %s at line %d: %s", name, config.ExpLineNum, "[WARNING]: More than 1 Experiment")
				findings = append(findings, finding)
			}
		}
	}
	return findings
}
