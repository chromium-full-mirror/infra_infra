// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build ignore

package main

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"log"
	"net/http"
	"os"
	"runtime"
	"slices"
	"time"

	"go.chromium.org/luci/auth"
	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/hardcoded/chromeinfra"
	rdbpb "go.chromium.org/luci/resultdb/proto/v1"
	sauth "go.chromium.org/luci/server/auth"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"go.chromium.org/infra/experimental/golangbuild/golangbuildpb"
)

var (
	nBuilds = flag.Int("n", 10, "number of builds to average over")
	verbose = flag.Bool("v", false, "print extra debug information")
)

func main() {
	// Validate flags.
	flag.Parse()
	if *nBuilds <= 0 {
		log.Fatal("-n must be a positive integer")
	}

	// Create authenticated client.
	ctx := context.Background()
	authOpts := chromeinfra.SetDefaultAuthOptions(auth.Options{
		Scopes: append([]string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/gerritcodereview",
		}, sauth.CloudOAuthScopes...),
	})
	hc, err := auth.NewAuthenticator(ctx, auth.SilentLogin, authOpts).Client()
	if err != nil {
		log.Fatalf("failed to create authenticated client: %v", err)
	}
	lc := NewLUCIClient(hc)

	// Fetch all sharded builders.
	log.Print("fetching builder list")
	builders, err := getShardedBuilders(ctx, lc)
	if err != nil {
		log.Fatalf("fetching sharded builders: %v", err)
	}
	log.Printf("found %d builders", len(builders))

	// Fetch all the test timings.
	//
	// This slice mirrors builders.
	if !*verbose {
		log.Printf("fetching package timings, averaged over %d builds", *nBuilds)
	}
	pkgTimes := make([][]pkgTiming, len(builders))
	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(runtime.GOMAXPROCS(-1) * 4) // Mostly I/O-bound.
	for i, b := range builders {
		eg.Go(func() error {
			if *verbose {
				log.Printf("%s: fetching package timings", b)
			}

			var err error
			pkgTimes[i], err = fetchPackageTimingsForBuilder(ctx, lc, b, *nBuilds)
			if err != nil {
				return fmt.Errorf("failed to fetch package timings for builder %s: %v", b, err)
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		log.Fatalf("fetching package timings: %v", err)
	}

	// Write out the file.
	log.Print("writing weights file")
	if err := writeWeightsFile(builders, pkgTimes); err != nil {
		log.Fatalf("writing weights file: %v", err)
	}
}

func getShardedBuilders(ctx context.Context, c *LUCIClient) ([]string, error) {
	var builders []string
	var pageToken string
	for page := 1; ; page++ {
		resp, err := c.Builders.ListBuilders(ctx, &bbpb.ListBuildersRequest{
			Project:   "golang",
			Bucket:    "ci",
			PageSize:  1000,
			PageToken: pageToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list page %d of builders: %v", page, err)
		}
		for _, b := range resp.GetBuilders() {
			var p golangbuildpb.Inputs
			err := json.Unmarshal([]byte(b.GetConfig().GetProperties()), &p)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal builder properties for %s: %v", b.Id, err)
			}
			if p.Mode != golangbuildpb.Mode_MODE_COORDINATOR { // Skip non-sharded builders.
				continue
			}
			if p.CoordMode.NumTestShards == 1 { // Skip builders with only one shard.
				continue
			}
			if p.MiscPorts { // Skip misccompile builders.
				continue
			}
			builders = append(builders, b.Id.Builder)
		}
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}
	slices.Sort(builders)
	return builders, nil
}

// LUCIClient is a LUCI client.
type LUCIClient struct {
	Builders bbpb.BuildersClient
	Builds   bbpb.BuildsClient
	ResultDB rdbpb.ResultDBClient
}

// NewLUCIClient creates a LUCI client.
//
// If c is nil, an unauthenticated http.DefaultClient is used,
// otherwise c is expected to be an authenticated HTTP client.
//
// nProc controls concurrency. NewLUCIClient returns an error
// if nProc is non-positive.
func NewLUCIClient(c *http.Client) *LUCIClient {
	return &LUCIClient{
		Builds: bbpb.NewBuildsClient(&prpc.Client{
			C:    c,
			Host: chromeinfra.BuildbucketHost,
		}),
		Builders: bbpb.NewBuildersClient(&prpc.Client{
			C:    c,
			Host: chromeinfra.BuildbucketHost,
		}),
		ResultDB: rdbpb.NewResultDBClient(&prpc.Client{
			C:    c,
			Host: chromeinfra.ResultDBHost,
		}),
	}
}

type pkgTiming struct {
	name     string
	duration time.Duration
}

func fetchPackageTimingsForBuilder(ctx context.Context, c *LUCIClient, builder string, n int) ([]pkgTiming, error) {
	// Fetch the last n successful builds for this builder.
	pred := &bbpb.BuildPredicate{
		Builder: &bbpb.BuilderID{Project: "golang", Bucket: "ci", Builder: builder},
		Status:  bbpb.Status_SUCCESS,
	}
	mask, err := fieldmaskpb.New((*bbpb.Build)(nil), "id", "infra")
	if err != nil {
		return nil, fmt.Errorf("error creating a build mask: %v", err)
	}
	resp, err := c.Builds.SearchBuilds(ctx, &bbpb.SearchBuildsRequest{
		Predicate: pred,
		Mask:      &bbpb.BuildMask{Fields: mask},
		PageSize:  int32(n),
	})
	if err != nil {
		return nil, fmt.Errorf("searching successful builds for builder %s: %v", builder, err)
	}

	// Fetch all the package test timings.
	pkgDurations := make(map[string][]time.Duration)
	for i, b := range resp.Builds {
		if *verbose {
			log.Printf("%s: fetch test results for build %d (https://ci.chromium.org/b/%d", builder, i+1, b.Id)
		}
		inv := b.GetInfra().GetResultdb().GetInvocation()
		var pageToken string
		for page := 1; ; page++ {
			resp, err := c.ResultDB.QueryTestResults(ctx, &rdbpb.QueryTestResultsRequest{
				Invocations: []string{inv},
				// Skip everything with a '.'. This should give us just package-level tests.
				Predicate: &rdbpb.TestResultPredicate{TestIdRegexp: "[^.]*"},
				PageSize:  1000,
				PageToken: pageToken,
			})
			if err != nil {
				return nil, fmt.Errorf("fetching page %d of test results for build %s: %v", page, b.Id, err)
			}
			for _, r := range resp.TestResults {
				pkgDurations[r.TestId] = append(pkgDurations[r.TestId], r.Duration.AsDuration())
			}
			if resp.NextPageToken == "" {
				break
			}
			pageToken = resp.NextPageToken
		}
	}

	// Aggregate the package durations with a mean.
	//
	// TODO(mknyszek): Sort the input and try the median, or maybe the p75 or something.
	pkgTimes := make([]pkgTiming, 0, len(pkgDurations))
	for pkg, durations := range pkgDurations {
		pkgTimes = append(pkgTimes, pkgTiming{name: pkg, duration: mean(durations)})
	}
	slices.SortFunc(pkgTimes, func(a, b pkgTiming) int {
		if a.duration == b.duration {
			return cmp.Compare(a.name, b.name)
		}
		return cmp.Compare(b.duration, a.duration) // Descending order, for humans.
	})
	return pkgTimes, nil
}

func writeWeightsFile(builders []string, timings [][]pkgTiming) error {
	// Generate a Go file.
	contents := generateWeightsFile(builders, timings)

	// Format it.
	formattedContents, err := format.Source(contents)
	if err != nil {
		log.Printf("failed to format generated file, see weights.go: %v", err)
		formattedContents = contents
	}

	// Write it.
	if err := os.WriteFile("weights.go", formattedContents, 0o644); err != nil {
		return fmt.Errorf("writing generated file: %v", err)
	}
	return nil
}

func mean(durations []time.Duration) time.Duration {
	var sum time.Duration
	for _, d := range durations {
		sum += d
	}
	return sum / time.Duration(len(durations))
}

func generateWeightsFile(builders []string, timings [][]pkgTiming) []byte {
	var buf bytes.Buffer
	fmt.Fprintln(&buf, header)
	fmt.Fprintln(&buf, "var allWeights = map[string]map[string]float64{")
	for i, builder := range builders {
		fmt.Fprintf(&buf, "\t%q: {\n", builder)
		for _, pkg := range timings[i] {
			fmt.Fprintf(&buf, "\t%q: %f,\n", pkg.name, pkg.duration.Seconds())
		}
		fmt.Fprintf(&buf, "\t},\n")
	}
	fmt.Fprintln(&buf, "}")
	return buf.Bytes()
}

const header = `// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Code generated by "go run computeweights.go". DO NOT EDIT.

package testweights
`
