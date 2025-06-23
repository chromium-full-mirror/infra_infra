// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"fmt"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/recovery/internal/components/cache"
	"go.chromium.org/infra/cros/recovery/internal/components/cft"
	"go.chromium.org/infra/cros/recovery/internal/components/linux"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

const (
	cacheTestFilePath = "gs://cros-lab-servers/caching-backend/downloading-test.txt"
)

// cacheDownloadCheckExec performs download check by cache service.
func cacheDownloadCheckExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	testFilePath := argsMap.AsString(ctx, "test_path", cacheTestFilePath)
	log.Debugf(ctx, "Used file: %s", testFilePath)
	// Requesting convert GC path to caches service path.
	// Example: `http://Addr:8082/download/....`
	downloadPath, err := info.GetAccess().GetCacheUrl(ctx, info.GetDut().Name, testFilePath)
	if err != nil {
		return errors.Annotate(err, "cache download check").Err()
	}
	run := info.DefaultRunner()
	timeout := info.GetExecTimeout()
	curlArgs := []string{"-v"}

	// Update header to enforce download file always fresh.
	header := cache.HTTPRequestHeaders(ctx)
	header["X-NO-CACHE"] = "1"

	out, responseCode, err := linux.CurlURL(ctx, run, timeout, downloadPath, header, curlArgs...)
	log.Debugf(ctx, "Cache download output: %s", out)
	if err != nil {
		cache.RecordCacheAccessFailure(ctx, downloadPath, responseCode)
	}
	return errors.WrapIf(err, "cache download check")
}

const (
	defaultLabServiceAddress = "localhost:1485" // lab-service address
)

func cacheAddressDetectionkExec(ctx context.Context, info *execs.ExecInfo) error {
	scopeAddr := func(addr string) error {
		log.Infof(ctx, "Cache address: %q", addr)
		if err := cft.ServiceAddressToScope(ctx, cft.CacheService, addr); err != nil {
			return errors.Annotate(err, "start foil-provision container").Err()
		}
		return nil
	}
	argsMap := info.GetActionArgs(ctx)
	// Example of cache address for Satlab: 192.168.100.1:8082
	cacheAddr := argsMap.AsString(ctx, "cache_address", "")
	if cacheAddr != "" {
		return scopeAddr(cacheAddr)
	}
	labServiceAddr := argsMap.AsString(ctx, "labservice_address", defaultLabServiceAddress)
	conn, err := common.ConnectWithService(ctx, labServiceAddr)
	if err != nil {
		return errors.Annotate(err, "cache address detection").Err()
	}
	client := labapi.NewInventoryServiceClient(conn)
	if client == nil {
		return errors.New("cache address detection: fail to create client")
	}
	stream, err := client.GetDutTopology(ctx,
		&labapi.GetDutTopologyRequest{
			Id: &labapi.DutTopology_Id{
				Value: info.GetDut().Name,
			},
		},
	)
	if err != nil {
		return errors.Annotate(err, "cache address detection: get dut topology").Err()
	}
	response := &labapi.GetDutTopologyResponse{}
	if err := stream.RecvMsg(response); err != nil {
		return errors.Annotate(err, "cache address detection: parse response").Err()
	}
	for _, dut := range response.GetSuccess().GetDutTopology().GetDuts() {
		if a := dut.GetCacheServer().GetAddress(); a != nil && a.GetAddress() != "" {
			cacheAddr := fmt.Sprintf("%s:%d", a.GetAddress(), a.GetPort())
			return scopeAddr(cacheAddr)
		}
	}
	return errors.Reason("cache address detection: could not get the address").Err()
}

func init() {
	execs.Register("cache_download_check", cacheDownloadCheckExec)
	execs.Register("cache_service_address_detection", cacheAddressDetectionkExec)
}
