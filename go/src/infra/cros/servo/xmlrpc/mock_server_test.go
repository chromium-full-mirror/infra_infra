// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package xmlrpc

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMockServer(t *testing.T) {
	control := "control"
	doubleValue := "3.14"
	intValue := "314"
	boolValue := "true"
	stringValue := "hello world"
	var mh MockHandler
	params := []Param{
		{Value: Value{Double: &doubleValue}},
		{Value: Value{Int: &intValue}},
		{Value: Value{Boolean: &boolValue}},
		{Value: Value{Str: &stringValue}},
	}
	mh = func(*MethodCall) *MethodResponse {
		return &MethodResponse{
			Params: &params,
		}
	}
	ms, err := NewMockServer(mh)
	if err != nil {
		t.Fatal("failed to create a mock server:", err)
	}
	host, port, err := ms.HostPort()
	if err != nil {
		t.Fatalf("failed to get host/port information: %v", err)
	}
	cl := New(host, port)
	resp, err := cl.Execute(context.Background(), NewCall("get", string(control)))
	if err != nil {
		t.Fatalf("failed to get control %q from servod: %v", control, err)
	}
	if len(*resp.Params) != len(params) {
		t.Fatalf("returned parameters have the wrong size: got: %d wanted: %d", len(*resp.Params), len(params))
	}
	if diff := cmp.Diff(*resp.Params, params); diff != "" {
		t.Errorf("DUTInfo mismatch (-got +want):\n%s", diff)
	}
}
