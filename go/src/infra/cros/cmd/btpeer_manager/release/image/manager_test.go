// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package image

import (
	"testing"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
)

func TestManager_ValidateConfig(t *testing.T) {
	type args struct {
		config *labapi.RaspiosCrosBtpeerImageConfig
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			"nil config",
			args{
				config: nil,
			},
			true,
		},
		{
			"empty config",
			args{
				config: &labapi.RaspiosCrosBtpeerImageConfig{},
			},
			false,
		},
		{
			"image missing uuid",
			args{
				config: &labapi.RaspiosCrosBtpeerImageConfig{
					Images: []*labapi.RaspiosCrosBtpeerImageConfig_OSImage{
						{
							Path: "pathA",
						},
					},
				},
			},
			true,
		},
		{
			"image missing path",
			args{
				config: &labapi.RaspiosCrosBtpeerImageConfig{
					Images: []*labapi.RaspiosCrosBtpeerImageConfig_OSImage{
						{
							Uuid: "uuidA",
						},
					},
				},
			},
			true,
		},
		{
			"image uuid not unique",
			args{
				config: &labapi.RaspiosCrosBtpeerImageConfig{
					Images: []*labapi.RaspiosCrosBtpeerImageConfig_OSImage{
						{
							Uuid: "uuidA",
							Path: "pathA",
						},
						{
							Uuid: "uuidB",
							Path: "pathB",
						},
						{
							Uuid: "uuidA",
							Path: "pathC",
						},
					},
				},
			},
			true,
		},
		{
			"current image undefined",
			args{
				config: &labapi.RaspiosCrosBtpeerImageConfig{
					Images: []*labapi.RaspiosCrosBtpeerImageConfig_OSImage{
						{
							Uuid: "uuidA",
							Path: "pathA",
						},
					},
					CurrentImageUuid: "uuidB",
				},
			},
			true,
		},
		{
			"next image undefined",
			args{
				config: &labapi.RaspiosCrosBtpeerImageConfig{
					Images: []*labapi.RaspiosCrosBtpeerImageConfig_OSImage{
						{
							Uuid: "uuidA",
							Path: "pathA",
						},
					},
					NextImageUuid: "uuidB",
				},
			},
			true,
		},
		{
			"valid current and no next",
			args{
				config: &labapi.RaspiosCrosBtpeerImageConfig{
					Images: []*labapi.RaspiosCrosBtpeerImageConfig_OSImage{
						{
							Uuid: "uuidA",
							Path: "pathA",
						},
						{
							Uuid: "uuidB",
							Path: "pathB",
						},
						{
							Uuid: "uuidC",
							Path: "pathC",
						},
					},
					CurrentImageUuid: "uuidA",
				},
			},
			false,
		},
		{
			"valid current and next",
			args{
				config: &labapi.RaspiosCrosBtpeerImageConfig{
					Images: []*labapi.RaspiosCrosBtpeerImageConfig_OSImage{
						{
							Uuid: "uuidA",
							Path: "pathA",
						},
						{
							Uuid: "uuidB",
							Path: "pathB",
						},
						{
							Uuid: "uuidC",
							Path: "pathC",
						},
					},
					CurrentImageUuid: "uuidA",
					NextImageUuid:    "uuidB",
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manager{}
			if err := m.ValidateConfig(tt.args.config); (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
