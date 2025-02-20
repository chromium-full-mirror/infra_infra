// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// command liveness-checker checks the specified service endpoints with
// expectations and sent th result to Monarch.
package main

import (
	"context"
	"crypto/md5"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"go.chromium.org/luci/common/tsmon"
	"go.chromium.org/luci/common/tsmon/field"
	"go.chromium.org/luci/common/tsmon/metric"
	"go.chromium.org/luci/common/tsmon/target"
)

func main() {
	if err := innerMain(); err != nil {
		log.Fatalf("Error: %s", err)
	}
	log.Printf("Done")
}

func innerMain() error {
	var (
		serviceName         = flag.String("service-name", "", "The service name.")
		serviceEndpoints    = flag.String("service-endpoints", "", "Comma separated service endpoints ('http://' is required), e.g. http://service1,http://service2:port.")
		serviceURI          = flag.String("uri", "/", "The service URI to check.")
		extraHeaders        = flag.String("extra-headers", "", "Comma separated extra HTTP headers added to the requests, e.g. HEADER1:value,HEADER2:value.")
		expectedStatusCode  = flag.Int("expected-status-code", 200, "The expected response HTTP status code.")
		expectedContentMD5  = flag.String("expected-content-md5", "", "The exptected response content MD5 hash value.")
		sourceAddr          = flag.String("use-source-addr", "", "Use the specified source IP addr instead of the one get automatically")
		tsmonEndpoint       = flag.String("tsmon-endpoint", "", "URL (including file://, https://, // pubsub://project/topic) to post monitoring metrics to. No metrics to report when skip the option.")
		tsmonCredentialPath = flag.String("tsmon-credential", "", "The credential file for tsmon client.")
		tsmonTaskHostname   = flag.String("tsmon-task-hostname", "", "Name of the host on which this checker is running. (default is the hostname)")
		timeout             = flag.Int("timeout-second", 30, "Total timeout (in seconds) for the checking.")
	)
	flag.Parse()
	if *serviceName == "" {
		return fmt.Errorf("service name must be specified.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout)*time.Second)
	defer cancel()

	if err := metricsInit(ctx, *serviceName, *tsmonEndpoint, *tsmonCredentialPath, *tsmonTaskHostname); err != nil {
		return err
	}
	defer metricsShutdown(ctx)

	reqs, err := createRequests(ctx, strings.Split(*serviceEndpoints, ","), *serviceURI, *extraHeaders)
	if err != nil {
		return err
	}
	checkers := []httpRespChecker{httpStatusChecker(*expectedStatusCode)}
	if *expectedContentMD5 != "" {
		checkers = append(checkers, httpContentChecker(*expectedContentMD5))
	}

	hc, err := getHTTPClient(*sourceAddr)
	if err != nil {
		return err
	}
	return checkEndpoints(ctx, hc, reqs, checkers)
}

// metricsInit sets up the tsmon metrics.
func metricsInit(ctx context.Context, serviceName, tsmonEndpoint, tsmonCredentialPath, tsmonTaskHostname string) error {
	log.Printf("Setting up metrics...")
	fl := tsmon.NewFlags()
	fl.Endpoint = tsmonEndpoint
	fl.Credentials = tsmonCredentialPath
	fl.Flush = tsmon.FlushManual
	fl.Target.TargetType = target.TaskType
	fl.Target.TaskServiceName = serviceName
	fl.Target.TaskJobName = serviceName
	fl.Target.TaskHostname = tsmonTaskHostname
	fl.Target.SetDefaultsFromHostname()

	if err := tsmon.InitializeFromFlags(ctx, &fl); err != nil {
		return fmt.Errorf("metrics init: %w", err)
	}
	return nil
}

// metricsShutdown stops the tsmon metrics.
func metricsShutdown(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	log.Printf("Shutting down metrics...")
	tsmon.Shutdown(ctx)
}

// createRequests creates HTTP requests objects for each endpoint.
func createRequests(ctx context.Context, endpoints []string, uri, extraHeaders string) ([]*http.Request, error) {
	var reqs []*http.Request
	for _, e := range endpoints {
		if e == "" {
			return nil, fmt.Errorf("create requests: got an empty endpoint: %v", endpoints)
		}
		base, err := url.Parse(e)
		if err != nil {
			return nil, fmt.Errorf("create requests for (%q, %q): %w", e, uri, err)
		}

		u, err := url.Parse(uri)
		if err != nil {
			return nil, fmt.Errorf("create requests for (%q, %q): %w", e, uri, err)
		}

		r, err := http.NewRequestWithContext(ctx, "GET", base.ResolveReference(u).String(), nil)
		if err != nil {
			return nil, fmt.Errorf("create requests for (%q, %q): %w", e, uri, err)
		}
		reqs = append(reqs, r)
	}
	if err := setExtraHeaders(reqs, extraHeaders); err != nil {
		return nil, fmt.Errorf("create requests: %w", err)
	}
	return reqs, nil
}

// setExtraHeaders sets the specified extra HTTP headers to the requests.
func setExtraHeaders(reqs []*http.Request, headerList string) error {
	if headerList == "" {
		return nil
	}
	for _, h := range strings.Split(headerList, ",") {
		fields := strings.SplitN(h, ":", 2)
		if len(fields) != 2 || fields[0] == "" || fields[1] == "" {
			return fmt.Errorf("set extra headers: header %q not in format of 'key:value'", h)
		}
		for _, r := range reqs {
			r.Header.Set(fields[0], fields[1])
		}
	}
	return nil
}

var serviceLiveness = metric.NewBool("chromeos/fleet/service/liveness",
	"True/False representing service alive/dead",
	nil,
	field.String("endpoint"),
)

// checkEndpoints checks the liveness of a series of endpoints in parallel.
func checkEndpoints(ctx context.Context, c *http.Client, reqs []*http.Request, checkers []httpRespChecker) error {
	log.Printf("checking %d endpoints", len(reqs))
	var wg sync.WaitGroup
	for _, r := range reqs {
		wg.Add(1)
		go func(r *http.Request) {
			defer wg.Done()
			if err := checkEndpoint(c, r, checkers); err != nil {
				log.Printf("checking endpoints: %s", err)
				serviceLiveness.Set(ctx, false, r.Host)
			} else {
				log.Printf("checking %q: OK", r.Host)
				serviceLiveness.Set(ctx, true, r.Host)
			}
		}(r)
	}
	wg.Wait()
	return nil
}

// checkEndpoint checks the liveness of a particular endpoint.
func checkEndpoint(c *http.Client, r *http.Request, checkers []httpRespChecker) error {
	resp, err := c.Do(r)
	if err != nil {
		return fmt.Errorf("check endpoint %q: %w", r.URL, err)
	}
	defer resp.Body.Close()

	for _, chk := range checkers {
		if err := chk(resp); err != nil {
			return fmt.Errorf("check endpoint %q: %w", r.URL, err)
		}
	}
	return nil
}

type httpRespChecker func(*http.Response) error

// httpStatusChecker creates a HTTP response code checker.
func httpStatusChecker(expectedStatusCode int) httpRespChecker {
	return func(resp *http.Response) error {
		if resp.StatusCode != expectedStatusCode {
			return fmt.Errorf("HTTP status check: got %d, expected %d", resp.StatusCode, expectedStatusCode)
		}
		return nil
	}
}

// httpContentChecker creates HTTP content checker.
// FIXME(guocb): We can only have one content checker for now, as it drains
// the response body and so other content checkers can read nothing
// more from the body.
func httpContentChecker(expectedMD5 string) httpRespChecker {
	return func(resp *http.Response) error {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("HTTP content MD5 check: read body: %w", err)
		}
		if m := fmt.Sprintf("%x", md5.Sum(b)); m != expectedMD5 {
			return fmt.Errorf("HTTP content MD5 check: got %q, exptected %q", m, expectedMD5)
		}
		return nil
	}
}

// getHTTPClient gets a customized http client.
func getHTTPClient(sourceAddr string) (*http.Client, error) {
	if sourceAddr == "" {
		return http.DefaultClient, nil
	}
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:0", sourceAddr))
	if err != nil {
		return nil, fmt.Errorf("get http client bind to %q: %w", sourceAddr, err)
	}
	dialer := &net.Dialer{LocalAddr: addr}
	transport := &http.Transport{DialContext: dialer.DialContext}
	return &http.Client{Transport: transport}, nil
}
