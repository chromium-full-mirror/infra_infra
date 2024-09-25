// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package amt

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/beevik/etree"
	dac "github.com/xinsnake/go-http-digest-auth-client"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/log"
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
	uri, username, password string
	useTLS                  bool
}

// NewAMTClient returns a new AMTClient instance.
func NewAMTClient(ctx context.Context, hostname string, username string, password string, useTLS bool) *AMTClient {
	protocol, port := "http", 16992
	if useTLS {
		protocol, port = "https", 16993
	}
	uri := fmt.Sprintf("%s://%s:%d/wsman", protocol, hostname, port)
	log.Infof(ctx, "Using AMT manager URI: %s", uri)
	return &AMTClient{uri, username, password, useTLS}
}

func (c AMTClient) post(ctx context.Context, request string) (string, error) {
	log.Debugf(ctx, "Posting HTTP request: %s", request)
	t := dac.NewTransport(c.username, c.password)
	r, err := http.NewRequest("POST", c.uri, strings.NewReader(request))
	if err != nil {
		return "", errors.Annotate(err, "failed to create the request").Err()
	}
	r.Header.Add("Content-Type", "application/soap+xml;charset=UTF-8")
	resp, err := t.RoundTrip(r)
	if err != nil {
		return "", errors.Annotate(err, "failed to post the data").Err()
	}
	log.Debugf(ctx, "Received HTTP status code: %d", resp.StatusCode)
	// Work around the following linter error:
	// Error return value of `resp.Body.Close` is not checked (errcheck)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", errors.Reason("responded with status %d", resp.StatusCode).Err()
	}
	body, _ := io.ReadAll(resp.Body)
	log.Debugf(ctx, "Received HTTP response: %s", body)
	return string(body), nil
}

// GetPowerState returns the power state as an int.
func (c AMTClient) GetPowerState(ctx context.Context) (int, error) {
	resp, err := c.post(ctx, createReadAMTPowerStateRequest(c.uri))
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
		resp, err := c.post(ctx, createUpdateAMTPowerStateRequest(c.uri, newState))
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
