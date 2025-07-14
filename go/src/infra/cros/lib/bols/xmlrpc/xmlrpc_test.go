// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package xmlrpc

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api/bols"

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
		t.Fatalf("response returned wrong value: got: %v wanted: 3.14", rspn.GetValue().GetDoubleValue())
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
		t.Fatalf("response returned wrong value: got: %v wanted: 314", rspn.GetValue().GetDoubleValue())
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
		t.Fatalf("response returned wrong value: got: %v wanted: %s", rspn.GetValue().GetDoubleValue(), stringValue)
	}
}

func TestSetServodDouble(t *testing.T) {
	var errCode error
	wanted := 3.14
	control := "control"
	mh := setRequestHandler(control, wanted, &errCode)
	ms, err := xmlrpc.NewMockServer(mh)
	if err != nil {
		t.Fatal("failed to create a mock server:", err)
	}
	host, port, err := ms.HostPort()
	if err != nil {
		t.Fatalf("failed to get host/port information: %v", err)
	}
	value := &bols.ServodValue{Value: &bols.ServodValue_DoubleValue{DoubleValue: wanted}}
	if _, err := SetServod(context.Background(), host, int32(port), control, value); err != nil {
		t.Fatalf("failed to call SetServod: %v", err)
	}
	if errCode != nil {
		t.Fatalf("incorrect request in SetServod: %v", errCode)
	}
}

func TestSetServodInt(t *testing.T) {
	var errCode error
	var wanted int32 = 3
	control := "control"
	mh := setRequestHandler(control, wanted, &errCode)
	ms, err := xmlrpc.NewMockServer(mh)
	if err != nil {
		t.Fatal("failed to create a mock server:", err)
	}
	host, port, err := ms.HostPort()
	if err != nil {
		t.Fatalf("failed to get host/port information: %v", err)
	}
	value := &bols.ServodValue{Value: &bols.ServodValue_IntValue{IntValue: wanted}}
	if _, err := SetServod(context.Background(), host, int32(port), control, value); err != nil {
		t.Fatalf("failed to call SetServod: %v", err)
	}
	if errCode != nil {
		t.Fatalf("incorrect request in SetServod: %v", errCode)
	}
}

func TestSetServodString(t *testing.T) {
	var errCode error
	wanted := "hello world"
	control := "control"
	mh := setRequestHandler(control, wanted, &errCode)
	ms, err := xmlrpc.NewMockServer(mh)
	if err != nil {
		t.Fatal("failed to create a mock server:", err)
	}
	host, port, err := ms.HostPort()
	if err != nil {
		t.Fatalf("failed to get host/port information: %v", err)
	}
	value := &bols.ServodValue{Value: &bols.ServodValue_StringValue{StringValue: wanted}}
	if _, err := SetServod(context.Background(), host, int32(port), control, value); err != nil {
		t.Fatalf("failed to call SetServod: %v", err)
	}
	if errCode != nil {
		t.Fatalf("incorrect request in SetServod: %v", errCode)
	}
}

func setRequestHandler(control string, expectedValue any, errCode *error) xmlrpc.MockHandler {
	mh := func(call *xmlrpc.MethodCall) *xmlrpc.MethodResponse {
		if call.MethodName != "set" {
			*errCode = fmt.Errorf("request has wrong method name got: %q wanted: %q", call.MethodName, "set")
			return &xmlrpc.MethodResponse{}
		}
		if call.Params == nil || len(*call.Params) < 2 {
			*errCode = errors.New("call request has no value")
			return &xmlrpc.MethodResponse{}
		}
		params := *call.Params
		if params[0].Value.Str == nil {
			*errCode = errors.New("no control value in request")
			return &xmlrpc.MethodResponse{}
		}
		if *params[0].Value.Str != control {
			*errCode = fmt.Errorf("incorrect control value got: %q wanted: %q", *params[0].Value.Str, control)
		}
		if err := sameAsExpectedValue(params[1].Value, expectedValue); err != nil {
			*errCode = fmt.Errorf("failed to compare value: %w", err)
			return &xmlrpc.MethodResponse{}
		}
		return &xmlrpc.MethodResponse{}
	}
	return mh
}

func sameAsExpectedValue(xmlValue xmlrpc.Value, expectedValue any) error {
	switch o := expectedValue.(type) {
	case string:
		if xmlValue.Str == nil {
			return fmt.Errorf("failed to get expected string value %q", o)
		}
		if *xmlValue.Str != o {
			return fmt.Errorf("unexpected value: got: %q wanted: %q",
				*xmlValue.Str, o)
		}
		return nil
	case int32:
		if xmlValue.Int == nil {
			return fmt.Errorf("failed to get expected string value %d", o)
		}
		got, err := strconv.ParseInt(*xmlValue.Int, 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse int value %q", *xmlValue.Int)
		}
		if int32(got) != o {
			return fmt.Errorf("unexpected value: got: %q wanted: %q",
				*xmlValue.Str, o)
		}
		return nil
	case float64:
		if xmlValue.Double == nil {
			return fmt.Errorf("failed to get expected string value %f", o)
		}
		got, err := strconv.ParseFloat(*xmlValue.Double, 64)
		if err != nil {
			return fmt.Errorf("failed to parse double value %q", *xmlValue.Double)
		}
		const threshold = 0.0001
		if math.Abs(got-o) > threshold {
			return fmt.Errorf("response returned wrong value: got: %f wanted: %f", got, o)
		}
		return nil
	default:
		return fmt.Errorf("unknown type %T", expectedValue)
	}
}

func TestGetServodVersion(t *testing.T) {
	version := "foo"
	var mh xmlrpc.MockHandler
	params := []xmlrpc.Param{
		{Value: xmlrpc.Value{Str: &version}},
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
	rspn, err := GetServodVersion(context.Background(),
		&bols.GetServodVersionRequest{
			StationId: &bols.StationIdentifier{
				ServodPort:    int32(port),
				ContainerName: host,
			},
		})
	if err != nil {
		t.Fatalf("failed to call GetServodVersion: %v", err)
	}
	if rspn.GetVersion() != version {
		t.Errorf("unexpected value: got: %q wanted: %q", rspn.GetVersion(), version)
	}
}

func TestEchoServod(t *testing.T) {
	msg := "foo"
	var mh xmlrpc.MockHandler
	var errCode error
	mh = func(call *xmlrpc.MethodCall) *xmlrpc.MethodResponse {
		params := *call.Params
		if params[0].Value.Str == nil {
			errCode = errors.New("no control value in request")
			return &xmlrpc.MethodResponse{}
		}
		if *params[0].Value.Str != msg {
			errCode = fmt.Errorf("incorrect echo value got: %q wanted: %q", *params[0].Value.Str, msg)
		}
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
	rspn, err := EchoServod(context.Background(), host, int32(port), msg)
	if err != nil {
		t.Fatalf("failed to call EchoServod: %v", err)
	}
	if errCode != nil {
		t.Fatal("encountered an error in serving EchoServod", errCode)
	}
	if rspn.GetResult() != msg {
		t.Errorf("unexpected value: got: %q wanted: %q", rspn.GetResult(), msg)
	}
}
