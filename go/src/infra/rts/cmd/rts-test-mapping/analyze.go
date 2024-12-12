// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/data/text"
	"go.chromium.org/luci/common/logging"

	"infra/rts"
	"infra/rts/presubmit/eval"
	evalpb "infra/rts/presubmit/eval/proto"
)

type analyzeCommandRun struct {
	subcommands.CommandRunBase
	authOpt                 *auth.Options
	ev                      eval.Eval
	builder                 string
	osStr                   string
	similarityJSONFile      string
	debugMode               bool
	testSelectionPerCluster int
}

type debugEntry struct {
	filePath        string
	optimizedReason int
	testSuites      string
	testIds         string
}

type variantEntry struct {
	lock       *sync.RWMutex
	candidates *sync.Map
}

func cmdAnalyze(authOpt *auth.Options) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: `analyze -rejections <path> -durations <path> -builder <name> -os <os_string> [-similarity_json <path>]`,
		ShortDesc: `Prints the expected recall and savings on the test mapping CQ run`,
		LongDesc: text.Doc(`
			This process uses a provided JSON file containing test cluster data to determine which tests to run during a CQ (Commit Queue) run, aiming to increase efficiency and resource savings.

			Flag -similarity_json is required. It should be in format of {test_id_1: [test_id_2, test_id_3, ...]}.
		`),
		CommandRun: func() subcommands.CommandRun {
			r := &analyzeCommandRun{authOpt: authOpt}
			r.Flags.StringVar(&r.builder, "builder", "", "(optional)Builder running the testSuite to exclude from tests")
			r.Flags.StringVar(&r.osStr, "os", "", "(optional)The os sub string value such as Ubuntu for linux, Win for windows, Mac for MacOS, etc.")
			r.Flags.StringVar(&r.similarityJSONFile, "similarity_json", "", "(required)A json file containing similarity between test case ids, such as {test_id_1: [test_id_2]}")
			r.Flags.IntVar(&r.testSelectionPerCluster, "test_per_cluster", 1, "(optional)The count of tests to be selected to run per cluster.")
			r.Flags.BoolVar(&r.debugMode, "debug_mode", false, "(optional)A flag to indicate whether to have debug output.")
			r.ev.LogProgressInterval = 1000
			if err := r.ev.RegisterFlags(&r.Flags); err != nil {
				logging.Warningf(context.Background(), "RegisterFlags() return: %s", err.Error())
			}
			return r
		},
	}
}

// Stores a debugEntry into entries list.
func logDebugEntry(m *sync.RWMutex, tv *evalpb.TestVariant, reason int, suite string, entries map[string][]debugEntry, changedFiles []*evalpb.SourceFile) {
	m.Lock()
	defer m.Unlock()
	d, ok := entries[tv.FileName]
	if !ok {
		d = make([]debugEntry, 0)
	}
	e := new(debugEntry)
	// Stop using tv.FileName as that is the test source file.
	files := []string{}
	for _, f := range changedFiles {
		files = append(files, f.Path)
	}
	slices.Sort(files)
	ck := signatureFromSourceFiles(changedFiles)
	e.filePath = strings.Join(files[:], ",")
	e.optimizedReason = reason
	e.testSuites = suite
	e.testIds = tv.Id
	entries[ck] = append(d, *e)
}

// Validates input command line flags.
func (r *analyzeCommandRun) validateFlags() error {
	if err := r.ev.ValidateFlags(); err != nil {
		return err
	}
	switch {
	case r.similarityJSONFile == "":
		return errors.New("-similarity_json is required.")
	case r.testSelectionPerCluster < 1:
		return errors.New("-test_per_cluster could not be less than 1.")
	default:
		return nil
	}
}

func addValuesToMap(data map[string]map[string]bool, key string, values []string) {
	if _, ok := data[key]; !ok {
		data[key] = map[string]bool{}
	}
	for _, value := range values {
		data[key][value] = true
	}
}

func (r *analyzeCommandRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, r, env)

	if err := r.validateFlags(); err != nil {
		logging.Infof(ctx, err.Error())
		return 1
	}

	fileDebuggingEntries := make(map[string][]debugEntry)
	testClusterMapping := make(map[string]map[string]bool)
	var mu = &sync.RWMutex{}
	var err error
	if r.similarityJSONFile != "" {
		testSimilarityMappingTemp, err := loadFromJSONFile(r.similarityJSONFile)
		if err != nil {
			logging.Infof(ctx, err.Error())
			return 1
		}
		for key, values := range testSimilarityMappingTemp {
			addValuesToMap(testClusterMapping, key, values)
		}
	}
	variantHashDisallowedTests := sync.Map{}
	unchangedVariantCount := 0
	changedVariantCount := 0
	// Build testClusterIdMapping from testClusterMapping
	res, err := r.ev.Run(ctx, func(ctx context.Context, in eval.Input, out *eval.Output) error {
		optimizedReason := -1

		for i, tv := range in.TestVariants {
			variantBuilderSuite := getBuilderSuiteString(tv.Variant)
			osString := getOsString(tv.Variant)
			affectedBuilder := r.builder
			inSignature := signatureFromSourceFiles(in.ChangedFiles)
			if !strings.Contains(variantBuilderSuite, affectedBuilder) || !strings.Contains(osString, r.osStr) {
				if !strings.Contains(variantBuilderSuite, affectedBuilder) {
					optimizedReason = -3
				}
				if !strings.Contains(osString, r.osStr) {
					optimizedReason = -4
				}
				out.TestVariantAffectedness[i] = rts.Affectedness{Distance: 0}
				unchangedVariantCount += 1
				logDebugEntry(mu, tv, optimizedReason, variantBuilderSuite, fileDebuggingEntries, in.ChangedFiles)
				continue
			}

			if _, ok := variantHashDisallowedTests.Load(inSignature); !ok {
				// If this inSignature was never saw before, create a variantEntry for this inSignature entry
				entry := variantEntry{}
				entry.candidates = &sync.Map{}
				entry.lock = &sync.RWMutex{}
				variantHashDisallowedTests.Store(inSignature, entry)
			}
			testToBeTrimmedTemp, _ := variantHashDisallowedTests.Load(inSignature)
			disallowedTestList := testToBeTrimmedTemp.(variantEntry)
			// If this test_id is already in testToBeTrimmed, skip this test run.
			disallowedTestList.lock.RLock()
			if _, ok := disallowedTestList.candidates.Load(tv.Id); ok {
				out.TestVariantAffectedness[i] = rts.Affectedness{Distance: math.Inf(1)}
				optimizedReason = 0
				logDebugEntry(mu, tv, optimizedReason, variantBuilderSuite, fileDebuggingEntries, in.ChangedFiles)
				disallowedTestList.lock.RUnlock()
				continue
			}
			disallowedTestList.lock.RUnlock()
			// If this test id has an entry in testSimilarityMapping, disallow its buddies.
			if _, ok := testClusterMapping[tv.Id]; ok {
				countToDisallow := len(testClusterMapping[tv.Id]) - r.testSelectionPerCluster
				for testID := range testClusterMapping[tv.Id] {
					if countToDisallow <= 0 {
						break
					}
					disallowedTestList.lock.Lock()
					disallowedTestList.candidates.Store(testID, true)
					disallowedTestList.lock.Unlock()
					countToDisallow -= 1
				}
			}
			out.TestVariantAffectedness[i] = rts.Affectedness{Distance: 0}
			unchangedVariantCount += 1
			logDebugEntry(mu, tv, optimizedReason, variantBuilderSuite, fileDebuggingEntries, in.ChangedFiles)
		}
		return nil
	})

	if err != nil {
		logging.Infof(ctx, err.Error())
		return 1
	}

	// We don't care about the 100% recall and 0% savings threshold
	if len(res.Thresholds) > 1 {
		res.Thresholds = res.Thresholds[:1]
	}

	r.ev.LogAndClearFurthest(ctx)
	if err := eval.PrintSpecificResults(res, os.Stdout, 0.0, true, false); err != nil {
		logging.Warningf(context.Background(), "PrintSpecificResults return %s", err.Error())
	}

	fmt.Printf("No behavior change Test variant run count (baseline): %d\n", unchangedVariantCount)
	fmt.Printf("Behavior change Test variant run count (saving): %d\n", changedVariantCount)
	if r.debugMode {
		for _, vs := range fileDebuggingEntries {
			for _, v := range vs {
				if v.optimizedReason == 0 {
					logging.Infof(ctx, "For file %s", v.filePath)
					logging.Infof(ctx, "OptimizedReason: %d", v.optimizedReason)
					logging.Infof(ctx, "Test Suite: %s", v.testSuites)
					logging.Infof(ctx, "TestId: %s", v.testIds)
					logging.Infof(ctx, "\n")
				}
			}
		}
		variantHashDisallowedTests.Range(func(key, value any) bool {
			fmt.Println("Signature:", key, "Tests to remove:", value)
			return true // Continue iteration
		})
	}
	return 0
}

// Uses a list of path from sourceFiles to generate a SHA256 hash.
func signatureFromSourceFiles(sourceFiles []*evalpb.SourceFile) string {
	var paths []string
	for _, f := range sourceFiles {
		paths = append(paths, f.Path)
	}
	sort.Strings(paths)
	concatenatedString := strings.Join(paths, "\n")
	hash := sha256.New()
	hash.Write([]byte(concatenatedString))
	sum := hash.Sum(nil)
	return fmt.Sprintf("%x", sum)
}

// Loads and returns the json file from given path.
func loadFromJSONFile(fileName string) (map[string][]string, error) {
	if fileName == "" {
		return nil, nil
	}
	f, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
		return nil, errors.New("failed to load the json file.")
	}
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Printf("Close file err: %v", err)
		}
	}()

	fmt.Println("The File is opened successfully...")

	byteResult, _ := io.ReadAll(f)

	var lookup map[string][]string
	if err := json.Unmarshal([]byte(byteResult), &lookup); err != nil {
		return nil, err
	}

	return lookup, nil
}

// Returns a string such as "linux-rel:browser-tests"
func getBuilderSuiteString(list []string) string {
	builder := ""
	testSuite := ""
	for _, b := range list {
		if strings.HasPrefix(b, "builder:") {
			builder = b[len("builder:"):]
		}
		if strings.HasPrefix(b, "test_suite:") {
			testSuite = b[len("test_suite:"):]
		}
	}
	if builder == "" || testSuite == "" {
		return ""
	}
	return builder + ":" + testSuite
}

// Returns a string such as "Ubuntu-22.04"
func getOsString(list []string) string {
	osStr := ""
	for _, b := range list {
		if strings.HasPrefix(b, "os:") {
			osStr = b[len("os:"):]
		}
	}
	if osStr == "" {
		return ""
	}
	return osStr
}
