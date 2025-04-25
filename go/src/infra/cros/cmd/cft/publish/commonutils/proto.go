// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commonutils

import (
	"fmt"
	"os"

	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/chromiumos/config/go/test/api"
)

// ParsePublishRequest parses PublishRequest input request data from
// the input file.
func ParsePublishRequest(path string) (*api.PublishRequest, error) {
	in := &api.PublishRequest{}
	r, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error while opening file at %s: %w", path, err)
	}

	data, err := os.ReadFile(r.Name())
	if err != nil {
		return nil, fmt.Errorf("error while reading file %s: %w", r.Name(), err)
	}

	umrsh := protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
	err = umrsh.Unmarshal(data, in)
	if err != nil {
		return nil, fmt.Errorf("err while unmarshalling: %w", err)
	}

	return in, nil
}
