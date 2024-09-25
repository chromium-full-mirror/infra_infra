// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package execs

import (
	"context"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/components"
	"infra/cros/recovery/internal/components/cft/adb"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/tlw"
)

var (
	// TODO: need remove these errors to use them from components.
	ErrCodeTag                   = components.ErrCodeTag
	StdErrTag                    = components.StdErrTag
	SSHErrorCLINotFound          = components.SSHErrorCLINotFound
	SSHErrorLinuxTimeout         = components.SSHErrorLinuxTimeout
	GeneralError                 = components.GeneralError
	SSHErrorInternal             = components.SSHErrorInternal
	FailToCreateSSHErrorInternal = components.FailToCreateSSHErrorInternal
	NoExitStatusErrorInternal    = components.NoExitStatusErrorInternal
	OtherErrorInternal           = components.OtherErrorInternal
)

// Runner defines the type for a function that will execute a command
// on a host, and returns the result as a single line.
// TODO: Remove as we do not need extra type.
type Runner = components.Runner

// NewBackgroundRunner returns runner for requested resource specified
// per plan.
//
// TODO(b/222698101): At this time this method is a
// placeholder. This will eventually be replaced with an
// implementation that will submit a command for background execution,
// and will return without waiting for it to complete.
func (ei *ExecInfo) NewBackgroundRunner(host string) components.Runner {
	return ei.newRunner(host, true)
}

// DefaultRunner returns runner for current resource name specified per plan.
func (ei *ExecInfo) DefaultRunner() components.Runner {
	return ei.newRunner(ei.GetActiveResource(), false)
}

// NewRunner returns a function of type Runner that executes a command
// on a host and returns the results as a single line. This function
// defines the specific host on which the command will be
// executed. Examples of such specific hosts can be the DUT, or the
// servo-host etc.
func (ei *ExecInfo) NewRunner(host string) components.Runner {
	return ei.newRunner(host, false)
}

func (ei *ExecInfo) newRunner(host string, inBackground bool) components.Runner {
	ha := ei.NewHostAccess(host)
	runner := func(ctx context.Context, timeout time.Duration, cmd string, args ...string) (string, error) {
		var res components.SSHRunResponse
		var err error
		if inBackground {
			res, err = ha.RunBackground(ctx, timeout, cmd, args...)
		} else {
			res, err = ha.Run(ctx, timeout, cmd, args...)
		}
		return strings.TrimSpace(res.GetStdout()), err
	}
	return runner
}

// hostAccess provides implementation of components.HostAccess interface.
//
// Implementation created in builder approach to simplify configuration.
type hostAccess struct {
	host   string
	user   string
	access tlw.Access
	dut    *tlw.Dut
}

// DefaultHostAccess returns HostAccess for current resource name specified per plan.
func (ei *ExecInfo) DefaultHostAccess() *hostAccess {
	return ei.NewHostAccess(ei.GetActiveResource())
}

// NewHostAccess creates new instance of HostAccess.
func (ei *ExecInfo) NewHostAccess(host string) *hostAccess {
	if ei == nil {
		panic("ExecInfo is nil")
	}
	return &hostAccess{
		host:   host,
		access: ei.GetAccess(),
		dut:    ei.GetDut(),
	}
}

func (b *hostAccess) UseUser(user string) *hostAccess {
	if b == nil {
		panic("Something went wrong as builder is nil")
	}
	b.user = user
	return b
}

// Run executes command by SSH and wait to receive results of the execution.
//
// For all exit codes != `0` an error will be generated.
func (b *hostAccess) Run(ctx context.Context, timeout time.Duration, command string, args ...string) (components.SSHRunResponse, error) {
	return b.run(ctx, false, timeout, command, args...)
}

// Run executes command by SSH and don't wait for results of the execution.
//
// For all exit codes != `0` an error will be generated.
func (b *hostAccess) RunBackground(ctx context.Context, timeout time.Duration, command string, args ...string) (components.SSHRunResponse, error) {
	return b.run(ctx, true, timeout, command, args...)
}

func (b *hostAccess) run(ctx context.Context, inBackground bool, timeout time.Duration, command string, args ...string) (components.SSHRunResponse, error) {
	fullCmd := command
	if len(args) > 0 {
		fullCmd += " " + strings.Join(args, " ")
	}
	if inBackground {
		log.Debugf(ctx, "Prepare to run in background command: %q", fullCmd)
	} else {
		log.Debugf(ctx, "Prepare to run command: %q", fullCmd)
	}
	// TODO(otabek): apply code logic from SSH run.
	adbRun := func() (components.SSHRunResponse, *errors.Annotator) {
		fullCmd = "adb shell " + fullCmd
		client, err := adb.FromScope(ctx, b.dut)
		if err != nil {
			return &adbResponse{
				err:  err.Error(),
				code: -1,
			}, errors.Annotate(err, "runner")
		}
		params := []string{command}
		if len(args) > 0 {
			params = append(params, args...)
		}
		// Response doe snot contains exit code.
		res, err := adb.ShellCommand(ctx, client, timeout, params...)
		if err != nil {
			return &adbResponse{
				err:  err.Error(),
				code: 1,
			}, errors.Annotate(err, "runner")
		}
		return &adbResponse{
			out:  string(res.GetStdout()),
			err:  string(res.GetStderr()),
			code: res.GetExitCode(),
		}, nil
	}
	var errAnnotator *errors.Annotator
	var res components.SSHRunResponse
	if b.host == b.dut.Name && b.dut.Chromeos.GetIsAndroidBased() {
		res, errAnnotator = adbRun()
	} else {
		res = b.access.Run(ctx, &tlw.RunRequest{
			Resource:     b.host,
			Timeout:      durationpb.New(timeout),
			Command:      command,
			Args:         args,
			InBackground: inBackground,
		})
	}
	log.Debugf(ctx, "Run %q completed with exit code %d", fullCmd, res.GetExitCode())
	log.Debugf(ctx, "Run output:\n%s", strings.TrimSpace(res.GetStdout()))
	if res.GetExitCode() == 0 {
		// Success execution.
		return res, nil
	}
	if errAnnotator == nil {
		errAnnotator = errors.Reason("runner: command %q completed with exit code %d", fullCmd, res.GetExitCode())
	}
	// Note: here the exitCode is stored in the field named
	// 'Value' of the TagValue structure. This field is an
	// empty interface. Since we are storing an exitCode of
	// type int32 in this field, we need to be mindful of
	// later comparing this to values of type int32
	// only. Specifically, literal integers are of type 'int',
	// and comparison with such literals will fail even if the
	// value of the literal matches the value of
	// exitCode. Ref: http://b/253326688.
	errCodeTagValue := errors.TagValue{Key: ErrCodeTag, Value: res.GetExitCode()}
	errAnnotator.Tag(errCodeTagValue)
	errAnnotator.Tag(errors.TagValue{Key: StdErrTag, Value: res.GetStderr()})
	log.Debugf(ctx, "Run stderr:\n%s", res.GetStderr())
	// different kinds of internal errors
	if res.GetExitCode() < 0 {
		errAnnotator.Tag(SSHErrorInternal)
		if res.GetExitCode() == -1 {
			errAnnotator.Tag(FailToCreateSSHErrorInternal)
		} else if res.GetExitCode() == -2 {
			errAnnotator.Tag(NoExitStatusErrorInternal)
		} else if res.GetExitCode() == -3 {
			errAnnotator.Tag(OtherErrorInternal)
		}
		// general linux errors
	} else if res.GetExitCode() == 124 {
		errAnnotator.Tag(SSHErrorLinuxTimeout)
	} else if res.GetExitCode() == 127 {
		errAnnotator.Tag(SSHErrorCLINotFound)
	} else {
		errAnnotator.Tag(GeneralError)
	}
	return res, errAnnotator.Err()
}

// adbResponse implements components.SSHRunResponse interface
type adbResponse struct {
	code     int32
	out, err string
}

// Provides exit code.
func (r *adbResponse) GetExitCode() int32 {
	return r.code
}

// Provides standard output.
func (r *adbResponse) GetStdout() string {
	return r.out
}

// Provides standard error output.
func (r *adbResponse) GetStderr() string {
	return r.err
}

// Ping the host.
//
// If error is nil ping is successful.
func (b *hostAccess) Ping(ctx context.Context, pingCount int) error {
	log.Debugf(ctx, "Start ping %q %d times", b.host, pingCount)
	return b.access.Ping(ctx, b.host, pingCount)
}
