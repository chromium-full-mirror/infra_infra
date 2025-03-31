// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package amt

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/beevik/etree"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/log"
)

// Map of human-readable states to AMT power states.
var powerStateMap = map[string]int{
	"on":  2,
	"off": 8,
}

// TODO(josephsussman): Check the namespace URI.
func findIntInResponse(response string, tagname string) (int, error) {
	doc := etree.NewDocument()
	err := doc.ReadFromString(response)
	if err != nil {
		return 0, errors.Reason("failed to parse response").Err()
	}
	elem := doc.FindElement(fmt.Sprintf("//%s", tagname))
	value, err := strconv.Atoi(elem.Text())
	if err != nil {
		return 0, errors.Reason("failed to convert to int").Err()
	}
	return value, nil
}

func findReturnValue(response string) (int, error) {
	return findIntInResponse(response, "ReturnValue")
}

func findPowerState(response string) (int, error) {
	return findIntInResponse(response, "PowerState")
}

// AMTClient holds WS-Management connection data.
type AMTClient struct {
	hostname, username, password string
	useTLS                       bool
	t                            *http.Transport
	// The "nonce count" tracks the number of requests sent in
	// response to a given nonce value. Required when using "auth"
	// or "auth-int" modes.
	nc int
}

// NewAMTClient returns a new AMTClient instance.
func NewAMTClient(ctx context.Context, hostname string, username string, password string, useTLS bool) *AMTClient {
	t := http.Transport{}
	if useTLS {
		tlsConfig := tls.Config{InsecureSkipVerify: true}
		t = http.Transport{
			TLSClientConfig: &tlsConfig,
		}
	}
	return &AMTClient{hostname, username, password, useTLS, &t, 0}
}

// uri returns the WS-Management endpoint URI.
func (c AMTClient) uri() string {
	protocol, port := "http", 16992
	if c.useTLS {
		protocol, port = "https", 16993
	}
	return fmt.Sprintf("%s://%s:%d/wsman", protocol, c.hostname, port)
}

// post does an HTTP POST with the given request envelope and returns the
// response envelope. An error is always returned if there's a soap:Fault
// element in the response body or the HTTP status isn't 200.
func (c AMTClient) post(ctx context.Context, request string) (string, error) {
	log.Debugf(ctx, "Posting HTTP request: %s", request)
	response, err := c.roundTrip("", request)
	if err != nil {
		return "", err
	}
	// Respond to any HTTP digest challenge.
	if response.status == http.StatusUnauthorized {
		dr, err := parseDigestResponse(response.authHeader)
		if err != nil {
			return "", err
		}
		uri, err := parseURI(c.uri())
		if err != nil {
			return "", err
		}
		// Increment the nonce counter.
		c.nc++
		da, err := dr.newAuthorization(http.MethodPost, uri, c.username, c.password, request, c.nc)
		if err != nil {
			return "", err
		}
		response, err = c.roundTrip(da.String(), request)
		if err != nil {
			return "", err
		}
	}
	log.Debugf(ctx, "Received HTTP status code: %d", response.status)
	if response.status != http.StatusOK {
		// Ensure that we return an error if the status isn't "200 OK".
		return "", errors.Reason("responded with HTTP status %q", http.StatusText(response.status)).Err()
	}
	log.Debugf(ctx, "Received HTTP response: %s", response.body)
	return response.body, nil
}

func parseURI(urlRaw string) (string, error) {
	u, err := url.ParseRequestURI(urlRaw)
	if err != nil {
		return "", err
	}
	return u.RequestURI(), nil
}

// httpResponse holds response data returned by roundTrip.
type httpResponse struct {
	status     int
	authHeader string
	body       string
}

// roundTrip posts the given envelope XML and returns an httpResponse.
func (c AMTClient) roundTrip(authHdr, body string) (httpResponse, error) {
	req, err := http.NewRequest(http.MethodPost, c.uri(), strings.NewReader(body))
	if err != nil {
		return httpResponse{}, err
	}
	req.Header.Add("content-type", "application/soap+xml; charset=utf-8")
	if authHdr != "" {
		req.Header.Add("Authorization", authHdr)
	}

	resp, err := c.t.RoundTrip(req)
	if err != nil {
		return httpResponse{}, err
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return httpResponse{}, err
	}

	return httpResponse{resp.StatusCode, resp.Header.Get("WWW-Authenticate"), string(respBody)}, nil
}

// GetPowerState returns the power state as an int.
func (c AMTClient) GetPowerState(ctx context.Context) (int, error) {
	resp, err := c.post(ctx, createReadAMTPowerStateRequest(c.uri()))
	if err != nil {
		return 0, err
	}
	state, err := findPowerState(resp)
	if err != nil {
		return 0, err
	}
	log.Debugf(ctx, "Got power state: %d", state)
	return state, nil
}

// SetPowerState attempts to set the specified power state: possible values are "on" or "off".
func (c AMTClient) SetPowerState(ctx context.Context, state string) error {
	if newState, exists := powerStateMap[state]; exists {
		currentState, err := c.GetPowerState(ctx)
		if err != nil {
			return err
		}
		if newState == currentState {
			log.Debugf(ctx, "AMT power state is already: %s", state)
			return nil
		}
		resp, err := c.post(ctx, createUpdateAMTPowerStateRequest(c.uri(), newState))
		if err != nil {
			return err
		}
		rvalue, err := findReturnValue(resp)
		if err != nil {
			return err
		}
		if rvalue != 0 {
			return errors.Reason("set power state failed with: %d", rvalue).Err()
		}
		return nil
	} else {
		return errors.Reason("power state is missing from powerStateMap").Err()
	}
}
