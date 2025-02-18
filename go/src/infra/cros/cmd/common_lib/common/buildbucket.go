// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"go.chromium.org/chromiumos/infra/proto/go/test_platform/config"
	"go.chromium.org/luci/auth"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"
)

// NewBBClient creates new bb client.
func NewBBClient(ctx context.Context) (buildbucketpb.BuildsClient, error) {
	hClient, err := HTTPClient(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "create buildbucket client").Err()
	}
	pClient := &prpc.Client{
		C:    hClient,
		Host: "cr-buildbucket.appspot.com",
	}
	return buildbucketpb.NewBuildsPRPCClient(pClient), nil
}

// HTTPClient creates a http client.
func HTTPClient(ctx context.Context) (*http.Client, error) {
	a := auth.NewAuthenticator(ctx, auth.SilentLogin, auth.Options{
		Scopes: []string{auth.OAuthScopeEmail},
	})
	h, err := a.Client()
	if err != nil {
		return nil, errors.Annotate(err, "create http client").Err()
	}
	return h, nil
}

// TestRunnerBuilderID returns builderid for test_runner.
func TestRunnerBuilderID(conf *config.Config) *buildbucketpb.BuilderID {
	bbConfig := conf.GetTestRunner().GetBuildbucket()
	if bbConfig != nil {
		return &buildbucketpb.BuilderID{
			Project: bbConfig.GetProject(),
			Bucket:  bbConfig.GetBucket(),
			Builder: bbConfig.GetBuilder(),
		}
	}
	return &buildbucketpb.BuilderID{
		Project: "chromeos",
		Bucket:  "test_runner",
		Builder: "test_runner",
	}
}

// BBUrl returns the Buildbucket URL of the task.
func BBUrl(builderID *buildbucketpb.BuilderID, bbID int64) string {
	return fmt.Sprintf("https://ci.chromium.org/p/%s/builders/%s/%s/b%d", builderID.Project, builderID.Bucket, builderID.Builder, bbID)
}

func ShortBBUrl(bbID int64) string {
	return fmt.Sprintf("http://ci.chromium.org/b/%d/infra", bbID)
}

func IsLedRun(builderID *buildbucketpb.BuilderID) bool {
	return strings.Contains(builderID.Bucket, "shadow")
}

type Environment int

const (
	Unknown Environment = iota
	Prod
	Staging
	Dev
)

func (e Environment) String() string {
	switch e {
	case Unknown:
		return "UNKNOWN"
	case Prod:
		return "PROD"
	case Staging:
		return "STAGING"
	case Dev:
		return "DEV"
	default:
		return ""
	}
}

func GetCTPEnvironment(builderID *buildbucketpb.BuilderID) Environment {
	if IsDev(builderID) {
		return Dev
	} else if IsStaging(builderID) {
		return Staging
	} else {
		return Prod
	}
}

func IsProd(builderID *buildbucketpb.BuilderID) bool {
	return !IsDev(builderID) && !IsStaging(builderID)
}

func IsStaging(builderID *buildbucketpb.BuilderID) bool {
	return strings.Contains(builderID.Builder, "staging")
}

func IsDev(builderID *buildbucketpb.BuilderID) bool {
	return strings.Contains(builderID.Builder, "dev")
}

func GetAtpProjectID(builderID *buildbucketpb.BuilderID) string {
	if IsDev(builderID) {
		return ATPSwitcherProjectIDAlpha
	} else if IsStaging(builderID) {
		return ATPSwitcherProjectIDStaging
	}

	// Default to prod
	return ATPSwitcherProjectIDProd
}
