// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package servod provides functions to manage connection and communication with servod daemon on servo-host.
package servod

import (
	"context"
	"fmt"
	"strings"
	"time"

	xmlrpc_value "go.chromium.org/chromiumos/config/go/api/test/xmlrpc"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/localtlw/ssh"
	"go.chromium.org/infra/cros/recovery/internal/localtlw/xmlrpc"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/logger/metrics"
	"go.chromium.org/infra/cros/recovery/tlw"
)

const (
	// Waiting 3 seconds when stopping servod daemon.
	stopServodTimeout = 3
	// Start timeout for servod to verify that servod started.
	servodStartTimeoutSec = 120
	// Local address with dynamic port.
	localAddr = "127.0.0.1:0"
	// Local address template for remote host.
	remoteAddrFmt = "127.0.0.1:%d"
)

// status of servod daemon on servo-host.
type status string

const (
	servodUndefined  status = "UNDEFINED"
	servodRunning    status = "RUNNING"
	servodStopping   status = "STOPPING"
	servodNotRunning status = "NOT_RUNNING"
)

// getServodStatus return status of servod daemon on the servo-host.
func getServodStatus(ctx context.Context, servodHost string, servoPort int32, provider ssh.SSHProvider) (status, error) {
	r := ssh.Run(ctx, provider, servodHost, fmt.Sprintf("status servod PORT=%d", servoPort))
	if r.ExitCode == 0 {
		if strings.Contains(strings.ToLower(r.Stdout), "start/running") {
			return servodRunning, nil
		} else if strings.Contains(strings.ToLower(r.Stdout), "stop/waiting") {
			return servodStopping, nil
		}
	} else if strings.Contains(strings.ToLower(r.Stderr), "unknown instance") {
		return servodNotRunning, nil
	}
	log.Debugf(ctx, "Status check: %s", r.Stderr)
	return servodUndefined, errors.Reason("servo status %q: fail to check status", servodHost).Err()
}

// startServod starts servod daemon on servo-host.
func startServod(ctx context.Context, servodHost string, servoPort int32, params []string, provider ssh.SSHProvider) error {
	log.Infof(ctx, "Start servod with %v", params)
	cmd := strings.Join(append([]string{"start", "servod"}, params...), " ")
	if r := ssh.Run(ctx, provider, servodHost, cmd); r.ExitCode != 0 {
		return errors.Reason("start servod: %s", r.Stderr).Err()
	}
	// Use servodtool to check whether the servod is started.
	log.Debugf(ctx, "Start servod: use servodtool to check and wait the servod on labstation device to be fully started.")
	startTime := time.Now()
	execResult := ssh.Run(ctx, provider, servodHost, fmt.Sprintf("servodtool instance wait-for-active -p %d --timeout %d", servoPort, servodStartTimeoutSec))
	servodStartDuration := time.Since(startTime)
	if execResult.ExitCode != 0 {
		metrics.DefaultActionAddObservations(ctx, metrics.NewFloat64Observation(observationKindStartServodTimeoutFail, servodStartDuration.Seconds()))
		return errors.Reason("start servod: servodtool check: %s", execResult.Stderr).Err()
	}
	metrics.DefaultActionAddObservations(ctx, metrics.NewFloat64Observation(observationKindStartServodTimeoutSuccess, servodStartDuration.Seconds()))
	return nil
}

func stopServod(ctx context.Context, servodHost string, servoPort int32, provider ssh.SSHProvider) error {
	r := ssh.Run(ctx, provider, servodHost, fmt.Sprintf("stop servod PORT=%d", servoPort))
	if r.ExitCode != 0 {
		log.Debugf(ctx, "stop servod: %s", r.Stderr)
		return errors.Reason("stop servod: %s", r.Stderr).Err()
	}
	// Wait to teardown the servod.
	log.Debugf(ctx, "Stop servod: waiting %d seconds to fully teardown the daemon.", stopServodTimeout)
	time.Sleep(stopServodTimeout * time.Second)
	return nil
}

// Call calls xmlrpc service with provided method and arguments.
func Call(ctx context.Context, c *xmlrpc.XMLRpc, timeout time.Duration, method string, args []*xmlrpc_value.Value) (r *xmlrpc_value.Value, rErr error) {
	var iArgs []any
	for _, ra := range args {
		iArgs = append(iArgs, ra)
	}
	call := xmlrpc.NewCallTimeout(timeout, method, iArgs...)
	val := &xmlrpc_value.Value{}
	if err := c.Run(ctx, call, val); err != nil {
		return nil, errors.Annotate(err, "call servod with timeout %s %q: %s", timeout, c.Addr(), method).Err()
	}
	return val, nil
}

// GenerateParams generates command's params based on options.
// Example output:
//
//	"BOARD=${VALUE}" - name of DUT board.
//	"MODEL=${VALUE}" - name of DUT model.
//	"PORT=${VALUE}" - port specified to run servod on servo-host.
//	"SERIAL=${VALUE}" - serial number of root servo.
//	"CONFIG=cr50.xml" - special config for extra ability of CR50.
//	"REC_MODE=1" - start servod in recovery-mode, if root device found then servod will start event not all components detected.
func GenerateParams(o *tlw.ServodOptions) []string {
	var parts []string
	if o == nil {
		return parts
	}
	if o.ServodPort > 0 {
		parts = append(parts, fmt.Sprintf("PORT=%d", o.ServodPort))
	}
	if o.DutBoard != "" {
		parts = append(parts, fmt.Sprintf("BOARD=%s", o.DutBoard))
		if o.DutModel != "" {
			parts = append(parts, fmt.Sprintf("MODEL=%s", o.DutModel))
		}
	}
	if o.ServoSerial != "" {
		parts = append(parts, fmt.Sprintf("SERIAL=%s", o.ServoSerial))
	}
	if o.ServoDual {
		parts = append(parts, "DUAL_V4=1")
	}
	if o.UseCr50Config {
		parts = append(parts, "CONFIG=cr50.xml")
	}
	if o.RecoveryMode {
		parts = append(parts, "REC_MODE=1")
	}
	return parts
}
