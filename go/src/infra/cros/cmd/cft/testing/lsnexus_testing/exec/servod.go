// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package exec implements the bols_testing for testing functionality of lsnexus.
package exec

import (
	"context"
	"fmt"
	"log"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	"go.chromium.org/chromiumos/config/go/test/api/bols"
	"go.chromium.org/chromiumos/config/go/test/api/lsnexus"
)

// verifyFileAPIs verifies file related APIs of lsnexus.
func verifyServodAPIs(ctx context.Context, logger *log.Logger, a *args, cl lsnexus.LSNexusServiceClient) error {
	req := &lsnexus.StartServodRequest{}
	if _, err := cl.StartServod(ctx, req); err != nil {
		return fmt.Errorf("failed to start servod: %w", err)
	}
	if err := verifyServodEcho(ctx, logger, cl); err != nil {
		return err
	}
	if err := verifyServodSetGetlidOpen(ctx, logger, cl); err != nil {
		return err
	}
	return nil
}

func verifyServodEcho(ctx context.Context, logger *log.Logger, cl lsnexus.LSNexusServiceClient) (err error) {
	logger.Println("verifyServodEcho: Verifying ServodEcho")
	wanted := "Hello world"
	rspn, err := cl.Echo(ctx, &lsnexus.EchoRequest{
		Msg: wanted,
	})
	if err != nil {
		return fmt.Errorf("failed to send echo request to servod: %w", err)
	}
	if rspn.GetResult() != wanted {
		return fmt.Errorf("failed to get correct echo response; got: %q, wanted: %q", rspn.GetResult(), wanted)
	}
	logger.Println("verifyServodEcho: verification was successful")
	return nil
}

func verifyServodSetGetlidOpen(ctx context.Context, logger *log.Logger, cl lsnexus.LSNexusServiceClient) (err error) {
	logger.Println("verifyServodSetGetlidOpen: Verifying set and get lid request to servod")
	control := "lid_open"
	if err := verifyServodSetGet(ctx, logger, cl, control,
		&bols.ServodValue{
			Value: &bols.ServodValue_StringValue{
				StringValue: "no",
			},
		}); err != nil {
		return fmt.Errorf("failed to close lid: %w", err)
	}
	if err := verifyServodSetGet(ctx, logger, cl, control,
		&bols.ServodValue{
			Value: &bols.ServodValue_StringValue{
				StringValue: "yes",
			},
		}); err != nil {
		return fmt.Errorf("failed to open lid: %w", err)
	}
	logger.Println("verifyServodSetGetlidOpen: verification was successful")
	return nil
}

func verifyServodSetGet(ctx context.Context, logger *log.Logger, cl lsnexus.LSNexusServiceClient,
	control string, value *bols.ServodValue) (err error) {
	if err := setControl(ctx, logger, cl, control, value); err != nil {
		return fmt.Errorf("failed to send set control request to BOLS: %w", err)
	}
	result, err := getControl(ctx, logger, cl, control)
	if err != nil {
		return fmt.Errorf("failed to send get control request to BOLS: %w", err)
	}
	if diff := cmp.Diff(result, value, protocmp.Transform()); diff != "" {
		return fmt.Errorf("Got unexpected results (-got +want):\n%s", diff)
	}
	return nil
}

func setControl(ctx context.Context, logger *log.Logger, cl lsnexus.LSNexusServiceClient,
	control string, value *bols.ServodValue) error {
	logger.Printf("Sending SetServodRequest Request for control %s\n", control)
	req := &lsnexus.CallServodRequest{
		Method:  lsnexus.CallServodRequest_SET,
		Control: control,
		Args:    []*bols.ServodValue{value},
	}
	_, err := cl.CallServod(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send SetServodRequest: %w", err)
	}
	logger.Printf("Successfully send SetServodRequest Request for control %s\n", control)
	return nil
}

func getControl(ctx context.Context, logger *log.Logger, cl lsnexus.LSNexusServiceClient,
	control string) (value *bols.ServodValue, err error) {
	logger.Printf("Sending GetServodRequest Request for control %s\n", control)
	req := &lsnexus.CallServodRequest{
		Method:  lsnexus.CallServodRequest_GET,
		Control: control,
	}
	rspn, err := cl.CallServod(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to send SetServodRequest: %w", err)
	}
	logger.Printf("Successfully send GetServodRequest Request for control %s\n", control)
	return rspn.GetSuccess().GetResult(), nil
}
