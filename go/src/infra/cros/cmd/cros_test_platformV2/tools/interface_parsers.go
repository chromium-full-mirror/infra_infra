// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package parsers

import (
	"fmt"
	"os"

	"github.com/golang/protobuf/jsonpb"

	"go.chromium.org/chromiumos/config/go/test/api"
)

// ReadInput reads a CTPRequest2 jsonproto file and returns a pointer to CTPRequest2.
func ReadInput(fileName string) (*api.CTPv2Request, error) {
	req := api.CTPv2Request{}

	f, err := os.Open(fileName)
	if err != nil {
		return &req, fmt.Errorf("fail to read file %v: %w", fileName, err)
	}
	umrsh := jsonpb.Unmarshaler{}
	umrsh.AllowUnknownFields = true
	if err := umrsh.Unmarshal(f, &req); err != nil {
		return &req, fmt.Errorf("fail to unmarshal file %v: %w", fileName, err)
	}
	return &req, nil
}
