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
	value, err := translateValue(resp.Params)
	if err != nil {
		return nil, fmt.Errorf("failed to parse value for control %q from servod: %w", control, err)
	}
	return &bols.GetServodResponse{Control: control, Value: value}, nil
}

func translateValue(params *[]xmlrpc.Param) (*bols.ServodValue, error) {
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
