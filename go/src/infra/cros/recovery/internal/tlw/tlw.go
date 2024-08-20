// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tlw

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/internal/tlw/cache"
)

type Server interface {
	CacheForDut(ctx context.Context, rawURL, dutName string) (string, error)
}

// New creates new TLW Server.
func New(ufs cache.UFSClient) (Server, error) {
	return &tlwServer{
		cFrontend: nil,
		ufsClient: ufs,
	}, nil
}

type tlwServer struct {
	// delay initialization as can be not used over the time.
	cFrontend *cache.Frontend
	ufsClient cache.UFSClient
}

// Close closes all open server resources.
func (s *tlwServer) Close() {
}

func (s *tlwServer) initialize(ctx context.Context) error {
	if s.cFrontend != nil {
		return nil
	}
	ce, err := cache.New(ctx, s.ufsClient)
	if err != nil {
		return errors.Annotate(err, "initialize: cFrontend for tlw").Err()
	}
	s.cFrontend = cache.NewFrontend(ce)
	log.Debugf(ctx, "TLW: cFrontend initialized!")
	return nil
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
	log.Debugf(ctx, "CacheForDut: Started Operation")
	if err := s.initialize(ctx); err != nil {
		return "", errors.Annotate(err, "cache").Err()
	}
	path := fmt.Sprintf("%s%s", parsedURL.Host, parsedURL.Path)
	// TODO (guocb): return a url.URL instead of string.
	cs, err := s.cFrontend.AssignBackend(dutName, path)
	if err != nil {
		log.Debugf(ctx, "CacheForDut: %s", err)
		return "", errors.Annotate(err, "cache").Err()
	}
	u, err := url.JoinPath(cs, "download", path)
	if err != nil {
		return "", errors.Annotate(err, "cache").Err()
	}
	log.Debugf(ctx, "CacheForDut: result URL: %s", u)
	return u, nil
}
