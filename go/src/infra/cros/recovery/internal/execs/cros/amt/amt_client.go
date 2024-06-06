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
	"time"

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
}

// NewAMTClient returns a new AMTClient instance.
func NewAMTClient(hostname string, username string, password string) AMTClient {
	protocol := "http"
	port := 16992
	uri := fmt.Sprintf("%s://%s:%d/wsman", protocol, hostname, port)
	return AMTClient{uri, username, password}
}

func (c AMTClient) post(ctx context.Context, request string) (string, error) {
	log.Debugf(ctx, "Posting HTTP request: %s", request)
	t := dac.NewTransport(c.username, c.password)
	r, err := http.NewRequest("POST", c.uri, strings.NewReader(request))
	if err != nil {
		return "", errors.Reason("failed to create the request").Err()
	}
	r.Header.Add("Content-Type", "application/soap+xml;charset=UTF-8")
	resp, err := t.RoundTrip(r)
	log.Debugf(ctx, "Received HTTP status code: %d", resp.StatusCode)
	if err != nil {
		return "", errors.Reason("failed to post the data").Err()
	}
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

// AMTPresent returns true if the client URI is accessible.
func (c AMTClient) AMTPresent(ctx context.Context) (bool, error) {
	client := http.Client{
		Timeout: 500 * time.Millisecond,
	}
	resp, err := client.Get(c.uri)
	if err != nil {
		return false, err
	}
	// Is the URI is not found then AMT is not enabled.
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	} else if resp.StatusCode == http.StatusUnauthorized {
		return true, nil
	} else {
		return false, errors.Reason("responded with status %d", resp.StatusCode).Err()
	}
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

// PowerOn powers on the DUT using Intel AMT (vPro).
func (c AMTClient) PowerOn(ctx context.Context) error {
	resp, err := c.post(ctx, createUpdateAMTPowerStateRequest(c.uri, powerStateMap["on"]))
	if err != nil {
		return err
	}
	rvalue, err := findReturnValue(resp)
	if err != nil {
		return err
	}
	if rvalue != 0 {
		return errors.Reason("power on failed with: %d", rvalue).Err()
	}
	return nil
}

// PowerOff powers off the DUT using Intel AMT (vPro).
func (c AMTClient) PowerOff(ctx context.Context) error {
	resp, err := c.post(ctx, createUpdateAMTPowerStateRequest(c.uri, powerStateMap["off"]))
	if err != nil {
		return err
	}
	rvalue, err := findReturnValue(resp)
	if err != nil {
		return err
	}
	if rvalue != 0 {
		return errors.Reason("power off failed with: %d", rvalue).Err()
	}
	return nil
}
