// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package bq implements bigquery-related logic.
package bq

import (
	"context"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/luci/common/bq"

	apibq "infra/appengine/cros/lab_inventory/api/bigquery"
	"infra/cros/lab_inventory/deviceconfig"
)

// GetPSTTimeStamp returns the PST timestamp for bq table.
func GetPSTTimeStamp(t time.Time) string {
	tz, _ := time.LoadLocation("America/Los_Angeles")
	return t.In(tz).Format("20060102")
}

// InitBQUploaderWithClient initialize a bigquery uploader with a given bigquery client.
func InitBQUploaderWithClient(ctx context.Context, client *bigquery.Client, dataset, table string) *bq.Uploader {
	up := bq.NewUploader(ctx, client, dataset, table)
	up.SkipInvalidRows = true
	up.IgnoreUnknownValues = true
	return up
}

// InitBQUploader initialize a bigquery uploader.
func InitBQUploader(ctx context.Context, project, dataset, table string) (*bq.Uploader, error) {
	client, err := bigquery.NewClient(ctx, project)
	if err != nil {
		return nil, err
	}
	return InitBQUploaderWithClient(ctx, client, dataset, table), nil
}

// GetDeviceConfigProtos prepares the proto messages for all device configs to upload to bq.
func GetDeviceConfigProtos(ctx context.Context) []proto.Message {
	devConfigs, err := deviceconfig.GetAllCachedConfig(ctx)
	if err != nil {
		return nil
	}
	msgs := make([]proto.Message, len(devConfigs))
	i := 0
	for dc, t := range devConfigs {
		ut := timestamppb.New(t)
		msgs[i] = &apibq.DeviceConfigInventory{
			Id:          deviceconfig.GetDeviceConfigIDStr(dc.GetId()),
			Config:      dc,
			UpdatedTime: ut,
		}
		i++
	}
	return msgs
}
