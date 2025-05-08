// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package execs

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	"go.chromium.org/chromiumos/config/go/api/test/xmlrpc"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	xmlrpc_utils "go.chromium.org/infra/cros/recovery/internal/localtlw/xmlrpc"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// Local implementation of components.Servod.
type iServod struct {
	dut *tlw.Dut
	a   tlw.Access
}

// NewServod returns a struct of type components.Servod that allowes communication with servod service.
func (ei *ExecInfo) NewServod() components.Servod {
	return &iServod{
		dut: ei.GetDut(),
		a:   ei.GetAccess(),
	}
}

// Call calls servod method with params.
func (s *iServod) Call(ctx context.Context, method string, timeout time.Duration, args ...any) (*xmlrpc.Value, error) {
	log.Debugf(ctx, "Servod call %q with %v: starting...", method, args)
	res := s.a.CallServod(ctx, &tlw.CallServodRequest{
		Resource: s.dut.Name,
		Method:   method,
		Args:     xmlrpc_utils.PackArgsToXMLRPCValues(args...),
		Timeout:  durationpb.New(timeout),
	})
	if res.Fault {
		return nil, errors.Reason("call %q: %s", method, res.GetValue().GetScalarOneof()).Err()
	}
	// Shorten output to reduce logging.
	v := fmt.Sprintf("%#v", res.GetValue().GetScalarOneof())
	if len(v) > 20 {
		v = v[:20] + "..."
	}
	log.Debugf(ctx, "Servod call %q with %v: received %s", method, args, v)
	return res.Value, nil
}

// Get read value by requested command.
func (s *iServod) Get(ctx context.Context, command string) (*xmlrpc.Value, error) {
	if command == "" {
		return nil, errors.Reason("get: command is empty").Err()
	}
	v, err := s.Call(ctx, components.ServodGet, components.ServodDefaultTimeout, command)
	return v, errors.Annotate(err, "get %q", command).Err()
}

// Set sets value to provided command.
func (s *iServod) Set(ctx context.Context, command string, val any) error {
	if command == "" {
		return errors.Reason("set: command is empty").Err()
	}
	if val == nil {
		return errors.Reason("set %q: value is empty", command).Err()
	}
	_, err := s.Call(ctx, components.ServodSet, components.ServodDefaultTimeout, command, val)
	return errors.Annotate(err, "set %q with %v", command, val).Err()
}

// Has verifies that command is known.
// Error is returned if the control is not listed in the doc.
func (s *iServod) Has(ctx context.Context, command string) error {
	if command == "" {
		return errors.Reason("has: command not specified").Err()
	}
	_, err := s.Call(ctx, components.ServodDoc, components.ServodDefaultTimeout, command)
	return errors.Annotate(err, "has: %q is not know", command).Err()
}

// Port provides port used for running servod daemon.
func (s *iServod) Port() int {
	return int(s.dut.GetChromeos().GetServo().GetServodPort())
}
