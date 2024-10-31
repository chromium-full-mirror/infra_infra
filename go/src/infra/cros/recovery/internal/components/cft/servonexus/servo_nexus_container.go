// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package servonexus contains methods to work with an servo-nexus container.
package servonexus

import (
	"context"
	"strings"

	"go.chromium.org/chromiumos/config/go/api/test/xmlrpc"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/recovery/ctr"
	"infra/cros/recovery/internal/components/cft"
	xmlrpc_utils "infra/cros/recovery/internal/localtlw/xmlrpc"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/tlw"
)

const (
	servoHostPort = ":22"
)

// StartServod starts servod daemon.
func StartServod(ctx context.Context, client api.ServodServiceClient, dut *tlw.Dut, useRecoveryMode bool) error {
	if client == nil {
		return errors.Reason("servo-nexus start servod: client is not provided").Err()
	}
	sh := dut.GetChromeos().GetServo()
	if sh == nil {
		return errors.Reason("servo-nexus start servod: servo-host is not provided").Err()
	}
	req := &api.StartServodRequest{
		ServoHostPath:             sh.GetName() + servoHostPort,
		ServodDockerContainerName: sh.GetContainerName(),
		// ServodDockerImagePath string
		ServodPort: sh.GetServodPort(),
		Board:      dut.GetChromeos().GetBoard(),
		Model:      dut.GetChromeos().GetModel(),
		SerialName: sh.GetSerialNumber(),
	}
	if useRecoveryMode {
		req.RecoveryMode = "1"
	}
	if pools, ok := dut.ExtraAttributes[tlw.ExtraAttributePools]; ok {
		for _, p := range pools {
			if strings.Contains(p, "faft-cr50") {
				req.Config = "cr50.xml"
				break
			}
		}
	}
	log.Debugf(ctx, "Calling start servod with request %v.", req)
	op, err := client.StartServod(ctx, req)
	if err != nil {
		return errors.Annotate(err, "servo-nexus start servod: call failure").Err()
	}
	opRes, err := common.ProcessDoneLro(ctx, op)
	if err != nil {
		return errors.Annotate(err, "servo-nexus start servod: lro failure").Err()
	}
	res := &api.StartServodResponse{}
	if err := opRes.UnmarshalTo(res); err != nil {
		return errors.Annotate(err, "servo-nexus start servod: lro response unmarshalling failed").Err()
	}
	log.Debugf(ctx, "Start servod response: %v.", res)
	return nil
}

// StopServod stops servod daemon.
func StopServod(ctx context.Context, client api.ServodServiceClient, dut *tlw.Dut) error {
	if client == nil {
		return errors.Reason("servo-nexus stop servod: client is not provided").Err()
	}
	sh := dut.GetChromeos().GetServo()
	if sh == nil {
		return errors.Reason("servo-nexus stop servod: servo-host is not provided").Err()
	}
	req := &api.StopServodRequest{
		ServoHostPath:             sh.GetName() + servoHostPort,
		ServodDockerContainerName: sh.GetContainerName(),
		ServodPort:                sh.GetServodPort(),
	}
	log.Debugf(ctx, "Calling stop servod with request %v.", req)
	op, err := client.StopServod(ctx, req)
	if err != nil {
		return errors.Annotate(err, "servo-nexus stop servod: call failure").Err()
	}
	opRes, err := common.ProcessDoneLro(ctx, op)
	if err != nil {
		return errors.Annotate(err, "servo-nexus stop servod: lro failure").Err()
	}
	res := &api.StopServodResponse{}
	if err := opRes.UnmarshalTo(res); err != nil {
		return errors.Annotate(err, "servo-nexus stop servod: lro response unmarshalling failed").Err()
	}
	log.Debugf(ctx, "Stop servod response: %v.", res)
	return nil
}

// CallServod calls servod methods with args.
func CallServod(ctx context.Context, client api.ServodServiceClient, dut *tlw.Dut, method string, args ...interface{}) (*xmlrpc.Value, error) {
	if client == nil {
		return nil, errors.Reason("servo-nexus call servod: client is not provided").Err()
	}
	sh := dut.GetChromeos().GetServo()
	if sh == nil {
		return nil, errors.Reason("servo-nexus call servod: servo-host is not provided").Err()
	}
	var callMethod api.CallServodRequest_Method
	if v, ok := api.CallServodRequest_Method_value[strings.ToUpper(method)]; ok {
		callMethod = api.CallServodRequest_Method(v)
	} else {
		return nil, errors.Reason("servo-nexus call servod: unsupported method %q", method).Err()
	}
	req := &api.CallServodRequest{
		Method:                    callMethod,
		Args:                      xmlrpc_utils.PackArgsToXMLRPCValues(args...),
		ServoHostPath:             sh.GetName() + servoHostPort,
		ServodDockerContainerName: sh.GetContainerName(),
		ServodPort:                sh.GetServodPort(),
	}
	log.Debugf(ctx, "Calling servod with request %v.", req)
	res, err := client.CallServod(ctx, req)
	if err != nil {
		return nil, errors.Annotate(err, "servo-nexus call servod: call failure").Err()
	}
	switch result := res.GetResult().(type) {
	case *api.CallServodResponse_Failure_:
		return nil, errors.Reason("servo-nexus call servod: %s", result.Failure.GetErrorMessage()).Err()
	case *api.CallServodResponse_Success_:
		log.Debugf(ctx, "CallServod Call succeeded with response %v", result.Success.GetResult())
		return result.Success.GetResult(), nil
	default:
		return nil, errors.Reason("servo-nexus call servod: unexpected result type").Err()
	}
}

// ServiceClient creates service client to the service running on CFT container.
func ServiceClient(ctx context.Context, ctrInfo ctr.ServiceInfo, dut *tlw.Dut) (api.ServodServiceClient, error) {
	if dut == nil {
		return nil, errors.Reason("servo-nexus service client: dut is not provided").Err()
	}
	if ctrInfo == nil {
		return nil, errors.Reason("servo-nexus service client: ctr client is not provided").Err()
	}
	container, err := ctrInfo.GetContainer(ctx, cft.ServoNexusName(dut))
	if err != nil {
		return nil, errors.Annotate(err, "servo-nexus service client").Err()
	}
	conn, err := container.GetClient(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "servo-nexus service client").Err()
	}
	client := api.NewServodServiceClient(conn)
	if client == nil {
		return nil, errors.Reason("servo-nexus service client: fail to create client").Err()
	}
	return client, nil
}
