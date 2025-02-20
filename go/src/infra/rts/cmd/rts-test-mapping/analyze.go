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
	"math/rand"
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

	"go.chromium.org/infra/rts"
	"go.chromium.org/infra/rts/presubmit/eval"
	evalpb "go.chromium.org/infra/rts/presubmit/eval/proto"
)

type analyzeCommandRun struct {
	subcommands.CommandRunBase
	authOpt                    *auth.Options
	builder                    string
	debugMode                  bool
	favoriteTestsJSONFile      string
	ev                         eval.Eval
	osStr                      string
	refinedMode                bool
	similarityJSONFile         string
	testSelectionEnagedCluster int
	testSelectionPerCluster    int
	testSelectionRestCluster   int
}

type debugEntry struct {
	filePath        string
	optimizedReason int
	testSuites      string
	testIds         string
}

type decisionEngine struct {
	advancedAlgorithm          bool
	allowedTests               map[string]struct{}
	cqTests                    map[string]struct{}
	fileFavoriteTestsMapping   map[string][]string
	skippedTests               map[string]struct{}
	testClusterIDsMapping      map[string]map[string]struct{}
	testSelectionEnagedCluster int
	testSelectionPerCluster    int
	testSelectionRestCluster   int
}

func cmdAnalyze(authOpt *auth.Options) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: `analyze -rejections <path> -durations <path> [-builder <name>] [-os <os_string>] -similarity_json <path> [-refined] [-favorite_json <path>]`,
		ShortDesc: `Prints the expected recall and savings on the test mapping CQ run`,
		LongDesc: text.Doc(`
			This process uses a provided JSON file containing test cluster data to determine which tests to run during a CQ (Commit Queue) run, aiming to increase efficiency and resource savings.

			Flag -similarity_json is required. It should be in format of {test_id_1: [test_id_2, test_id_3, ...]}.
		`),
		CommandRun: func() subcommands.CommandRun {
			r := &analyzeCommandRun{authOpt: authOpt}
			r.Flags.StringVar(&r.builder, "builder", "", "(Optional)Builder running the testSuite to exclude from tests")
			r.Flags.BoolVar(&r.debugMode, "debug_mode", false, "(Optional)A flag to indicate whether to have debug output.")
			r.Flags.StringVar(&r.favoriteTestsJSONFile, "favorite_json", "", "(Optional)A json file containing depot filepath and its favorite tests.")
			r.Flags.StringVar(&r.osStr, "os", "", "(Optional)The os sub string value such as Ubuntu for linux, Win for windows, Mac for MacOS, etc.")
			r.Flags.BoolVar(&r.refinedMode, "refined", false, "(Optional)A flag to indicate whether to run on random algorithm or refined algorithm.")
			r.Flags.StringVar(&r.similarityJSONFile, "similarity_json", "", "(Required)A json file containing similarity between test case ids, such as {test_id_1: [test_id_2]}")
			r.Flags.IntVar(&r.testSelectionPerCluster, "test_per_cluster", 1, "(Optional)The count of tests to be selected to run per cluster.")
			r.Flags.IntVar(&r.testSelectionEnagedCluster, "test_per_engaged_cluster", 3, "(Optional)The count of tests to be selected to run per engaged cluster.")
			r.Flags.IntVar(&r.testSelectionRestCluster, "test_per_rest_cluster", 5, "(Optional)The count of tests to be selected to run per rest cluster.")
			r.ev.LogProgressInterval = 1000
			if err := r.ev.RegisterFlags(&r.Flags); err != nil {
				logging.Warningf(context.Background(), "RegisterFlags() return: %s", err.Error())
			}
			return r
		},
	}
}

func (r *analyzeCommandRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, r, env)

	if err := r.validateFlags(); err != nil {
		logging.Infof(ctx, err.Error())
		return 1
	}

	fileDebuggingEntries := make(map[string][]debugEntry)
	testClusterMapping := make(map[string]map[string]struct{})
	fileFavoriteTestsMapping := make(map[string][]string)
	var mu = &sync.RWMutex{}
	var err error
	if r.similarityJSONFile != "" {
		testSimilarityMappingTemp, err := loadFromJSONFile(r.similarityJSONFile)
		if err != nil {
			logging.Infof(ctx, err.Error())
			return 1
		}
		for key, values := range testSimilarityMappingTemp {
			if _, ok := testClusterMapping[key]; !ok {
				testClusterMapping[key] = map[string]struct{}{}
			}
			for _, value := range values {
				testClusterMapping[key][value] = struct{}{}
			}
		}
	}
	// Gets some stats
	distinctTests := make(map[string]struct{})
	if r.favoriteTestsJSONFile != "" {
		fileFavoriteTestsMappingTemp, err := loadFromJSONFile(r.favoriteTestsJSONFile)
		if err != nil {
			logging.Infof(ctx, err.Error())
			return 1
		}
		for key, values := range fileFavoriteTestsMappingTemp {
			fileFavoriteTestsMapping[key] = values
			for _, v := range values {
				distinctTests[v] = struct{}{}
			}
		}
	}
	logging.Infof(ctx, "Size of favorite_tests: %d", len(fileFavoriteTestsMapping))
	logging.Infof(ctx, "Number of distinct tests: %d", len(distinctTests))
	// Build a {cluster_id: test_ids} table
	testClusterIDsMapping := clusterTests(testClusterMapping)
	unchangedVariantCount := 0
	changedVariantCount := 0
	res, err := r.ev.Run(ctx, func(ctx context.Context, in eval.Input, out *eval.Output) error {
		// Build allowedTests and skippedTests.
		optimizedReason := -1
		affectedBuilder := r.builder
		e := decisionEngine{
			advancedAlgorithm:          r.refinedMode,
			allowedTests:               make(map[string]struct{}),
			cqTests:                    make(map[string]struct{}),
			fileFavoriteTestsMapping:   fileFavoriteTestsMapping,
			skippedTests:               make(map[string]struct{}),
			testClusterIDsMapping:      testClusterIDsMapping,
			testSelectionEnagedCluster: r.testSelectionEnagedCluster,
			testSelectionPerCluster:    r.testSelectionPerCluster,
			testSelectionRestCluster:   r.testSelectionRestCluster,
		}
		e.update(in)
		if r.debugMode {
			logging.Infof(ctx, "The number of tests in allowedTests: %d\n", len(e.allowedTests))
			logging.Infof(ctx, "The number of tests in skippedTests: %d\n", len(e.skippedTests))
		}

		for i, tv := range in.TestVariants {
			variantBuilderSuite := getBuilderSuiteString(tv.Variant)
			osString := getOsString(tv.Variant)

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

			if _, ok := e.skippedTests[tv.Id]; ok {
				out.TestVariantAffectedness[i] = rts.Affectedness{Distance: math.Inf(1)}
				optimizedReason = 0
				logDebugEntry(mu, tv, optimizedReason, variantBuilderSuite, fileDebuggingEntries, in.ChangedFiles)
				continue
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
	}
	return 0
}

// Uses advanced algorithm to pick up tests into allowedTests and skippedTests.
func (r *decisionEngine) updateInAdvancedAlgorithm(in eval.Input) {
	// Advanced algorithm to select allowedTests and skippedTests.
	//
	// Generate a list of allowedTests via a FER formula:
	//   F for favorite tests from joining the table with in.ChangedFiles,
	//   E for tests from engaged clusters,
	//   R for tests from rest clusters.
	// allowedTests = F + E + R.

	// For F tests: retrieve from changed files join fileFavoriteTestsMapping.
	r.allowedTests = testsFromChangedFiles(in.ChangedFiles, r.fileFavoriteTestsMapping)
	engagedClusters := make(map[string]struct{})
	allClusters := make(map[string]struct{})
	for id, tests := range r.testClusterIDsMapping {
		allClusters[id] = struct{}{}

		for t := range tests {
			if _, ok := r.allowedTests[t]; ok {
				// Mark the cluster as engaged cluster
				engagedClusters[id] = struct{}{}
				break
			}
		}
	}
	restClusters := diffSet(allClusters, engagedClusters)
	for id := range engagedClusters {
		// For E tests: retrieve from engaged clusters.
		tests := r.testClusterIDsMapping[id]
		base := intersectSets(tests, r.cqTests)
		testPerCluster := r.testSelectionEnagedCluster
		if testPerCluster > len(base) {
			testPerCluster = len(base)
		}
		selected := randomSelectFromSet(base, testPerCluster)
		extendSetFromList(r.allowedTests, selected)
		for t := range tests {
			if _, ok := r.allowedTests[t]; !ok {
				r.skippedTests[t] = struct{}{}
			}
		}
	}
	for id := range restClusters {
		// For R tests: retrieve from other clusters.
		tests := r.testClusterIDsMapping[id]
		base := intersectSets(tests, r.cqTests)
		testPerCluster := r.testSelectionRestCluster
		if testPerCluster > len(base) {
			testPerCluster = len(base)
		}
		selected := randomSelectFromSet(base, testPerCluster)
		extendSetFromList(r.allowedTests, selected)
		for t := range tests {
			if _, ok := r.allowedTests[t]; !ok {
				r.skippedTests[t] = struct{}{}
			}
		}
	}
}

// Uses random selection to pick up tests into allowedTests and skippedTests.
func (r *decisionEngine) updateInRandomAlgorithm() {
	// Random algorithms to select allowedTests and skippedTests.
	for _, tests := range r.testClusterIDsMapping {
		base := intersectSets(tests, r.cqTests)
		testPerCluster := r.testSelectionPerCluster
		if testPerCluster > len(base) {
			testPerCluster = len(base)
		}
		selected := randomSelectFromSet(base, testPerCluster)
		extendSetFromList(r.allowedTests, selected)
		for t := range tests {
			if _, ok := r.allowedTests[t]; !ok {
				r.skippedTests[t] = struct{}{}
			}
		}
	}
}

// Populates the decisionEngine with allowedTests and skippedTests.
func (r *decisionEngine) update(in eval.Input) {
	for _, tv := range in.TestVariants {
		r.cqTests[tv.Id] = struct{}{}
	}
	if r.advancedAlgorithm {
		r.updateInAdvancedAlgorithm(in)
	} else {
		r.updateInRandomAlgorithm()
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
		return errors.New("-similarity_json is required")
	case r.testSelectionPerCluster < 1:
		return errors.New("-test_per_cluster could not be less than 1")
	case r.refinedMode && r.favoriteTestsJSONFile == "":
		return errors.New("-favorite_json is required when -refined is specified")
	default:
		return nil
	}
}

// Builds a cluster sequence of {cluster_id : [test_id]} from input dict.
func clusterTests(input map[string]map[string]struct{}) map[string]map[string]struct{} {
	clusters := make(map[string]map[string]struct{})
	clusterID := 0
	// Input is {test_id_1 : {test_id_2: true, test_id_3: true}}
	// Output is {cluster_id_1: {test_id_1: true, test_id_2: true, test_id_3: true}}
	for key, values := range input {
		found := false

		for _, cluster := range clusters {
			if _, ok := cluster[key]; ok {
				extendSet(cluster, map[string]struct{}{key: {}})
				extendSet(cluster, values)
				found = true
			}
		}
		if !found {
			c := make(map[string]struct{})
			extendSet(c, map[string]struct{}{key: {}})
			extendSet(c, values)
			clusters[fmt.Sprint(clusterID)] = c
			clusterID += 1
		}
	}
	return clusters
}

// Randomly selects n entries from input.
func randomSelectFromSet(input map[string]struct{}, n int) []string {
	if n <= 0 || n > len(input) {
		return nil // Handle invalid input
	}

	keys := make([]string, 0, len(input))
	for k := range input {
		keys = append(keys, k)
	}

	rand.Shuffle(len(keys), func(i, j int) { keys[i], keys[j] = keys[j], keys[i] })

	return keys[:n]
}

// Generates a map that is the output of source - target.
func diffSet(source, target map[string]struct{}) map[string]struct{} {
	results := make(map[string]struct{})
	for k, v := range source {
		if _, ok := target[k]; !ok {
			results[k] = v
		}
	}
	return results
}

// Generates a map that is the intersect between map1 and map2.
func intersectSets(map1, map2 map[string]struct{}) map[string]struct{} {
	intersection := make(map[string]struct{})

	for key := range map1 {
		if _, ok := map2[key]; ok {
			intersection[key] = struct{}{}
		}
	}

	return intersection
}

// Extends src into dest.
func extendSet(dest, src map[string]struct{}) {
	for k := range src {
		dest[k] = struct{}{}
	}
}

// Extends src into dest.
func extendSetFromList(dest map[string]struct{}, src []string) {
	for _, k := range src {
		dest[k] = struct{}{}
	}
}

// Returns a dict of tests from changedFiles
func testsFromChangedFiles(sourceFiles []*evalpb.SourceFile, lookup map[string][]string) map[string]struct{} {
	candidates := make(map[string]struct{})
	var paths []string
	for _, f := range sourceFiles {
		paths = append(paths, f.Path)
	}
	for _, p := range paths {
		if _, ok := lookup[p]; ok {
			for _, k := range lookup[p] {
				candidates[k] = struct{}{}
			}
		}
	}
	return candidates

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
