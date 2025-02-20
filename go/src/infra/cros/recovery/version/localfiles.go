// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package version

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	lab_platform "go.chromium.org/chromiumos/infra/proto/go/lab_platform"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/models"
)

func readLocalVersion(ctx context.Context, board, model string) (Data, error) {
	localDir := os.Getenv("DRONE_RECOVERY_VERSIONS_DIR")
	if localDir == "" {
		return nil, errors.Reason("read local version: target directory is not defined").Err()
	}
	return readLocalVersionFile(ctx, localDir, board, model)
}

func readLocalVersionFile(ctx context.Context, localDir, board, model string) (Data, error) {
	if localDir == "" {
		return nil, errors.Reason("read local version file: directory is not provider").Err()
	}
	if board == "" || model == "" {
		return nil, errors.Reason("read local version file: board or model is empty").Err()
	}
	svFilePath := fmt.Sprintf("%s%s-%s.json", localDir, board, model)
	svFile, err := os.Open(svFilePath)
	if err != nil {
		return nil, errors.Annotate(err, "read local version file: failed to open file %q", svFilePath).Err()
	}
	defer func() {
		if err := svFile.Close(); err != nil {
			log.Debugf(ctx, "Fail to close file %q: %s", svFilePath, err)
		}
	}()

	svByteArr, err := io.ReadAll(svFile)
	if err != nil {
		return nil, errors.Annotate(err, "read local version file %q: failed to read file", svFilePath).Err()
	}

	rv := models.RecoveryVersion{}
	err = json.Unmarshal(svByteArr, &rv)
	if err != nil {
		return nil, errors.Annotate(err, "read local version file %q: failed to parse", svFilePath).Err()
	}
	return &lab_platform.StableVersion{
		OsVersion:           rv.GetOsImage(),
		OsImagePath:         fmt.Sprintf("%s-release/%s", board, rv.GetOsImage()),
		FirmwareRoVersion:   rv.GetFwVersion(),
		FirmwareRoImagePath: rv.GetFwImage(),
	}, nil
}
