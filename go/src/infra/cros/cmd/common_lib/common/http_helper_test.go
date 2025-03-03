// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"fmt"
	"net/http"
	"testing"
)

// clientWithHTTPError returns numErrorsToReturn HTTP error codes before
// returning a 200 HTTP response.
type clientWithHTTPError struct {
	numErrorsToReturn int
}

func (c *clientWithHTTPError) Do(_ *http.Request) (resp *http.Response, err error) {
	if c.numErrorsToReturn > 0 {
		c.numErrorsToReturn -= 1
		return &http.Response{StatusCode: http.StatusInternalServerError}, nil
	}
	return &http.Response{StatusCode: http.StatusOK}, nil
}

var testSendRequestWithRetriesData = []struct {
	client              *clientWithHTTPError
	wantRemainingErrors int
	wantStatusCode      int
}{
	{
		&clientWithHTTPError{7},
		2,
		http.StatusInternalServerError,
	},
	{
		&clientWithHTTPError{5},
		0,
		http.StatusInternalServerError,
	},
	{
		&clientWithHTTPError{4},
		0,
		http.StatusOK,
	},
	{
		&clientWithHTTPError{0},
		0,
		http.StatusOK,
	},
}

func TestSendRequestWithRetries(t *testing.T) {
	t.Parallel()
	for _, tt := range testSendRequestWithRetriesData {
		t.Run(fmt.Sprintf("%v", tt.client), func(t *testing.T) {
			t.Parallel()
			gotResp, err := sendHTTPRequestWithRetries(tt.client, nil, false)
			if err != nil {
				t.Errorf("unexpected error %v", err)
			}
			if gotResp.StatusCode != tt.wantStatusCode {
				t.Errorf("gotResp.StatusCode: got %v, wanted %v", gotResp.StatusCode, tt.wantStatusCode)
			}
			if tt.client.numErrorsToReturn != tt.wantRemainingErrors {
				t.Errorf("remaining errors: got %v, wanted %v", tt.client.numErrorsToReturn, tt.wantRemainingErrors)
			}
		})
	}
}
