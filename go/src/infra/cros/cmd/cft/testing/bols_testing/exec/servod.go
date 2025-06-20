// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package exec implements the bols_testing for testing functionality of BOLS.
package exec

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	"go.chromium.org/chromiumos/config/go/test/api/bols"
)

// verifyFileAPIs verifies file related APIs of BOLS.
func verifyServodAPIs(ctx context.Context, logger *log.Logger, a *args, cl bols.BolsServiceClient) error {
	req := &bols.StartServodRequest{
		StationId: &bols.StationIdentifier{
			ServodPort:    int32(a.servodPort),
			ContainerName: a.servodContainer,
			ServoSerial:   a.servoSerial,
		},
		Board: a.board,
		Model: a.model,
	}
	if _, err := cl.StartServod(ctx, req); err != nil {
		return fmt.Errorf("failed to start servod: %w", err)
	}
	if err := verifyServodEcho(ctx, logger, a, cl); err != nil {
		return err
	}
	if err := verifyServodSetGetLibOpen(ctx, logger, a, cl); err != nil {
		return err
	}
	return nil
}

func verifyServodEcho(ctx context.Context, logger *log.Logger, a *args, cl bols.BolsServiceClient) (err error) {
	logger.Println("verifyServodEcho: Verifying ServodEcho")
	wanted := "Hello world"
	rspn, err := cl.EchoServod(ctx, &bols.EchoServodRequest{
		StationId: &bols.StationIdentifier{
			ServodPort:    int32(a.servodPort),
			ContainerName: a.servodContainer,
		},
		Echo: wanted,
	})
	if err != nil {
		return fmt.Errorf("failed to send echo request to servod: %w", err)
	}
	if rspn.GetResult() != wanted {
		return fmt.Errorf("got unexpect echo value; got: %q want: %q", rspn.GetResult(), wanted)
	}
	logger.Println("verifyServodEcho: verification was successful")
	return nil
}

func verifyServodSetGetLibOpen(ctx context.Context, logger *log.Logger, a *args, cl bols.BolsServiceClient) (err error) {
	logger.Println("verifyServodSetGetLibOpen: Verifying ServodEcho")
	control := "lid_open"
	if err := verifyServodSetGet(ctx, logger, a, cl, control,
		&bols.ServodValue{
			Value: &bols.ServodValue_StringValue{
				StringValue: "no",
			},
		}); err != nil {
		return fmt.Errorf("failed to close lid: %w", err)
	}
	if err := verifyServodSetGet(ctx, logger, a, cl, control,
		&bols.ServodValue{
			Value: &bols.ServodValue_StringValue{
				StringValue: "yes",
			},
		}); err != nil {
		return fmt.Errorf("failed to open lid: %w", err)
	}
	logger.Println("verifyServodSetGetLibOpen: verification was successful")
	return nil
}

func verifyServodSetGet(ctx context.Context, logger *log.Logger, a *args, cl bols.BolsServiceClient,
	control string, value *bols.ServodValue) (err error) {
	if err := setControl(ctx, logger, a, cl, control, value); err != nil {
		return fmt.Errorf("failed to send set control request to BOLS: %w", err)
	}
	result, err := getControl(ctx, logger, a, cl, control)
	if err != nil {
		return fmt.Errorf("failed to send get control request to BOLS: %w", err)
	}
	// Log the values for easier debugging if there's a mismatch
	logger.Printf("verifyServodSetGet: Comparing values for control %q. Expected: %q, Got: %q", control, servodValueToString(value), servodValueToString(result))
	if diff := cmp.Diff(result, value, protocmp.Transform()); diff != "" {
		return fmt.Errorf("got unexpected results (-got +want):\n%s", diff)
	}
	return nil
}

func setControl(ctx context.Context, logger *log.Logger, a *args, cl bols.BolsServiceClient,
	control string, value *bols.ServodValue) error {
	logger.Printf("Sending SetServodRequest Request for control %s, value: %s\n", control, servodValueToString(value))
	req := &bols.SetServodRequest{
		StationId: &bols.StationIdentifier{
			ServodPort:    int32(a.servodPort),
			ContainerName: a.servodContainer,
		},
		Control: control,
		Value:   value,
	}
	_, err := cl.SetServod(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send SetServodRequest: %w", err)
	}
	logger.Printf("Successfully send SetServodRequest Request for control %s\n", control)
	return nil
}

func getControl(ctx context.Context, logger *log.Logger, a *args, cl bols.BolsServiceClient,
	control string) (value *bols.ServodValue, err error) {
	logger.Printf("Sending GetServodRequest Request for control %s\n", control)
	req := &bols.GetServodRequest{
		StationId: &bols.StationIdentifier{
			ServodPort:    int32(a.servodPort),
			ContainerName: a.servodContainer,
		},
		Control: control,
	}
	rspn, err := cl.GetServod(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to send SetServodRequest: %w", err)
	}
	logger.Printf("Successfully send GetServodRequest Request for control %s, received value: %s\n", control, servodValueToString(rspn.GetValue()))
	return rspn.GetValue(), nil
}

// servodValueToString converts a *bols.ServodValue to its string representation.
func servodValueToString(value *bols.ServodValue) string {
	if value == nil {
		return "<nil>"
	}
	switch v := value.Value.(type) {
	case *bols.ServodValue_StringValue:
		return v.StringValue
	case *bols.ServodValue_IntValue:
		return strconv.FormatInt(int64(v.IntValue), 10)
	case *bols.ServodValue_FloatValue: // Deprecated, but handle for completeness
		return strconv.FormatFloat(float64(v.FloatValue), 'f', -1, 32)
	case *bols.ServodValue_DoubleValue:
		return strconv.FormatFloat(v.DoubleValue, 'f', -1, 64)
	default:
		return fmt.Sprintf("<unknown type: %T>", v)
	}
}
