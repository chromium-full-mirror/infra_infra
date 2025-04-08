// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package metadata handing reading of test metadata.
package metadata

import (
	"fmt"
	"os"
	"path/filepath"

	"google.golang.org/protobuf/proto"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/cft/common/errors"
)

// ReadDir reads all test metadata files recursively from a specified root directory.
func ReadDir(dir string) (metadataList *api.TestCaseMetadataList, err error) {
	metadataList = &api.TestCaseMetadataList{}
	walkFunc := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return errors.NewStatusError(errors.IOAccessError,
				fmt.Errorf("failed to access directory %v: %w", dir, err))
		}
		if info.IsDir() {
			return nil
		}
		buf, err := os.ReadFile(path)
		if err != nil {
			return errors.NewStatusError(errors.IOAccessError,
				fmt.Errorf("failed to read file %v: %w", path, err))
		}
		var ml api.TestCaseMetadataList
		if err := proto.Unmarshal(buf, &ml); err != nil {
			// ignore non-metadata file.
			return nil
		}
		metadataList.Values = append(metadataList.Values, ml.Values...)
		return nil
	}
	if err := filepath.Walk(dir, walkFunc); err != nil {
		return nil, errors.NewStatusError(errors.IOAccessError,
			fmt.Errorf("failed to read from directory %v: %w", dir, err))
	}
	return metadataList, nil
}
