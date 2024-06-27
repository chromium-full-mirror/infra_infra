// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package localtlw provides local implementation of TLW Access.
package localtlw

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc"

	"go.chromium.org/luci/common/errors"

	fleet "infra/appengine/crosskylabadmin/api/fleet/v1"
	"infra/cros/internal/env"
	"infra/cros/recovery/docker"
	"infra/cros/recovery/internal/localtlw/localproxy"
	"infra/cros/recovery/internal/localtlw/ssh"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/internal/rpm"
	"infra/cros/recovery/internal/tls"
	"infra/cros/recovery/tlw"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
)

// UFSClient is a client that knows how to work with UFS RPC methods.
type UFSClient interface {
	// GetDeviceData retrieves requested device data from the UFS and inventoryV2.
	GetDeviceData(ctx context.Context, req *ufsAPI.GetDeviceDataRequest, opts ...grpc.CallOption) (rsp *ufsAPI.GetDeviceDataResponse, err error)
	// UpdateDeviceRecoveryData updates the labdata, dutdata, resource state, dut states for a DUT
	UpdateDeviceRecoveryData(ctx context.Context, in *ufsAPI.UpdateDeviceRecoveryDataRequest, opts ...grpc.CallOption) (*ufsAPI.UpdateDeviceRecoveryDataResponse, error)
}

// CSAClient is a client that knows how to respond to the GetStableVersion RPC call.
type CSAClient interface {
	GetStableVersion(ctx context.Context, in *fleet.GetStableVersionRequest, opts ...grpc.CallOption) (*fleet.GetStableVersionResponse, error)
}

type hostType int64

const (
	hostTypeChromeOs hostType = iota
	hostTypeAndroid
	hostTypeServo
	hostTypeBtPeer
	hostTypeRouter
	hostTypeChameleon
	hostTypeHmrPi
	hostTypeHmrGateway // AKA HMR touchhost
)

// tlwClient holds data and represents the local implementation of TLW Access interface.
type tlwClient struct {
	csaClient   CSAClient
	ufsClient   UFSClient
	sshProvider ssh.SSHProvider
	// Cache received devices from inventory
	devices   map[string]*tlw.Dut
	hostTypes map[string]hostType
	// Map to provide name if the DUT host as value and other hosts as key.
	hostToParents map[string]string
	// Map of version requested and received.
	versionMap map[string]*tlw.VersionResponse
	// Indicates that the client runs on a cloudbot
	isCloudBot bool
}

// New build new local TLW Access instance.
func New(ufs UFSClient, csac CSAClient) (tlw.Access, error) {
	config, err := ssh.NewDefaultConfig(nil)
	if err != nil {
		return nil, errors.Annotate(err, "new tlw client").Err()
	}
	isCloudBot := env.IsCloudBot()
	if isCloudBot {
		if err = config.Load(env.DefaultSSHConfigPathOnCloudBot); err != nil {
			return nil, errors.Annotate(err, "new tlw client").Err()
		}
	}
	c := &tlwClient{
		ufsClient:     ufs,
		csaClient:     csac,
		sshProvider:   ssh.NewProvider(config),
		devices:       make(map[string]*tlw.Dut),
		hostTypes:     make(map[string]hostType),
		hostToParents: make(map[string]string),
		versionMap:    make(map[string]*tlw.VersionResponse),
		isCloudBot:    isCloudBot,
	}
	return c, nil
}

// Close closes all used resources.
func (c *tlwClient) Close(ctx context.Context) error {
	log.Debugf(ctx, "Starting closing client..")
	if err := c.sshProvider.Close(); err != nil {
		return errors.Annotate(err, "close tlw client").Err()
	}
	return nil
}

// Ping performs ping by resource name.
//
// For containers it checks if it is up.
func (c *tlwClient) Ping(ctx context.Context, resourceName string, count int) error {
	dut, err := c.getDevice(ctx, resourceName)
	if err != nil {
		return errors.Annotate(err, "ping").Err()
	}
	if c.isServoHost(resourceName) && isServodContainer(dut) {
		log.Infof(ctx, "Ping: servod container %s starting...", resourceName)
		d, err := c.dockerClient(ctx)
		if err != nil {
			return errors.Annotate(err, "ping").Err()
		}
		containerName := servoContainerName(dut)
		if up, err := d.IsUp(ctx, containerName); err != nil {
			return errors.Annotate(err, "ping").Err()
		} else if up {
			log.Infof(ctx, "Ping: servod container %s is up!", containerName)
			return nil
		}
		return errors.Reason("ping: container %q is down", containerName).Err()
	} else if c.isCloudBot {
		cmd, err := formatPingCommand(resourceName, count)
		if err != nil {
			return errors.Annotate(err, "ping").Err()
		}
		var pingResult *tlw.RunResult
		cr := make(chan bool, 1)
		go func() {
			pingResult = ssh.Run(ctx, c.sshProvider, cloudBotPingServer, "sudo "+strings.Join(cmd[:], " "))
			cr <- true
		}()
		select {
		case <-cr:
			if pingResult.ExitCode != 0 {
				return errors.Reason("ping %s failed: %s", resourceName, pingResult.Stderr).Err()
			}
			return nil
		case <-ctx.Done():
			return errors.Reason("ping %s timeout", resourceName).Err()
		}
	} else {
		err = ping(resourceName, count)
		return errors.Annotate(err, "ping").Err()
	}
}

// Run executes command on device by SSH related to resource name.
//
// Foc containers: For backwards compatibility if command provided without arguments
// we assume the whole command in one string and run it in linux shell (/bin/sh -c).
func (c *tlwClient) Run(ctx context.Context, req *tlw.RunRequest) *tlw.RunResult {
	fullCmd := strings.Join(append([]string{req.GetCommand()}, req.GetArgs()...), " ")
	dut, err := c.getDevice(ctx, req.GetResource())
	if err != nil {
		return &tlw.RunResult{
			Command:  fullCmd,
			ExitCode: -1,
			Stderr:   fmt.Sprintf("run: %s", err),
		}
	}
	log.Debugf(ctx, "Prepare %q to run: %q", req.GetResource(), fullCmd)
	// For backward compatibility we set max limit 1 hour for any request.
	// 1 hour as some provisioning or download can take longer.
	timeout := time.Hour
	if req.GetTimeout().IsValid() {
		timeout = req.GetTimeout().AsDuration()
	}
	// Servod-container does not have ssh access so to execute any commands
	// we need to use the docker client.
	if c.isServoHost(req.GetResource()) && isServodContainer(dut) {
		if req.GetInBackground() {
			log.Infof(ctx, "Container execution is not supported in background!")
			log.Infof(ctx, "Please file a bug if your require to run in background.")
		}
		d, err := c.dockerClient(ctx)
		if err != nil {
			return &tlw.RunResult{
				Command:  fullCmd,
				ExitCode: -1,
				Stderr:   fmt.Sprintf("run: %s", err),
			}
		}
		eReq := &docker.ExecRequest{
			Timeout: timeout,
			Cmd:     append([]string{req.GetCommand()}, req.GetArgs()...),
		}
		containerName := servoContainerName(dut)
		// For backwards compatibility if only command provide we assume
		// that that is whole command in one line. We will run it in linux shell.
		if strings.Contains(req.GetCommand(), " ") && len(req.GetArgs()) == 0 {
			eReq.Cmd = []string{"/bin/sh", "-c", req.GetCommand()}
			// Quoting is only works because the string created for user
			// representation and logs, not for use for execution.
			fullCmd = fmt.Sprintf("/bin/sh -c %q", req.GetCommand())
		}
		containerIsUp, err := d.IsUp(ctx, containerName)
		if err != nil {
			return &tlw.RunResult{
				Command:  fullCmd,
				ExitCode: -1,
				Stderr:   fmt.Sprintf("run: %s", err),
			}
		} else if containerIsUp {
			// As container is created and running we can execute the commands.
			if res, err := d.Exec(ctx, containerName, eReq); err != nil {
				return &tlw.RunResult{
					Command:  fullCmd,
					ExitCode: -1,
					Stderr:   fmt.Sprintf("run: %s", err),
				}
			} else {
				return &tlw.RunResult{
					Command:  fullCmd,
					ExitCode: int32(res.ExitCode),
					Stdout:   res.Stdout,
					Stderr:   res.Stderr,
				}
			}
		} else {
			// If container is down we will run all command directly by container.
			// TODO(otabek): Simplify running a container when move outside.
			containerArgs := createServodContainerArgs(false, nil, nil, eReq.Cmd)
			res, err := d.Start(ctx, containerName, containerArgs, eReq.Timeout)
			if err != nil {
				return &tlw.RunResult{
					Command:  fullCmd,
					ExitCode: -1,
					Stderr:   fmt.Sprintf("run: %s", err),
				}
			}
			return &tlw.RunResult{
				Command:  fullCmd,
				ExitCode: int32(res.ExitCode),
				Stdout:   res.Stdout,
				Stderr:   res.Stderr,
			}
		}
	} else {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		cr := make(chan bool, 1)
		var runResult *tlw.RunResult
		sshProvider := c.sshProvider
		if req.SshUsername != "" {
			sshProvider = sshProvider.Clone()
			sshProvider.SetUser(req.SshUsername)
		}
		go func() {
			addr := localproxy.BuildAddr(req.GetResource())
			if req.GetInBackground() {
				runResult = ssh.RunBackground(ctx, sshProvider, addr, fullCmd)
			} else {
				runResult = ssh.Run(ctx, sshProvider, addr, fullCmd)
			}
			cr <- true
		}()
		select {
		case <-cr:
			log.Debugf(ctx, "Finished SSH command %q on host %q finished in time!", fullCmd, req.GetResource())
			return runResult
		case <-ctx.Done():
			log.Debugf(ctx, "Finished SSH command %q on host %q timed out!", fullCmd, req.GetResource())
			// If we reached timeout first.
			return &tlw.RunResult{
				Command:  fullCmd,
				ExitCode: 124,
				Stderr:   fmt.Sprintf("run: exited due to timeout %s", timeout),
			}
		}
	}
}

// RunRPMAction performs power action on RPM outlet per request.
func (c *tlwClient) RunRPMAction(ctx context.Context, req *tlw.RunRPMActionRequest) error {
	if req.GetHostname() == "" {
		return errors.Reason("run rpm action: hostname of DUT is not provided").Err()
	}
	if req.GetRpmHostname() == "" {
		return errors.Reason("run rpm action: power unit hostname is not provided").Err()
	}
	if req.GetRpmOutlet() == "" {
		return errors.Reason("run rpm action: power unit outlet is not provided").Err()
	}
	var s rpm.PowerState
	switch req.GetAction() {
	case tlw.RunRPMActionRequest_ON:
		s = rpm.PowerStateOn
	case tlw.RunRPMActionRequest_OFF:
		s = rpm.PowerStateOff
	case tlw.RunRPMActionRequest_CYCLE:
		s = rpm.PowerStateCycle
	default:
		return errors.Reason("run rpm action: unknown action: %s", req.GetAction().String()).Err()
	}
	log.Debugf(ctx, "Changing state RPM outlet %s:%s to state %q.", req.GetRpmHostname(), req.GetRpmOutlet(), s)
	rpmReq := &rpm.RPMPowerRequest{
		Hostname:          req.GetHostname(),
		PowerUnitHostname: req.GetRpmHostname(),
		PowerunitOutlet:   req.GetRpmOutlet(),
		State:             s,
	}
	if err := rpm.SetPowerState(ctx, rpmReq); err != nil {
		return errors.Annotate(err, "run rpm action").Err()
	}
	return nil
}

// GetCacheUrl provides URL to download requested path to file.
// URL will use to download image to USB-drive and provisioning.
func (c *tlwClient) GetCacheUrl(ctx context.Context, dutName, filePath string) (string, error) {
	// TODO(otabek@): Add logic to understand local file and just return it back.
	bt, err := tls.NewBackgroundTLS()
	if err != nil {
		return "", errors.Annotate(err, "get cache URL").Err()
	}
	defer func() { _ = bt.Close() }()
	return bt.CacheForDut(ctx, filePath, dutName)
}

// Provision triggers provisioning of the device.
func (c *tlwClient) Provision(ctx context.Context, req *tlw.ProvisionRequest) error {
	if req == nil {
		return errors.Reason("provision: request is empty").Err()
	}
	if req.GetResource() == "" {
		return errors.Reason("provision: resource is not specified").Err()
	}
	if req.GetSystemImagePath() == "" {
		return errors.Reason("provision: system image path is not specified").Err()
	}
	log.Debugf(ctx, "Started provisioning by TLS: %s", req)
	bt, err := tls.NewBackgroundTLS()
	if err != nil {
		return errors.Annotate(err, "tls provision").Err()
	}
	defer func() { _ = bt.Close() }()
	if err := bt.Provision(ctx, req); err != nil {
		return errors.Annotate(err, "provision").Err()
	}
	return nil
}
