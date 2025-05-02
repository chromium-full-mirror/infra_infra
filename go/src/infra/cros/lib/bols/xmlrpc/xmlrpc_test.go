// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package xmlrpc

import (
	"context"
	"math"
	"testing"

	"go.chromium.org/infra/cros/servo/xmlrpc"
)

func TestGetServodDouble(t *testing.T) {
	doubleValue := "3.14"
	var mh xmlrpc.MockHandler
	params := []xmlrpc.Param{
		{Value: xmlrpc.Value{Double: &doubleValue}},
	}
	mh = func(*xmlrpc.MethodCall) *xmlrpc.MethodResponse {
		return &xmlrpc.MethodResponse{
			Params: &params,
		}
	}
	ms, err := xmlrpc.NewMockServer(mh)
	if err != nil {
		t.Fatal("failed to create a mock server:", err)
	}
	host, port, err := ms.HostPort()
	if err != nil {
		t.Fatalf("failed to get host/port information: %v", err)
	}
	rspn, err := GetServod(context.Background(), host, int32(port), "control")
	if err != nil {
		t.Fatalf("failed to call GetServod: %v", err)
	}
	const threshold = 0.0001
	if math.Abs(rspn.GetValue().GetDoubleValue()-3.14) > threshold {
		t.Fatalf("reponse returned wrong value: got: %v wanted: 3.14", rspn.GetValue().GetDoubleValue())
	}
}

func TestGetServodInt(t *testing.T) {
	intValue := "314"
	var mh xmlrpc.MockHandler
	params := []xmlrpc.Param{
		{Value: xmlrpc.Value{Int: &intValue}},
	}
	mh = func(*xmlrpc.MethodCall) *xmlrpc.MethodResponse {
		return &xmlrpc.MethodResponse{
			Params: &params,
		}
	}
	ms, err := xmlrpc.NewMockServer(mh)
	if err != nil {
		t.Fatal("failed to create a mock server:", err)
	}
	host, port, err := ms.HostPort()
	if err != nil {
		t.Fatalf("failed to get host/port information: %v", err)
	}
	rspn, err := GetServod(context.Background(), host, int32(port), "control")
	if err != nil {
		t.Fatalf("failed to call GetServod: %v", err)
	}
	if rspn.GetValue().GetIntValue() != 314 {
		t.Fatalf("reponse returned wrong value: got: %v wanted: 314", rspn.GetValue().GetDoubleValue())
	}
}

func TestGetServodString(t *testing.T) {
	stringValue := "hello world"
	var mh xmlrpc.MockHandler
	params := []xmlrpc.Param{
		{Value: xmlrpc.Value{Str: &stringValue}},
	}
	mh = func(*xmlrpc.MethodCall) *xmlrpc.MethodResponse {
		return &xmlrpc.MethodResponse{
			Params: &params,
		}
	}
	ms, err := xmlrpc.NewMockServer(mh)
	if err != nil {
		t.Fatal("failed to create a mock server:", err)
	}
	host, port, err := ms.HostPort()
	if err != nil {
		t.Fatalf("failed to get host/port information: %v", err)
	}
	rspn, err := GetServod(context.Background(), host, int32(port), "control")
	if err != nil {
		t.Fatalf("failed to call GetServod: %v", err)
	}
	if rspn.GetValue().GetStringValue() != stringValue {
		t.Fatalf("reponse returned wrong value: got: %v wanted: %s", rspn.GetValue().GetDoubleValue(), stringValue)
	}
}
