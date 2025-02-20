// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package localtlw provides local implementation of TLW Access.
package localtlw

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	xmlrpclib "go.chromium.org/chromiumos/config/go/api/test/xmlrpc"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/localtlw/localproxy"
	"go.chromium.org/infra/cros/recovery/internal/localtlw/ssh"
	"go.chromium.org/infra/cros/recovery/internal/localtlw/xmlrpc"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

const (
	defaultTouchhostdPort = 9992
	// Local address with dynamic port.
	localAddr = "127.0.0.1:0"
	// Local address template for remote host.
	remoteAddrFmt = "127.0.0.1:%d"
)

// CallTouchHostd executes a command on touchhostd.
func (c *tlwClient) CallTouchHostd(ctx context.Context, req *tlw.CallTouchHostdRequest) *tlw.CallTouchHostdResponse {
	// Translator to convert error to response structure.
	fail := func(err error) *tlw.CallTouchHostdResponse {
		return &tlw.CallTouchHostdResponse{
			Value: &xmlrpclib.Value{
				ScalarOneof: &xmlrpclib.Value_String_{
					String_: fmt.Sprintf("failed to call touchhostd with hostname %s: %s", req.GetResource(), err),
				},
			},
			Fault: true,
		}
	}
	// Check if the name was detected by loaded device.
	_, err := c.getDevice(ctx, req.GetResource())
	if err != nil {
		return fail(err)
	}

	val, err := callTouchHostd(ctx, req, c.sshProvider)
	if err != nil {
		return fail(err)
	}
	return &tlw.CallTouchHostdResponse{
		Value: val,
		Fault: false,
	}

}

// callTouchHostd implements the generic XMLRPC call to any API of touchhostd.
func callTouchHostd(ctx context.Context, req *tlw.CallTouchHostdRequest, sp ssh.SSHProvider) (*xmlrpclib.Value, error) {
	log.Debugf(ctx, "calling hostname %v...", req.GetResource())
	if req.GetMethod() == "" {
		return nil, errors.Reason("missing API method").Err()
	}
	if req.GetResource() == "" {
		return nil, errors.Reason("missing API resource (hostname)").Err()
	}

	// port forwarding
	host := localproxy.BuildAddr(req.GetResource())

	sc, err := sp.Get(ctx, host)
	if err != nil {
		return nil, errors.Annotate(err, "unable to establish SSH client").Err()
	}
	defer func() {
		if err := sc.Close(); err != nil {
			log.Debugf(ctx, "SSH client closed with error: %s", err)
		} else {
			log.Debugf(ctx, "SSH client closed!")
		}
	}()

	remoteAddr := fmt.Sprintf(remoteAddrFmt, defaultTouchhostdPort)
	f, err := sc.ForwardLocalToRemote(localAddr, remoteAddr, func(fErr error) {
		log.Debugf(ctx, "failed while forwarding: %s", fErr)
	})
	if err != nil {
		return nil, errors.Annotate(err, "failed to forward touchhost Pi").Err()
	}
	defer f.Close()

	newAddressStr := f.LocalAddr().String()
	log.Debugf(ctx, "new address: %s", newAddressStr)
	newAddress, newPort, err := addressParser(newAddressStr)
	if err != nil {
		return nil, errors.Annotate(err, "unable to parse address").Err()
	}
	log.Debugf(ctx, "new address after parsed - host: %s , port: %d", newAddress, newPort)

	// prepare the XMLRPC call
	callTimeout := 30 * time.Second
	if req.GetTimeout().GetSeconds() > 0 {
		callTimeout = req.GetTimeout().AsDuration()
	}
	client := xmlrpc.New(newAddress, newPort)
	val, err := callXMLRpc(ctx, client, callTimeout, req.Method, req.GetArgs())
	if err != nil {
		return nil, errors.Annotate(err, "call touchhostd").Err()
	}
	return val, nil
}

// addressParser parses address into host and port
func addressParser(address string) (string, int, error) {
	host, portString, err := net.SplitHostPort(address)
	if err != nil {
		return host, 0, errors.Annotate(err, "unable to split address %s", address).Err()
	}
	newPort, err := strconv.Atoi(portString)
	if err != nil {
		return host, newPort, errors.Annotate(err, "unable to parse port %s", portString).Err()
	}
	return host, newPort, nil
}

// callXMLRpc calls xmlrpc service with provided method and arguments.
func callXMLRpc(ctx context.Context, client *xmlrpc.XMLRpc, timeout time.Duration, method string, args []*xmlrpclib.Value) (*xmlrpclib.Value, error) {
	var iArgs []interface{}
	for _, ra := range args {
		iArgs = append(iArgs, ra)
	}
	log.Debugf(ctx, "calling touchhostd XMLRPC api with timeout %s", timeout)
	call := xmlrpc.NewCallTimeout(timeout, method, iArgs...)
	val := &xmlrpclib.Value{}
	if err := client.Run(ctx, call, val); err != nil {
		return nil, errors.Annotate(err, "call xmlrpc").Err()
	}
	return val, nil
}
