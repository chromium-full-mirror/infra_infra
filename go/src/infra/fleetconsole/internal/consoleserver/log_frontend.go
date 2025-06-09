// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import (
	"context"
	"fmt"
	"strings"

	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
)

// Ping is the ping RPC. It responds with an empty response and never fails.
func (frontend *FleetConsoleFrontend) LogFrontend(ctx context.Context, req *fleetconsolerpc.LogFrontendRequest) (*fleetconsolerpc.LogFrontendResponse, error) {
	var b strings.Builder

	b.WriteString(req.GetMessage())
	if req.GetSource() != "" {
		b.WriteString(fmt.Sprintf(" at %s:%d:%d", req.GetSource(), req.GetLineno(), req.GetColno()))
	}

	if req.GetStack() != "" {
		b.WriteString(fmt.Sprintf("\n\nstack: %s", req.GetStack()))
	}

	if req.GetComponentStack() != "" {
		b.WriteString(fmt.Sprintf("\n\ncomponent stack: %s", req.GetComponentStack()))
	}

	if req.GetUrl() != "" {
		b.WriteString(fmt.Sprintf("\n\nurl: %s", req.GetUrl()))
	}

	switch req.Severity {
	case fleetconsolerpc.LogFrontendRequest_INFO:
		logging.Infof(ctx, "Frontend Info: %s", b.String())
	case fleetconsolerpc.LogFrontendRequest_WARNING:
		logging.Warningf(ctx, "Frontend Warning: %s", b.String())
	case fleetconsolerpc.LogFrontendRequest_ERROR:
		logging.Errorf(ctx, "Frontend Error: %s", b.String())
	}

	return &fleetconsolerpc.LogFrontendResponse{}, nil
}
