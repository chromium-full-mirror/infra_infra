// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servodserver

import (
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"go.chromium.org/chromiumos/config/go/longrunning"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/lro"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/cft/common/portdiscovery"
)

// StartServer starts servod server on requested port
func (s *ServodService) StartServer(port int32) error {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return errors.Annotate(err, "Start servod server: failed to create listener at %d", port).Err()
	}

	s.manager = lro.New()
	defer s.manager.Close()
	// Write port number to ~/.cftmeta for go/cft-port-discovery
	err = portdiscovery.WriteServiceMetadata("cros-servod", l.Addr().String(), s.logger)
	if err != nil {
		s.logger.Println("Warning: error when writing to metadata file: ", err)
	}
	server := grpc.NewServer()

	reflection.Register(server)
	api.RegisterServodServiceServer(server, s)
	longrunning.RegisterOperationsServer(server, s.manager)

	s.logger.Println("Servod server is listening to request at ", l.Addr().String())
	return server.Serve(l)
}
