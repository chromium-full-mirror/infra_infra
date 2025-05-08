// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package moblab

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"time"

	"cloud.google.com/go/longrunning"
	lroauto "cloud.google.com/go/longrunning/autogen"
	"github.com/golang/protobuf/proto"
	gax "github.com/googleapis/gax-go/v2"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/api/option/internaloption"
	gtransport "google.golang.org/api/transport/grpc"
	moblabpb "google.golang.org/genproto/googleapis/chromeos/moblab/v1beta1"
	longrunningpb "google.golang.org/genproto/googleapis/longrunning"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var newBuildClientHook clientHook

// BuildCallOptions contains the retry settings for each method of BuildClient.
type BuildCallOptions struct {
	ListBuildTargets      []gax.CallOption
	ListModels            []gax.CallOption
	ListBuilds            []gax.CallOption
	CheckBuildStageStatus []gax.CallOption
	StageBuild            []gax.CallOption
	FindMostStableBuild   []gax.CallOption
}

func defaultBuildClientOptions() []option.ClientOption {
	return []option.ClientOption{
		internaloption.WithDefaultEndpoint("chromeosmoblab.googleapis.com:443"),
		internaloption.WithDefaultMTLSEndpoint("chromeosmoblab.mtls.googleapis.com:443"),
		internaloption.WithDefaultAudience("https://chromeosmoblab.googleapis.com/"),
		internaloption.WithDefaultScopes(DefaultAuthScopes()...),
		option.WithGRPCDialOption(grpc.WithDisableServiceConfig()),
		option.WithGRPCDialOption(grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(math.MaxInt32))),
	}
}

func defaultBuildCallOptions() *BuildCallOptions {
	return &BuildCallOptions{
		ListBuildTargets:      []gax.CallOption{},
		ListModels:            []gax.CallOption{},
		ListBuilds:            []gax.CallOption{},
		CheckBuildStageStatus: []gax.CallOption{},
		StageBuild:            []gax.CallOption{},
		FindMostStableBuild:   []gax.CallOption{},
	}
}

// BuildClient is a client for interacting with .
//
// Methods, except Close, may be called concurrently. However, fields must not be modified concurrently with method calls.
type BuildClient struct {
	// Connection pool of gRPC connections to the service.
	connPool gtransport.ConnPool

	// flag to opt out of default deadlines via GOOGLE_API_GO_EXPERIMENTAL_DISABLE_DEFAULT_DEADLINE
	disableDeadlines bool

	// The gRPC API client.
	buildClient moblabpb.BuildServiceClient

	// LROClient is used internally to handle longrunning operations.
	// It is exposed so that its CallOptions can be modified if required.
	// Users should not Close this client.
	LROClient *lroauto.OperationsClient

	// The call options for this service.
	CallOptions *BuildCallOptions

	// The x-goog-* metadata to be sent with each request.
	xGoogMetadata metadata.MD
}

// NewBuildClient creates a new build service client.
//
// Manages Chrome OS build services.
func NewBuildClient(ctx context.Context, opts ...option.ClientOption) (*BuildClient, error) {
	clientOpts := defaultBuildClientOptions()

	if newBuildClientHook != nil {
		hookOpts, err := newBuildClientHook(ctx, clientHookParams{})
		if err != nil {
			return nil, err
		}
		clientOpts = append(clientOpts, hookOpts...)
	}

	disableDeadlines, err := checkDisableDeadlines()
	if err != nil {
		return nil, err
	}

	connPool, err := gtransport.DialPool(ctx, append(clientOpts, opts...)...)
	if err != nil {
		return nil, err
	}
	c := &BuildClient{
		connPool:         connPool,
		disableDeadlines: disableDeadlines,
		CallOptions:      defaultBuildCallOptions(),

		buildClient: moblabpb.NewBuildServiceClient(connPool),
	}
	c.setGoogleClientInfo()

	c.LROClient, err = lroauto.NewOperationsClient(ctx, gtransport.WithConnPool(connPool))
	if err != nil {
		// This error "should not happen", since we are just reusing old connection pool
		// and never actually need to dial.
		// If this does happen, we could leak connp. However, we cannot close conn:
		// If the user invoked the constructor with option.WithGRPCConn,
		// we would close a connection that's still in use.
		// TODO: investigate error conditions.
		return nil, err
	}
	return c, nil
}

// Connection returns a connection to the API service.
//
// Deprecated.
func (c *BuildClient) Connection() *grpc.ClientConn {
	return c.connPool.Conn()
}

// Close closes the connection to the API service. The user should invoke this when
// the client is no longer required.
func (c *BuildClient) Close() error {
	return c.connPool.Close()
}

// setGoogleClientInfo sets the name and version of the application in
// the `x-goog-api-client` header passed on each request. Intended for
// use by Google-written clients.
func (c *BuildClient) setGoogleClientInfo(keyval ...string) {
	kv := append([]string{"gl-go", versionGo()}, keyval...)
	kv = append(kv, "gapic", versionClient, "gax", gax.Version, "grpc", grpc.Version)
	c.xGoogMetadata = metadata.Pairs("x-goog-api-client", gax.XGoogHeader(kv...))
}

// ListBuildTargets lists all build targets that a user has access to.
func (c *BuildClient) ListBuildTargets(ctx context.Context, req *moblabpb.ListBuildTargetsRequest, opts ...gax.CallOption) *BuildTargetIterator {
	ctx = insertMetadata(ctx, c.xGoogMetadata)
	opts = append(c.CallOptions.ListBuildTargets[0:len(c.CallOptions.ListBuildTargets):len(c.CallOptions.ListBuildTargets)], opts...)
	it := &BuildTargetIterator{}
	req = proto.Clone(req).(*moblabpb.ListBuildTargetsRequest)
	it.InternalFetch = func(pageSize int, pageToken string) ([]*moblabpb.BuildTarget, string, error) {
		var resp *moblabpb.ListBuildTargetsResponse
		req.PageToken = pageToken
		if pageSize > math.MaxInt32 {
			req.PageSize = math.MaxInt32
		} else {
			req.PageSize = int32(pageSize)
		}
		err := gax.Invoke(ctx, func(ctx context.Context, settings gax.CallSettings) error {
			var err error
			resp, err = c.buildClient.ListBuildTargets(ctx, req, settings.GRPC...)
			return err
		}, opts...)
		if err != nil {
			return nil, "", err
		}

		it.Response = resp
		return resp.GetBuildTargets(), resp.GetNextPageToken(), nil
	}
	fetch := func(pageSize int, pageToken string) (string, error) {
		items, nextPageToken, err := it.InternalFetch(pageSize, pageToken)
		if err != nil {
			return "", err
		}
		it.items = append(it.items, items...)
		return nextPageToken, nil
	}
	it.pageInfo, it.nextFunc = iterator.NewPageInfo(fetch, it.bufLen, it.takeBuf)
	it.pageInfo.MaxSize = int(req.GetPageSize())
	it.pageInfo.Token = req.GetPageToken()
	return it
}

// ListModels lists all models for the given build target.
func (c *BuildClient) ListModels(ctx context.Context, req *moblabpb.ListModelsRequest, opts ...gax.CallOption) *ModelIterator {
	md := metadata.Pairs("x-goog-request-params", fmt.Sprintf("%s=%v", "parent", url.QueryEscape(req.GetParent())))
	ctx = insertMetadata(ctx, c.xGoogMetadata, md)
	opts = append(c.CallOptions.ListModels[0:len(c.CallOptions.ListModels):len(c.CallOptions.ListModels)], opts...)
	it := &ModelIterator{}
	req = proto.Clone(req).(*moblabpb.ListModelsRequest)
	it.InternalFetch = func(pageSize int, pageToken string) ([]*moblabpb.Model, string, error) {
		var resp *moblabpb.ListModelsResponse
		req.PageToken = pageToken
		if pageSize > math.MaxInt32 {
			req.PageSize = math.MaxInt32
		} else {
			req.PageSize = int32(pageSize)
		}
		err := gax.Invoke(ctx, func(ctx context.Context, settings gax.CallSettings) error {
			var err error
			resp, err = c.buildClient.ListModels(ctx, req, settings.GRPC...)
			return err
		}, opts...)
		if err != nil {
			return nil, "", err
		}

		it.Response = resp
		return resp.GetModels(), resp.GetNextPageToken(), nil
	}
	fetch := func(pageSize int, pageToken string) (string, error) {
		items, nextPageToken, err := it.InternalFetch(pageSize, pageToken)
		if err != nil {
			return "", err
		}
		it.items = append(it.items, items...)
		return nextPageToken, nil
	}
	it.pageInfo, it.nextFunc = iterator.NewPageInfo(fetch, it.bufLen, it.takeBuf)
	it.pageInfo.MaxSize = int(req.GetPageSize())
	it.pageInfo.Token = req.GetPageToken()
	return it
}

// ListBuilds lists all builds for the given build target and model in descending order
// for the milestones and build versions.
func (c *BuildClient) ListBuilds(ctx context.Context, req *moblabpb.ListBuildsRequest, opts ...gax.CallOption) *BuildIterator {
	md := metadata.Pairs("x-goog-request-params", fmt.Sprintf("%s=%v", "parent", url.QueryEscape(req.GetParent())))
	ctx = insertMetadata(ctx, c.xGoogMetadata, md)
	opts = append(c.CallOptions.ListBuilds[0:len(c.CallOptions.ListBuilds):len(c.CallOptions.ListBuilds)], opts...)
	it := &BuildIterator{}
	req = proto.Clone(req).(*moblabpb.ListBuildsRequest)
	it.InternalFetch = func(pageSize int, pageToken string) ([]*moblabpb.Build, string, error) {
		var resp *moblabpb.ListBuildsResponse
		req.PageToken = pageToken
		if pageSize > math.MaxInt32 {
			req.PageSize = math.MaxInt32
		} else {
			req.PageSize = int32(pageSize)
		}
		err := gax.Invoke(ctx, func(ctx context.Context, settings gax.CallSettings) error {
			var err error
			resp, err = c.buildClient.ListBuilds(ctx, req, settings.GRPC...)
			return err
		}, opts...)
		if err != nil {
			return nil, "", err
		}

		it.Response = resp
		return resp.GetBuilds(), resp.GetNextPageToken(), nil
	}
	fetch := func(pageSize int, pageToken string) (string, error) {
		items, nextPageToken, err := it.InternalFetch(pageSize, pageToken)
		if err != nil {
			return "", err
		}
		it.items = append(it.items, items...)
		return nextPageToken, nil
	}
	it.pageInfo, it.nextFunc = iterator.NewPageInfo(fetch, it.bufLen, it.takeBuf)
	it.pageInfo.MaxSize = int(req.GetPageSize())
	it.pageInfo.Token = req.GetPageToken()
	return it
}

// CheckBuildStageStatus checks the stage status for a given build artifact in a partner Google
// Cloud Storage bucket.
func (c *BuildClient) CheckBuildStageStatus(ctx context.Context, req *moblabpb.CheckBuildStageStatusRequest, opts ...gax.CallOption) (*moblabpb.CheckBuildStageStatusResponse, error) {
	md := metadata.Pairs("x-goog-request-params", fmt.Sprintf("%s=%v", "name", url.QueryEscape(req.GetName())))
	ctx = insertMetadata(ctx, c.xGoogMetadata, md)
	opts = append(c.CallOptions.CheckBuildStageStatus[0:len(c.CallOptions.CheckBuildStageStatus):len(c.CallOptions.CheckBuildStageStatus)], opts...)
	var resp *moblabpb.CheckBuildStageStatusResponse
	err := gax.Invoke(ctx, func(ctx context.Context, settings gax.CallSettings) error {
		var err error
		resp, err = c.buildClient.CheckBuildStageStatus(ctx, req, settings.GRPC...)
		return err
	}, opts...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// StageBuild stages a given build artifact from a internal Google Cloud Storage bucket
// to a partner Google Cloud Storage bucket. If any of objects has already
// been copied, it will overwrite the previous objects. Operation <response:
// StageBuildResponse,
// metadata: StageBuildMetadata>
func (c *BuildClient) StageBuild(ctx context.Context, req *moblabpb.StageBuildRequest, opts ...gax.CallOption) (*StageBuildOperation, error) {
	md := metadata.Pairs("x-goog-request-params", fmt.Sprintf("%s=%v", "name", url.QueryEscape(req.GetName())))
	ctx = insertMetadata(ctx, c.xGoogMetadata, md)
	opts = append(c.CallOptions.StageBuild[0:len(c.CallOptions.StageBuild):len(c.CallOptions.StageBuild)], opts...)
	var resp *longrunningpb.Operation
	err := gax.Invoke(ctx, func(ctx context.Context, settings gax.CallSettings) error {
		var err error
		resp, err = c.buildClient.StageBuild(ctx, req, settings.GRPC...)
		return err
	}, opts...)
	if err != nil {
		return nil, err
	}
	return &StageBuildOperation{
		lro: longrunning.InternalNewOperation(c.LROClient, resp),
	}, nil
}

// FindMostStableBuild finds the most stable build for the given build target. The definition of
// the most stable build is determined by evaluating the following rules in
// order until one is true. If none are true, then there is no stable build
// and it will return an empty response.
//
// Evaluation rules:
//
// Stable channel build with label “Live”
//
// Beta channel build with label “Live”
//
// Dev channel build with label “Live”
//
// # Most recent stable channel build with build status Pass
//
// # Most recent beta channel build with build status Pass
//
// Most recent dev channel build with build status Pass
func (c *BuildClient) FindMostStableBuild(ctx context.Context, req *moblabpb.FindMostStableBuildRequest, opts ...gax.CallOption) (*moblabpb.FindMostStableBuildResponse, error) {
	md := metadata.Pairs("x-goog-request-params", fmt.Sprintf("%s=%v", "build_target", url.QueryEscape(req.GetBuildTarget())))
	ctx = insertMetadata(ctx, c.xGoogMetadata, md)
	opts = append(c.CallOptions.FindMostStableBuild[0:len(c.CallOptions.FindMostStableBuild):len(c.CallOptions.FindMostStableBuild)], opts...)
	var resp *moblabpb.FindMostStableBuildResponse
	err := gax.Invoke(ctx, func(ctx context.Context, settings gax.CallSettings) error {
		var err error
		resp, err = c.buildClient.FindMostStableBuild(ctx, req, settings.GRPC...)
		return err
	}, opts...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// StageBuildOperation manages a long-running operation from StageBuild.
type StageBuildOperation struct {
	lro *longrunning.Operation
}

// StageBuildOperation returns a new StageBuildOperation from a given name.
// The name must be that of a previously created StageBuildOperation, possibly from a different process.
func (c *BuildClient) StageBuildOperation(name string) *StageBuildOperation {
	return &StageBuildOperation{
		lro: longrunning.InternalNewOperation(c.LROClient, &longrunningpb.Operation{Name: name}),
	}
}

// Wait blocks until the long-running operation is completed, returning the response and any errors encountered.
//
// See documentation of Poll for error-handling information.
func (op *StageBuildOperation) Wait(ctx context.Context, opts ...gax.CallOption) (*moblabpb.StageBuildResponse, error) {
	var resp moblabpb.StageBuildResponse
	if err := op.lro.WaitWithInterval(ctx, &resp, time.Minute, opts...); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Poll fetches the latest state of the long-running operation.
//
// Poll also fetches the latest metadata, which can be retrieved by Metadata.
//
// If Poll fails, the error is returned and op is unmodified. If Poll succeeds and
// the operation has completed with failure, the error is returned and op.Done will return true.
// If Poll succeeds and the operation has completed successfully,
// op.Done will return true, and the response of the operation is returned.
// If Poll succeeds and the operation has not completed, the returned response and error are both nil.
func (op *StageBuildOperation) Poll(ctx context.Context, opts ...gax.CallOption) (*moblabpb.StageBuildResponse, error) {
	var resp moblabpb.StageBuildResponse
	if err := op.lro.Poll(ctx, &resp, opts...); err != nil {
		return nil, err
	}
	if !op.Done() {
		return nil, nil
	}
	return &resp, nil
}

// Metadata returns metadata associated with the long-running operation.
// Metadata itself does not contact the server, but Poll does.
// To get the latest metadata, call this method after a successful call to Poll.
// If the metadata is not available, the returned metadata and error are both nil.
func (op *StageBuildOperation) Metadata() (*moblabpb.StageBuildMetadata, error) {
	var meta moblabpb.StageBuildMetadata
	if err := op.lro.Metadata(&meta); err == longrunning.ErrNoMetadata {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &meta, nil
}

// Done reports whether the long-running operation has completed.
func (op *StageBuildOperation) Done() bool {
	return op.lro.Done()
}

// Name returns the name of the long-running operation.
// The name is assigned by the server and is unique within the service from which the operation is created.
func (op *StageBuildOperation) Name() string {
	return op.lro.Name()
}

// BuildIterator manages a stream of *moblabpb.Build.
type BuildIterator struct {
	items    []*moblabpb.Build
	pageInfo *iterator.PageInfo
	nextFunc func() error

	// Response is the raw response for the current page.
	// It must be cast to the RPC response type.
	// Calling Next() or InternalFetch() updates this value.
	Response any

	// InternalFetch is for use by the Google Cloud Libraries only.
	// It is not part of the stable interface of this package.
	//
	// InternalFetch returns results from a single call to the underlying RPC.
	// The number of results is no greater than pageSize.
	// If there are no more results, nextPageToken is empty and err is nil.
	InternalFetch func(pageSize int, pageToken string) (results []*moblabpb.Build, nextPageToken string, err error)
}

// PageInfo supports pagination. See the google.golang.org/api/iterator package for details.
func (it *BuildIterator) PageInfo() *iterator.PageInfo {
	return it.pageInfo
}

// Next returns the next result. Its second return value is iterator.Done if there are no more
// results. Once Next returns Done, all subsequent calls will return Done.
func (it *BuildIterator) Next() (*moblabpb.Build, error) {
	var item *moblabpb.Build
	if err := it.nextFunc(); err != nil {
		return item, err
	}
	item = it.items[0]
	it.items = it.items[1:]
	return item, nil
}

func (it *BuildIterator) bufLen() int {
	return len(it.items)
}

func (it *BuildIterator) takeBuf() any {
	b := it.items
	it.items = nil
	return b
}

// BuildTargetIterator manages a stream of *moblabpb.BuildTarget.
type BuildTargetIterator struct {
	items    []*moblabpb.BuildTarget
	pageInfo *iterator.PageInfo
	nextFunc func() error

	// Response is the raw response for the current page.
	// It must be cast to the RPC response type.
	// Calling Next() or InternalFetch() updates this value.
	Response any

	// InternalFetch is for use by the Google Cloud Libraries only.
	// It is not part of the stable interface of this package.
	//
	// InternalFetch returns results from a single call to the underlying RPC.
	// The number of results is no greater than pageSize.
	// If there are no more results, nextPageToken is empty and err is nil.
	InternalFetch func(pageSize int, pageToken string) (results []*moblabpb.BuildTarget, nextPageToken string, err error)
}

// PageInfo supports pagination. See the google.golang.org/api/iterator package for details.
func (it *BuildTargetIterator) PageInfo() *iterator.PageInfo {
	return it.pageInfo
}

// Next returns the next result. Its second return value is iterator.Done if there are no more
// results. Once Next returns Done, all subsequent calls will return Done.
func (it *BuildTargetIterator) Next() (*moblabpb.BuildTarget, error) {
	var item *moblabpb.BuildTarget
	if err := it.nextFunc(); err != nil {
		return item, err
	}
	item = it.items[0]
	it.items = it.items[1:]
	return item, nil
}

func (it *BuildTargetIterator) bufLen() int {
	return len(it.items)
}

func (it *BuildTargetIterator) takeBuf() any {
	b := it.items
	it.items = nil
	return b
}

// ModelIterator manages a stream of *moblabpb.Model.
type ModelIterator struct {
	items    []*moblabpb.Model
	pageInfo *iterator.PageInfo
	nextFunc func() error

	// Response is the raw response for the current page.
	// It must be cast to the RPC response type.
	// Calling Next() or InternalFetch() updates this value.
	Response any

	// InternalFetch is for use by the Google Cloud Libraries only.
	// It is not part of the stable interface of this package.
	//
	// InternalFetch returns results from a single call to the underlying RPC.
	// The number of results is no greater than pageSize.
	// If there are no more results, nextPageToken is empty and err is nil.
	InternalFetch func(pageSize int, pageToken string) (results []*moblabpb.Model, nextPageToken string, err error)
}

// PageInfo supports pagination. See the google.golang.org/api/iterator package for details.
func (it *ModelIterator) PageInfo() *iterator.PageInfo {
	return it.pageInfo
}

// Next returns the next result. Its second return value is iterator.Done if there are no more
// results. Once Next returns Done, all subsequent calls will return Done.
func (it *ModelIterator) Next() (*moblabpb.Model, error) {
	var item *moblabpb.Model
	if err := it.nextFunc(); err != nil {
		return item, err
	}
	item = it.items[0]
	it.items = it.items[1:]
	return item, nil
}

func (it *ModelIterator) bufLen() int {
	return len(it.items)
}

func (it *ModelIterator) takeBuf() any {
	b := it.items
	it.items = nil
	return b
}
