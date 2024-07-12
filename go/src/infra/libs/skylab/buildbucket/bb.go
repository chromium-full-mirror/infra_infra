// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package buildbucket provides a buildbucket Client with helper methods to schedule the tasks.
package buildbucket

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	structbuilder "google.golang.org/protobuf/types/known/structpb"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"
	buildbucket_pb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"

	schedulingapi "infra/libs/fleet/scheduling/api"
)

// Set higher priority for admin task to compite with tests.
const defaultTaskPriority = 4

// ScheduleLabpackTaskParams includes the parameters necessary to schedule a labpack task.
type ScheduleLabpackTaskParams struct {
	UnitName         string
	ExpectedDUTState string
	Props            *structbuilder.Struct
	ExtraTags        []string
	// TODO(gregorynisbet): Support map[string]string as dims value.
	ExtraDims map[string]string
	// Bullder custom fields. If not provide default values will be used
	BuilderName    string
	BuilderProject string
	BuilderBucket  string
}

// Client provides helper methods to interact with buildbucket builds.
type Client interface {
	// ScheduleLabpackTask schedules a labpack task.
	// TODO(gregorynisbet): refactor this method to return a structured result.
	// Method is deprecated and will removed in favor of CreateLabpackTask.
	ScheduleLabpackTask(ctx context.Context, params *ScheduleLabpackTaskParams, clientName string) (string, int64, error)
	// CreateLabpackTask create a labpack task.
	CreateLabpackTask(ctx context.Context, params *ScheduleLabpackTaskParams, sc schedulingapi.TaskSchedulingAPI) (string, int64, error)
}

// ClientImpl is the implementation of the Client interface.
type clientImpl struct {
	client buildbucket_pb.BuildsClient
}

// NewClient returns a new client to interact with buildbucket builds.
func NewClient(ctx context.Context, hc *http.Client, options *prpc.Options) (Client, error) {
	if hc == nil {
		return nil, errors.Reason("buildbucket client cannot be created from nil http.Client").Err()
	}
	pClient := &prpc.Client{
		C:       hc,
		Host:    "cr-buildbucket.appspot.com",
		Options: options,
	}

	return &clientImpl{
		client: buildbucket_pb.NewBuildsPRPCClient(pClient),
	}, nil
}

// NewHTTPClient returns an HTTP client with authentication set up.
func NewHTTPClient(ctx context.Context, f *authcli.Flags) (*http.Client, error) {
	o, err := f.Options()
	if err != nil {
		return nil, errors.Annotate(err, "failed to get auth options").Err()
	}
	a := auth.NewAuthenticator(ctx, auth.SilentLogin, o)
	c, err := a.Client()
	if err != nil {
		return nil, errors.Annotate(err, "failed to create HTTP client").Err()
	}
	return c, nil
}

// ScheduleLabpackTask creates new task in build bucket with labpack.
// Method is deprecated and will removed in favor of CreateLabpackTask.
func (c *clientImpl) ScheduleLabpackTask(ctx context.Context, params *ScheduleLabpackTaskParams, clientName string) (string, int64, error) {
	return c.CreateLabpackTask(ctx, params, nil)
}

// CreateLabpackTask creates new task in build bucket with labpack.
func (c *clientImpl) CreateLabpackTask(ctx context.Context, params *ScheduleLabpackTaskParams, sc schedulingapi.TaskSchedulingAPI) (string, int64, error) {
	if params == nil {
		return "", 0, errors.Reason("create labpack task: params cannot be nil").Err()
	}
	dims := make(map[string]string)
	dims["dut_name"] = params.UnitName
	if params.ExpectedDUTState != "" {
		dims["dut_state"] = params.ExpectedDUTState
	}
	for key, value := range params.ExtraDims {
		if _, ok := dims[key]; !ok {
			dims[key] = value
		}
	}

	tags := []string{
		fmt.Sprintf("dut-name:%s", params.UnitName),
	}
	tags = append(tags, params.ExtraTags...)

	tagPairs, err := splitTagPairs(tags)
	if err != nil {
		return "", -1, err
	}
	b := &buildbucket_pb.BuilderID{
		Project: "chromeos",
		Bucket:  "labpack_runner",
		Builder: "labpack_builder",
	}
	if params.BuilderName != "" {
		b.Builder = params.BuilderName
	}
	if params.BuilderProject != "" {
		b.Project = params.BuilderProject
	}
	if params.BuilderBucket != "" {
		b.Bucket = params.BuilderBucket
	}
	bbReq := &buildbucket_pb.ScheduleBuildRequest{
		Builder:    b,
		Properties: params.Props,
		Tags:       tagPairs,
		Dimensions: bbDimensions(dims),
		Priority:   defaultTaskPriority,
	}

	if sc != nil {
		if shouldUseDM, err := sc.ShouldUseDM(ctx); err != nil {
			fmt.Println("Unable to check whether Scheduke should be used. Falling back to BB scheduler.", err)
			// return "", -1, errors.Annotate(err, "create labpack task").Err()
		} else if shouldUseDM {
			t, err := sc.ScheduleTask(ctx, &schedulingapi.ScheduleTaskRequest{
				DeviceName:         params.UnitName,
				BuildbucketRequest: bbReq,
			})
			if err != nil {
				return "", -1, errors.Annotate(err, "create labpack task: scheduling task by scheduke").Err()
			}
			return t.GetUrl(), 0, nil
		}
	}

	build, err := c.client.ScheduleBuild(ctx, bbReq)
	if err != nil {
		return "", -1, err
	}
	url := fmt.Sprintf(BuildURLFmt, params.BuilderProject, params.BuilderBucket, params.BuilderName, build.Id)
	return url, build.Id, nil
}

// BuildURLFmt is the format of a build URL.
const BuildURLFmt = "https://ci.chromium.org/p/%s/builders/%s/%s/b%d"

func splitTagPairs(tags []string) ([]*buildbucket_pb.StringPair, error) {
	ret := make([]*buildbucket_pb.StringPair, 0, len(tags))
	for _, t := range tags {
		p := strings.SplitN(t, ":", 2)
		if len(p) != 2 {
			return nil, errors.Reason("malformed tag %s", t).Err()
		}
		ret = append(ret, &buildbucket_pb.StringPair{
			Key:   strings.Trim(p[0], " "),
			Value: strings.Trim(p[1], " "),
		})
	}
	return ret, nil
}

// bbDimensions converts a map of dimensions to a slice of
// *buildbucket_pb.RequestedDimension.
func bbDimensions(dims map[string]string) []*buildbucket_pb.RequestedDimension {
	ret := make([]*buildbucket_pb.RequestedDimension, len(dims))
	i := 0
	for key, value := range dims {
		ret[i] = &buildbucket_pb.RequestedDimension{
			Key:   strings.Trim(key, " "),
			Value: strings.Trim(value, " "),
		}
		i++
	}
	return ret
}
