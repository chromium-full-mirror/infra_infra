// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package androidapi

import (
	"context"
	"fmt"

	"google.golang.org/api/option"

	"infra/cros/cmd/common_lib/ants/androidbuildinternal/v3"
)

const (
	// defaultMaxResults is the default max number of results to be returned
	// by the list endpoints in android build API.
	defaultMaxResults = int64(1000)
	quotaProject      = "chromeos-bot"
)

var (
	errInit = fmt.Errorf("build client is not initialized")
)

// Service is used for accessing the android build api.
// It is broken into sub services similar to the underlying API.
// All sub services share the same client, so the caller needs to initialize it
// when the service is first used.
type Service struct {
	WorkUnitService      WorkUnitService
	InvocationService    InvocationService
	TestResultService    TestResultService
	TestArtifactsService TestArtifactsService
}

// AndroidBuildAPIOptions represents the common request options
// when communicating with android build API.
type AndroidBuildAPIOptions struct {
	// The optional page token for the request.
	// If empty, the first page will be returned.
	PageToken string

	// The max results to be returned by the API.
	// If empty, the default max results will be used.
	MaxResults int64
}

// NewAndroidBuildService returns a new service which is used to interact with the android build api.
// It initializes the build client depending on rt RunType(environment SA, container or local)
func NewAndroidBuildService(ctx context.Context, rt RunType) (*Service, error) {
	creds, err := FetchCredentials(rt)
	if err != nil {
		return nil, err
	}

	opts := []option.ClientOption{
		option.WithTokenSource(creds.TokenSource),
	}

	// This breaks the other run types.
	if rt == LOCAL {
		opts = append(opts, option.WithQuotaProject(quotaProject))
	}

	client, err := androidbuildinternal.NewService(ctx, opts...)
	if err != nil {
		return nil, err
	}

	service := &Service{}
	service.WorkUnitService = &WorkUnitServiceImpl{client.Workunit}
	service.InvocationService = &InvocationServiceImpl{client.Invocation}
	service.TestResultService = &TestResultServiceImpl{client.Testresult}
	service.TestArtifactsService = &TestArtifactsServiceImpl{client.Testartifact}
	return service, nil
}
