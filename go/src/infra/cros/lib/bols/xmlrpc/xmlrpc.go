// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package xmlrpc contains xmlrpc client utilities for BOLS.
package xmlrpc

import (
	"context"
	"encoding/xml"
	"fmt"
	"strconv"

	"go.chromium.org/chromiumos/config/go/test/api/bols"

	"go.chromium.org/infra/cros/servo/xmlrpc"
)

// GetServod gets a servod control value.
func GetServod(ctx context.Context, host string, port int32, control string) (*bols.GetServodResponse, error) {
	cl := xmlrpc.New(host, int(port))
	resp, err := cl.Execute(ctx, xmlrpc.NewCall("get", string(control)))
	if err != nil {
		return nil, fmt.Errorf("failed to get control %q from servod: %w", control, err)
	}
	value, err := xmlValueToServodValue(resp.Params)
	if err != nil {
		return nil, fmt.Errorf("failed to parse value for control %q from servod: %w", control, err)
	}
	return &bols.GetServodResponse{Control: control, Value: value}, nil
}

// SetServod gets a servod control value.
func SetServod(ctx context.Context, req *bols.SetServodRequest) (*bols.SetServodResponse, error) {
	host := req.GetStationId().GetContainerName()
	if host == "" {
		host = "localhost"
	}
	port := req.GetStationId().GetServodPort()
	control := req.GetControl()
	servodValue := req.GetValue()
	cl := xmlrpc.New(host, int(port))
	call, err := servodValueToXMLRequest("set", control, servodValue)
	if err != nil {
		return nil, fmt.Errorf("failed to translate value from servod request: %w", err)
	}

	if _, err := cl.Execute(ctx, call); err != nil {
		return nil, fmt.Errorf("failed to get control %q from servod: %w", control, err)
	}
	return &bols.SetServodResponse{}, nil
}

func xmlValueToServodValue(params *[]xmlrpc.Param) (*bols.ServodValue, error) {
	if params == nil {
		return nil, nil
	}
	values := *params
	if len(values) == 0 {
		return nil, nil
	}
	// We only care about the first value for now.
	src := values[0]
	switch {
	case src.Value.Boolean != nil:
		return &bols.ServodValue{
			Value: &bols.ServodValue_StringValue{StringValue: *src.Value.Boolean},
		}, nil
	case src.Value.Double != nil:
		val, err := strconv.ParseFloat(*src.Value.Double, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to convert %q to double: %w", *src.Value.Double, err)
		}
		return &bols.ServodValue{
			Value: &bols.ServodValue_DoubleValue{DoubleValue: val},
		}, nil
	case src.Value.Int != nil:
		val, err := strconv.ParseInt(*src.Value.Int, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("failed to convert %q to int: %w", *src.Value.Int, err)
		}
		return &bols.ServodValue{
			Value: &bols.ServodValue_IntValue{IntValue: int32(val)},
		}, nil
	case src.Value.Str != nil:
		return &bols.ServodValue{
			Value: &bols.ServodValue_StringValue{StringValue: *src.Value.Str},
		}, nil
	case src.Value.Base64 != nil:
		return &bols.ServodValue{
			Value: &bols.ServodValue_StringValue{StringValue: *src.Value.Base64},
		}, nil
	case src.Value.Base64 != nil:
		return &bols.ServodValue{
			Value: &bols.ServodValue_StringValue{StringValue: *src.Value.Base64},
		}, nil
	case src.Value.Array != nil:
		data, err := xml.Marshal(src.Value.Array)
		if err != nil {
			return nil, fmt.Errorf("failed to convert array to xml string: %w", err)
		}
		return &bols.ServodValue{
			Value: &bols.ServodValue_StringValue{StringValue: string(data)},
		}, nil
	case src.Value.Struct != nil:
		data, err := xml.Marshal(src.Value.Struct)
		if err != nil {
			return nil, fmt.Errorf("failed to convert struct to xml string: %w", err)
		}
		return &bols.ServodValue{
			Value: &bols.ServodValue_StringValue{StringValue: string(data)},
		}, nil
	}
	return nil, fmt.Errorf("unsupport format")
}

func servodValueToXMLRequest(method, control string, servodValue *bols.ServodValue) (xmlrpc.Call, error) {
	switch servodValue.GetValue().(type) {
	case *bols.ServodValue_StringValue:
		v := servodValue.GetStringValue()
		return xmlrpc.NewCall(method, control, v), nil
	case *bols.ServodValue_DoubleValue:
		v := servodValue.GetDoubleValue()
		return xmlrpc.NewCall(method, control, v), nil
	case *bols.ServodValue_IntValue:
		v := servodValue.GetIntValue()
		return xmlrpc.NewCall(method, control, int(v)), nil
	default:
		return xmlrpc.Call{}, fmt.Errorf("unsupport type %T", servodValue.GetValue())
	}
}
