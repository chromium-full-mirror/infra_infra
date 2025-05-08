// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package server implement lsnexus-service API.
package server

import (
	"context"
	"fmt"

	"go.chromium.org/chromiumos/config/go/test/api/bols"
	"go.chromium.org/chromiumos/config/go/test/api/lsnexus"
)

func (s *service) StartServod(ctx context.Context, req *lsnexus.StartServodRequest) (*lsnexus.StartServodResponse, error) {
	s.logger.Println("Serving StartServod request")
	bolsReq := &bols.StartServodRequest{
		StationId: &bols.StationIdentifier{
			ServodPort:    s.dutTopology.GetChromeos().GetServo().GetServodAddress().GetPort(),
			ServoSerial:   s.dutTopology.GetChromeos().GetServo().GetSerial(),
			ContainerName: s.dutTopology.GetChromeos().GetServo().GetContainerName(),
		},
		Board: s.dutTopology.GetChromeos().GetDutModel().GetBuildTarget(),
		Model: s.dutTopology.GetChromeos().GetDutModel().GetModelName(),
	}
	if _, err := s.cl.StartServod(ctx, bolsReq); err != nil {
		err = fmt.Errorf("failed to start servod: %w", err)
		s.logger.Println(err)
		return nil, err
	}
	s.logger.Println("Successfully served StartServod request")
	return &lsnexus.StartServodResponse{}, nil
}
