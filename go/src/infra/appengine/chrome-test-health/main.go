// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/proto/protowalk"
	"go.chromium.org/luci/grpc/appstatus"
	"go.chromium.org/luci/server"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/openid"
	"go.chromium.org/luci/server/auth/rpcacl"
	"go.chromium.org/luci/server/cron"
	"go.chromium.org/luci/server/encryptedcookies"
	// Store auth sessions in the datastore.
	_ "go.chromium.org/luci/server/encryptedcookies/session/datastore"
	"go.chromium.org/luci/server/gaeemulation"
	"go.chromium.org/luci/server/module"
	"go.chromium.org/luci/server/secrets"

	"go.chromium.org/infra/appengine/chrome-test-health/api"
	"go.chromium.org/infra/appengine/chrome-test-health/internal/coverage"
	covCron "go.chromium.org/infra/appengine/chrome-test-health/internal/coverage/cron"
	"go.chromium.org/infra/appengine/chrome-test-health/internal/testmetrics"
)

const (
	serviceAccessGroup = "project-chrome-test-health-access"
	dataOwnersGroup    = "mdb/chrome-browser-infra"
)

var (
	stats *testResourcesServer
	cov   *coverageServer
	// RPC-level ACLs.
	rpcACL = rpcacl.Map{
		"/discovery.Discovery/*":                                     serviceAccessGroup,
		"/test_resources.Stats/UpdateMetricsTable":                   dataOwnersGroup,
		"/test_resources.Stats/ListComponents":                       serviceAccessGroup,
		"/test_resources.Stats/FetchTestMetrics":                     serviceAccessGroup,
		"/test_resources.Stats/FetchDirectoryMetrics":                serviceAccessGroup,
		"/test_resources.Coverage/GetProjectDefaultConfig":           serviceAccessGroup,
		"/test_resources.Coverage/GetCoverageSummary":                serviceAccessGroup,
		"/test_resources.Coverage/GetAbsoluteCoverageDataOneYear":    serviceAccessGroup,
		"/test_resources.Coverage/GetIncrementalCoverageDataOneYear": serviceAccessGroup,
	}
	// Data set to work with
	dataSet = flag.String(
		"data-set",
		"test_results",
		"The data set to use (e.g. test_results_test for testing).",
	)
	// Flag to reference GCP project Findit.
	finditCloudProject = flag.String(
		"findit-cloud-project",
		"findit-for-me-staging",
		"Findit's cloud project required to query the data for new coverage dashboard.",
	)
)

const (
	// Constants related to Coverage related APIs
	luciBuilderFormat = `^[a-zA-Z0-9\-_.\(\) ]{1,128}$`
	luciBucketFormat  = `^[a-z0-9\-_.]{1,100}$`
)

type Client interface {
	UpdateSummary(ctx context.Context, fromDate civil.Date, toDate civil.Date) error
	ListComponents(ctx context.Context, req *api.ListComponentsRequest) (*api.ListComponentsResponse, error)
	FetchMetrics(ctx context.Context, req *api.FetchTestMetricsRequest) (*api.FetchTestMetricsResponse, error)
	FetchDirectoryMetrics(ctx context.Context, req *api.FetchDirectoryMetricsRequest) (*api.FetchDirectoryMetricsResponse, error)
}

type CoverageClient interface {
	GetProjectDefaultConfig(ctx context.Context, req *api.GetProjectDefaultConfigRequest) (*api.GetProjectDefaultConfigResponse, error)
	GetCoverageSummary(ctx context.Context, req *api.GetCoverageSummaryRequest) (*api.GetCoverageSummaryResponse, error)
	GetAbsoluteCoverageDataOneYear(
		ctx context.Context,
		req *api.GetAbsoluteCoverageDataOneYearRequest,
	) (*api.GetAbsoluteCoverageDataOneYearResponse, error)
	GetIncrementalCoverageDataOneYear(
		ctx context.Context,
		req *api.GetIncrementalCoverageDataOneYearRequest,
	) (*api.GetIncrementalCoverageDataOneYearResponse, error)
}

type CoverageCronClient interface {
	UpdatePresubmitData(ctx context.Context) error
}

func main() {
	modules := []module.Module{
		cron.NewModuleFromFlags(),
		encryptedcookies.NewModuleFromFlags(), // Required for auth sessions.
		gaeemulation.NewModuleFromFlags(),     // Needed by encryptedcookies.
		secrets.NewModuleFromFlags(),          // Needed by encryptedcookies.
	}
	server.Main(nil, modules, func(srv *server.Server) error {
		client, err := setupClient(srv)
		if err != nil {
			return err
		}
		stats = &testResourcesServer{
			Client: client,
		}

		coverageClient, err := setupCoverageClient(srv)
		if err != nil {
			return err
		}
		coverageCronClient, err := setupCoverageCronClient(srv)
		if err != nil {
			return err
		}
		cov = &coverageServer{
			Client:     coverageClient,
			CronClient: coverageCronClient,
		}

		cron.RegisterHandler("update-daily-summary", updateDailySummary)
		cron.RegisterHandler("update-presubmit-data", updatePresubmitData)

		// All RPC APIs.
		api.RegisterStatsServer(srv, stats)
		api.RegisterCoverageServer(srv, cov)

		// Authentication methods for RPC APIs.
		srv.SetRPCAuthMethods([]auth.Method{
			// The preferred authentication method.
			&openid.GoogleIDTokenAuthMethod{
				AudienceCheck: openid.AudienceMatchesHost,
				SkipNonJWT:    true, // pass OAuth2 access tokens through
			},
			// Backward compatibility for the RPC Explorer and old clients.
			&auth.GoogleOAuth2Method{
				Scopes: []string{"https://www.googleapis.com/auth/userinfo.email"},
			},
		})

		// Per-RPC authorization interceptor.
		srv.RegisterUnifiedServerInterceptors(rpcacl.Interceptor(rpcACL))
		return nil
	})
}

func updateDailySummary(ctx context.Context) error {
	today := time.Now().Format("2006-01-02")
	cDate, err := civil.ParseDate(today)
	if err != nil {
		logging.Errorf(ctx, "Failed parsing current date: %s", err)
		return err
	}

	go func() {
		deadlineCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Hour*2))
		defer cancel()
		startTime := time.Now()

		// Update today and yesterday. Average cores for instance will need the
		// total day seconds included to finalize it's value
		err = stats.Client.UpdateSummary(deadlineCtx, cDate.AddDays(-1), cDate)
		updateRuntime := time.Since(startTime)
		if err != nil {
			logging.Errorf(deadlineCtx, "Failed updating current date: %s which took %s seconds", err, updateRuntime.Seconds())
		} else {
			logging.Infof(deadlineCtx, "Succeeded updating current date: %s which took %s seconds", err, updateRuntime.Seconds())
		}
	}()
	return nil
}

func setupClient(srv *server.Server) (*testmetrics.Client, error) {
	bqClient, err := bigquery.NewClient(srv.Context, srv.Options.CloudProject)
	if err != nil {
		return nil, err
	}
	var client = &testmetrics.Client{
		BqClient:  bqClient,
		ProjectId: "`" + srv.Options.CloudProject + "`",
		DataSet:   *dataSet,
	}
	err = client.Init("")
	if err != nil {
		return nil, err
	}
	return client, nil
}

func setupCoverageClient(srv *server.Server) (*coverage.Client, error) {
	var client = &coverage.Client{
		ChromeTestHealthCloudProject: srv.Options.CloudProject,
		FinditCloudProject:           *finditCloudProject,
	}
	err := client.Init(srv.Context)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func setupCoverageCronClient(srv *server.Server) (*covCron.CronClient, error) {
	client, err := covCron.NewClient(srv.Context, *finditCloudProject, srv.Options.CloudProject)
	if err != nil {
		return nil, err
	}
	return client, nil
}

type testResourcesServer struct {
	Client Client
}

type coverageServer struct {
	Client     CoverageClient
	CronClient CoverageCronClient
}

var getProjectDefaultConfigRequestWalker = protowalk.NewWalker[*api.GetProjectDefaultConfigRequest](&protowalk.RequiredProcessor{})

func (covServer *coverageServer) GetProjectDefaultConfig(ctx context.Context, req *api.GetProjectDefaultConfigRequest) (*api.GetProjectDefaultConfigResponse, error) {
	if err := validateRequest(ctx, getProjectDefaultConfigRequestWalker, req); err != nil {
		return nil, appstatus.Errorf(codes.InvalidArgument, "%s", err.Error())
	}

	if isValidProject := validateFormat(req.LuciProject, `^[a-z0-9-_]+$`); !isValidProject {
		logging.Errorf(ctx, "Argument project did not match required format")
		return nil, appstatus.Errorf(codes.InvalidArgument, "Argument Project is invalid")
	}

	resp, err := covServer.Client.GetProjectDefaultConfig(ctx, req)
	if err != nil {
		logging.Errorf(ctx, "Error fetching the default Config: %s", err)
		return nil, err
	}
	return resp, nil
}

func (covServer *coverageServer) GetCoverageSummary(ctx context.Context, req *api.GetCoverageSummaryRequest) (*api.GetCoverageSummaryResponse, error) {
	requiredFields := []any{
		[]string{"Gitiles Host", req.GitilesHost, ""},
		[]string{"Gitiles Project", req.GitilesProject, ""},
		[]string{"Gitiles Ref", req.GitilesRef, ""},
		[]string{"Gitiles Revision", req.GitilesRevision, ""},
		[]string{"Builder", req.Builder, luciBuilderFormat},
		[]string{"Bucket", req.Bucket, luciBucketFormat},
	}

	for _, field := range requiredFields {
		fieldName := field.([]string)[0]
		fieldValue := field.([]string)[1]
		fieldRegex := field.([]string)[2]
		if isPresent := validatePresence(fieldValue); !isPresent {
			return nil, appstatus.Errorf((codes.InvalidArgument), "%s is a required argument", fieldName)
		}
		if isValidFormat := validateFormat(fieldValue, fieldRegex); !isValidFormat {
			return nil, appstatus.Errorf((codes.InvalidArgument), "%s is not provided in required format", fieldName)
		}
	}

	isPathPresent := validatePresence(req.Path)
	isComponentsListPresent := validatePresence(req.Components)
	if isPathPresent && isComponentsListPresent {
		return nil, appstatus.Errorf((codes.InvalidArgument), "Either path or components should be specified not both")
	}
	if !isPathPresent && !isComponentsListPresent {
		return nil, appstatus.Errorf((codes.InvalidArgument), "Either path or components should be specified")
	}

	resp, err := covServer.Client.GetCoverageSummary(ctx, req)
	if err != nil {
		logging.Errorf(ctx, "Error fetching the coverage summary: %s", err)
		return nil, err
	}
	return resp, nil
}

func (covServer *coverageServer) GetAbsoluteCoverageDataOneYear(
	ctx context.Context,
	req *api.GetAbsoluteCoverageDataOneYearRequest,
) (*api.GetAbsoluteCoverageDataOneYearResponse, error) {
	if isBuilderPresent := validatePresence(req.Builder); !isBuilderPresent {
		return nil, appstatus.Errorf((codes.InvalidArgument), "Builder is a required argument")
	}

	if isValidBuilder := validateFormat(req.Builder, luciBuilderFormat); !isValidBuilder {
		return nil, appstatus.Errorf((codes.InvalidArgument), "Builder is not provided in required format")
	}

	if isBucketPresent := validatePresence(req.Bucket); !isBucketPresent {
		return nil, appstatus.Errorf((codes.InvalidArgument), "Bucket is a required argument")
	}

	if isValidBucket := validateFormat(req.Bucket, luciBuilderFormat); !isValidBucket {
		return nil, appstatus.Errorf((codes.InvalidArgument), "Bucket is not provided in required format")
	}

	isPathListPresent := validatePresence(req.Paths)
	isComponentsListPresent := validatePresence(req.Components)

	if !isPathListPresent && !isComponentsListPresent {
		return nil, appstatus.Errorf((codes.InvalidArgument), "Either paths or components should be specified")
	}

	resp, err := covServer.Client.GetAbsoluteCoverageDataOneYear(ctx, req)
	if err != nil {
		logging.Errorf(ctx, "Error fetching the absolute coverage stats: %s", err)
		return nil, err
	}
	return resp, nil
}

func (covServer *coverageServer) GetIncrementalCoverageDataOneYear(
	ctx context.Context,
	req *api.GetIncrementalCoverageDataOneYearRequest,
) (*api.GetIncrementalCoverageDataOneYearResponse, error) {
	isPathListPresent := validatePresence(req.Paths)
	if !isPathListPresent {
		return nil, appstatus.Errorf((codes.InvalidArgument), "Paths should be specified")
	}

	for _, path := range req.Paths {
		if !pathRelativeToRoot(path) {
			return nil, appstatus.Errorf(
				(codes.InvalidArgument),
				"Path %s is not relative to root, it should start with //",
				path,
			)
		}
	}

	resp, err := covServer.Client.GetIncrementalCoverageDataOneYear(ctx, req)
	if err != nil {
		logging.Errorf(ctx, "Error fetching the incremental coverage stats: %s", err)
		return nil, err
	}
	return resp, nil
}

func updatePresubmitData(ctx context.Context) error {
	deadlineCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Hour*2))
	defer cancel()
	err := cov.CronClient.UpdatePresubmitData(deadlineCtx)
	if err != nil {
		logging.Errorf(deadlineCtx, "Presubmit: Failed backfilling data. Error: %w", err)
		return err
	}
	return nil
}

var updateMetricsRequiredWalker = protowalk.NewWalker[*api.UpdateMetricsTableRequest](&protowalk.RequiredProcessor{})

func (s *testResourcesServer) UpdateMetricsTable(ctx context.Context, req *api.UpdateMetricsTableRequest) (*api.UpdateMetricsTableResponse, error) {
	if err := validateRequest(ctx, updateMetricsRequiredWalker, req); err != nil {
		return nil, appstatus.Errorf(codes.InvalidArgument, "%s", err.Error())
	}
	fromDate, err := civil.ParseDate(req.FromDate)
	if err != nil {
		return nil, appstatus.Errorf(codes.InvalidArgument, "%s", err.Error())
	}
	toDate, err := civil.ParseDate(req.ToDate)
	if err != nil {
		return nil, appstatus.Errorf(codes.InvalidArgument, "%s", err.Error())
	}

	go func() {
		deadlineCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Hour*2))
		defer cancel()
		err = s.Client.UpdateSummary(deadlineCtx, fromDate, toDate)
		if err != nil {
			logging.Errorf(deadlineCtx, "Failed backfilling days %s - %s: %s", fromDate, toDate, err)
		} else {
			logging.Infof(deadlineCtx, "Succeeded backfilling days %s - %s: %s", fromDate, toDate, err)
		}
	}()
	return &api.UpdateMetricsTableResponse{}, nil
}

func (s *testResourcesServer) ListComponents(ctx context.Context, req *api.ListComponentsRequest) (*api.ListComponentsResponse, error) {
	resp, err := s.Client.ListComponents(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

var fetchDirectoryMetricsRequestWalker = protowalk.NewWalker[*api.FetchDirectoryMetricsRequest](&protowalk.RequiredProcessor{})

func (s *testResourcesServer) FetchDirectoryMetrics(ctx context.Context, req *api.FetchDirectoryMetricsRequest) (*api.FetchDirectoryMetricsResponse, error) {
	if err := validateRequest(ctx, fetchDirectoryMetricsRequestWalker, req); err != nil {
		return nil, appstatus.Errorf(codes.InvalidArgument, "%s", err.Error())
	}
	resp, err := s.Client.FetchDirectoryMetrics(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

var fetchTestMetricsRequestWalker = protowalk.NewWalker[*api.FetchTestMetricsRequest](&protowalk.RequiredProcessor{})

func (s *testResourcesServer) FetchTestMetrics(ctx context.Context, req *api.FetchTestMetricsRequest) (*api.FetchTestMetricsResponse, error) {
	if err := validateRequest(ctx, fetchTestMetricsRequestWalker, req); err != nil {
		return nil, appstatus.Errorf(codes.InvalidArgument, "%s", err.Error())
	}
	resp, err := s.Client.FetchMetrics(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func validateRequest[T proto.Message](ctx context.Context, requiredChecker protowalk.Walker[T], req T) error {
	if procRes := requiredChecker.Execute(req); procRes != nil {
		if resStrs := procRes.Strings(); len(resStrs) > 0 {
			logging.Infof(ctx, strings.Join(resStrs, ". "))
		}
		if err := procRes.Err(); err != nil {
			return err
		}
	}
	return nil
}

// validatePresence takes in an interface{} and checks if
// the it's present (not nil). In case of string it also
// checks if the string is empty.
func validatePresence(value any) bool {
	if value == nil {
		return false
	}
	if fmt.Sprintf("%T", value) == "string" && len(strings.TrimSpace(value.(string))) == 0 {
		return false
	}
	if fmt.Sprintf("%T", value) == "[]string" && len(value.([]string)) == 0 {
		return false
	}
	return true
}

// pathRelativeToRoot checks if the given path is relative to
// project root, ie: //
func pathRelativeToRoot(path string) bool {
	return strings.HasPrefix(path, "//") && filepath.IsAbs(path)
}

// validateFormat takes in a value, pattern as arguments and
// checks if the value matches the regex pattern provided.
func validateFormat(value string, pattern string) bool {
	if match, _ := regexp.MatchString(pattern, value); !match {
		return false
	}
	return true
}
