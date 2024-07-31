// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tlw

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/tlw/cache"
)

var (
	// TLW_CACHING_PREFERRED_SERVICES is a comma separated list of caching
	// services (each in format of 'http://<server name or IP>:<port>'). When
	// specified, it bypasses the normal cache server selection.
	preferredCachingServices = os.Getenv("TLW_CACHING_PREFERRED_SERVICES")
)

type Server interface {
	CacheForDut(ctx context.Context, rawURL, dutName string) (string, error)
}

func New(ufs cache.UFSClient) (Server, error) {
	ce, err := cache.NewEnv(preferredCachingServices, ufs)
	if err != nil {
		return nil, errors.Reason("newTLWServer: %s", err).Err()
	}
	return &tlwServer{
		cFrontend: cache.NewFrontend(ce),
		ufsClient: ufs,
	}, nil
}

type tlwServer struct {
	cFrontend *cache.Frontend
	ufsClient cache.UFSClient
}

// Close closes all open server resources.
func (s *tlwServer) Close() {
}

func (s *tlwServer) CacheForDut(ctx context.Context, rawURL, dutName string) (string, error) {
	if rawURL == "" {
		return "", errors.Reason("CacheForDut: unsupported url %s", rawURL).Err()
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", errors.Reason("CacheForDut: unsupported url %s", rawURL).Err()
	}
	if dutName == "" {
		return "", errors.Reason("CacheForDut: DutName is empty").Err()
	}
	dutName = extractDutName(dutName)
	if dutName == "" {
		return "", errors.Reason("CacheForDut: unsupported DutName %s", dutName).Err()
	}
	return s.cache(ctx, parsedURL, dutName)
}

var (
	// Peripheral suffixes used with dut-name to create peripheral names.
	peripheralNameSuffixes = []string{
		"-router",
		"-pcap",
		"-btpeer1",
		"-btpeer2",
		"-btpeer3",
		"-btpeer4",
		"-btpeer5",
		"-btpeer6",
	}
)

// extractDutName extracts dut-name from peripherals names.
// If the name is peripheral, then it will have one of the suffixes.
func extractDutName(name string) string {
	for _, s := range peripheralNameSuffixes {
		if strings.HasSuffix(name, s) {
			return strings.TrimSuffix(name, s)
		}
	}
	return name
}

// cache implements the logic for the CacheForDut method and runs as a goroutine.
func (s *tlwServer) cache(ctx context.Context, parsedURL *url.URL, dutName string) (string, error) {
	log.Printf("CacheForDut: Started Operation")

	path := fmt.Sprintf("%s%s", parsedURL.Host, parsedURL.Path)
	// TODO (guocb): return a url.URL instead of string.
	cs, err := s.cFrontend.AssignBackend(dutName, path)
	if err != nil {
		log.Printf("CacheForDut: %s", err)
		return "", errors.Annotate(err, "cache").Err()
	}

	u := fmt.Sprintf("%s/download/%s", strings.TrimSuffix(cs, "/"), path)
	log.Printf("CacheForDut: result URL: %s", u)
	return u, nil
}
