// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dumper

import (
	"context"
	"io"

	"cloud.google.com/go/storage"
	"github.com/golang/protobuf/jsonpb"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/controller"
)

const (
	// Bucket From which GoldenEye data for devices is read
	GoldenEyeBucketName = "chromeos-build-release-console"
	GoldenEyeObjectName = "all_devices.json"
)

func getGoldenEyeData(ctx context.Context) (retErr error) {
	defer func() {
		getGoldenEyeDataTick.Add(ctx, 1, retErr == nil)
	}()
	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		logging.Warningf(ctx, "failed to create cloud storage client while reading golden eye data")
		return err
	}
	bucket := storageClient.Bucket(GoldenEyeBucketName)
	logging.Infof(ctx, "reading Golden Eye Data from https://storage.cloud.google.com/%s/%s", GoldenEyeBucketName, GoldenEyeObjectName)
	r, err := bucket.Object(GoldenEyeObjectName).NewReader(ctx)
	if err != nil {
		return errors.Annotate(err, "creating reader for GoldenEye Object %q", GoldenEyeObjectName).Err()
	}
	defer func() {
		if err := r.Close(); err != nil && retErr == nil {
			retErr = errors.Annotate(err, "closing object reader").Err()
		}
	}()

	devices, retErr := parseGoldenEyeData(ctx, r)
	if retErr != nil {
		return retErr
	}
	retErr = controller.ImportPublicBoardsAndModels(ctx, devices)
	return retErr
}

func parseGoldenEyeData(ctx context.Context, reader io.Reader) (content *ufspb.GoldenEyeDevices, err error) {
	unmarshaler := jsonpb.Unmarshaler{AllowUnknownFields: true}
	content = &ufspb.GoldenEyeDevices{}
	err = unmarshaler.Unmarshal(reader, content)
	if err != nil {
		return nil, errors.Annotate(err, "unmarshal chunk failed while reading golden eye data for devices").Err()
	}
	logging.Infof(ctx, "read Golden Eye Data for %d devices", len(content.Devices))
	return content, nil
}
