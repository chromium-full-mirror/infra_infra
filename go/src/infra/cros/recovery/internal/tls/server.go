// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tls

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/localtlw/ssh"
	tlw_server "infra/cros/recovery/internal/tlw"
	"infra/cros/recovery/tlw"
)

type Server interface {
	Provision(ctx context.Context, sshProvider ssh.SSHProvider, req *tlw.ProvisionRequest) error
	CacheForDut(ctx context.Context, imageURL, dutName string) (string, error)
}

type tlsServer struct {
	tlw tlw_server.Server
}

// New creates new Server instance.
// Please note that is just a legacy name due to keep assisiation with that.
// No Server running here.
func New(tlw tlw_server.Server) (Server, error) {
	return &tlsServer{
		tlw: tlw,
	}, nil
}

// Provision calls TLS service and request provisioning with force.
func (t *tlsServer) Provision(ctx context.Context, sshProvider ssh.SSHProvider, req *tlw.ProvisionRequest) error {
	if t == nil || t.tlw == nil {
		return errors.Reason("Provision: tlw is not provided").Err()
	}
	run := &runner{
		hostname: req.GetResource() + ":22",
		provider: sshProvider,
	}
	err := provision(
		ctx,
		t.tlw,
		run,
		req)
	return errors.Annotate(err, "provision").Err()
}

// CacheForDut queries the underlying TLW server to find a healthy devserver
// with a cached version of the given chromeOS image, and returns the URL
// of the cached image on the devserver.
func (t *tlsServer) CacheForDut(ctx context.Context, imageURL, dutName string) (string, error) {
	if t == nil || t.tlw == nil {
		return "", errors.Reason("CacheForDut: tlw is not provided").Err()
	}
	url, err := t.tlw.CacheForDut(ctx, imageURL, dutName)
	return url, errors.Annotate(err, "CacheForDut").Err()
}
