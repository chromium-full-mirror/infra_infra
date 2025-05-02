// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package xmlrpc implements the XML-RPC client library.
package xmlrpc

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
)

type MockHandler func(*MethodCall) *MethodResponse

type MockServer struct {
	httpServer *httptest.Server
}

// Addr returns the address of
func (ms *MockServer) Addr() string {
	if ms == nil || ms.httpServer == nil {
		return ""
	}
	return ms.httpServer.URL
}

// HostPort parse the http URL of the mock server, and return the host
// address and the port number.
func (ms *MockServer) HostPort() (string, int, error) {
	if ms == nil || ms.httpServer == nil {
		return "", 0, errors.New("there is no http server")
	}
	fullAddress, found := strings.CutPrefix(ms.httpServer.URL, "http://")
	if !found {
		fullAddress = ms.httpServer.URL
	}
	l := strings.Split(fullAddress, ":")
	if len(l) <= 1 {
		return fullAddress, 0, nil
	}
	port, err := strconv.Atoi(l[len(l)-1])
	if err != nil {
		return "", 0, fmt.Errorf("failed to parse %s to get host and port: %w", ms.httpServer.URL, err)
	}
	return strings.Join(l[:len(l)-1], ""), port, nil
}

// New creates a new XMLRpc mock server for unit testing.
func NewMockServer(handler MockHandler) (*MockServer, error) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rspn := &MethodResponse{}
		defer func() {
			out, _ := xml.Marshal(rspn)
			io.Writer.Write(w, out)
		}()
		if handler == nil {
			msg := "No handler defined"
			rspn.Fault = &Fault{Value: Value{Str: &msg}}
			return
		}
		// Read body and unmarshal XML.
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			msg := fmt.Sprint(err)
			rspn.Fault = &Fault{Value: Value{Str: &msg}}
			return
		}
		req := MethodCall{}
		if err = xml.Unmarshal(bodyBytes, &req); err != nil {
			msg := fmt.Sprint(err)
			rspn.Fault = &Fault{Value: Value{Str: &msg}}
			return
		}
		rspn = handler(&req)
	}))
	return &MockServer{
		httpServer: s,
	}, nil
}
