// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package image

import (
	"reflect"
	"testing"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
)

var testImageA = &labapi.RaspiosCrosBtpeerImageConfig_OSImage{
	Uuid: "uuidA",
	Path: "pathA",
}
var testImageB = &labapi.RaspiosCrosBtpeerImageConfig_OSImage{
	Uuid: "uuidB",
	Path: "pathB",
}
var testImageC = &labapi.RaspiosCrosBtpeerImageConfig_OSImage{
	Uuid: "uuidC",
	Path: "pathC",
}

func TestSelectBtpeerImageForDut(t *testing.T) {
	dutA := "dutA"
	dutB := "dutB"
	type args struct {
		config      *labapi.RaspiosCrosBtpeerImageConfig
		dutHostname string
	}
	tests := []struct {
		name    string
		args    args
		want    *labapi.RaspiosCrosBtpeerImageConfig_OSImage
		wantErr bool
	}{
		{
			"nil config",
			args{
				config:      nil,
				dutHostname: "dut1",
			},
			nil,
			true,
		},
		{
			"empty config",
			args{
				config:      &labapi.RaspiosCrosBtpeerImageConfig{},
				dutHostname: "dut1",
			},
			nil,
			true,
		},
		{
			"current dut no next pool",
			args{
				config: &labapi.RaspiosCrosBtpeerImageConfig{
					Images: []*labapi.RaspiosCrosBtpeerImageConfig_OSImage{
						testImageA,
						testImageB,
						testImageC,
					},
					CurrentImageUuid: testImageA.Uuid,
					NextImageUuid:    testImageB.Uuid,
				},
				dutHostname: dutA,
			},
			testImageA,
			false,
		},
		{
			"current dut with next pool",
			args{
				config: &labapi.RaspiosCrosBtpeerImageConfig{
					Images: []*labapi.RaspiosCrosBtpeerImageConfig_OSImage{
						testImageA,
						testImageB,
						testImageC,
					},
					CurrentImageUuid: testImageA.Uuid,
					NextImageUuid:    testImageB.Uuid,
					NextImageVerificationDutPool: []string{
						dutB,
					},
				},
				dutHostname: dutA,
			},
			testImageA,
			false,
		},
		{
			"next dut",
			args{
				config: &labapi.RaspiosCrosBtpeerImageConfig{
					Images: []*labapi.RaspiosCrosBtpeerImageConfig_OSImage{
						testImageA,
						testImageB,
						testImageC,
					},
					CurrentImageUuid: testImageA.Uuid,
					NextImageUuid:    testImageB.Uuid,
					NextImageVerificationDutPool: []string{
						dutB,
					},
				},
				dutHostname: dutB,
			},
			testImageB,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SelectBtpeerImageForDut(tt.args.config, tt.args.dutHostname)
			if (err != nil) != tt.wantErr {
				t.Errorf("SelectBtpeerImageForDut() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SelectBtpeerImageForDut() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelectCurrentBtpeerImage(t *testing.T) {
	type args struct {
		config *labapi.RaspiosCrosBtpeerImageConfig
	}
	tests := []struct {
		name    string
		args    args
		want    *labapi.RaspiosCrosBtpeerImageConfig_OSImage
		wantErr bool
	}{
		{
			"nil config",
			args{
				config: nil,
			},
			nil,
			true,
		},
		{
			"empty config",
			args{
				config: &labapi.RaspiosCrosBtpeerImageConfig{},
			},
			nil,
			true,
		},
		{
			"current undefined",
			args{
				config: &labapi.RaspiosCrosBtpeerImageConfig{
					Images: []*labapi.RaspiosCrosBtpeerImageConfig_OSImage{
						testImageA,
						testImageB,
						testImageC,
					},
				},
			},
			nil,
			true,
		},
		{
			"current defined and missing",
			args{
				config: &labapi.RaspiosCrosBtpeerImageConfig{
					Images: []*labapi.RaspiosCrosBtpeerImageConfig_OSImage{
						testImageA,
						testImageC,
					},
					CurrentImageUuid: testImageB.Uuid,
				},
			},
			nil,
			true,
		},
		{
			"current defined",
			args{
				config: &labapi.RaspiosCrosBtpeerImageConfig{
					Images: []*labapi.RaspiosCrosBtpeerImageConfig_OSImage{
						testImageA,
						testImageB,
						testImageC,
					},
					CurrentImageUuid: testImageB.Uuid,
				},
			},
			testImageB,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SelectCurrentBtpeerImage(tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("SelectCurrentBtpeerImage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SelectCurrentBtpeerImage() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelectBtpeerImageByUUID(t *testing.T) {
	type args struct {
		config    *labapi.RaspiosCrosBtpeerImageConfig
		imageUUID string
	}
	tests := []struct {
		name    string
		args    args
		want    *labapi.RaspiosCrosBtpeerImageConfig_OSImage
		wantErr bool
	}{
		{
			"nil config",
			args{
				config: nil,
			},
			nil,
			true,
		},
		{
			"empty config",
			args{
				config: &labapi.RaspiosCrosBtpeerImageConfig{},
			},
			nil,
			true,
		},
		{
			"no match",
			args{
				config: &labapi.RaspiosCrosBtpeerImageConfig{
					Images: []*labapi.RaspiosCrosBtpeerImageConfig_OSImage{
						testImageA,
						testImageC,
					},
				},
				imageUUID: testImageB.Uuid,
			},
			nil,
			true,
		},
		{
			"match",
			args{
				config: &labapi.RaspiosCrosBtpeerImageConfig{
					Images: []*labapi.RaspiosCrosBtpeerImageConfig_OSImage{
						testImageA,
						testImageB,
						testImageC,
					},
				},
				imageUUID: testImageB.Uuid,
			},
			testImageB,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SelectBtpeerImageByUUID(tt.args.config, tt.args.imageUUID)
			if (err != nil) != tt.wantErr {
				t.Errorf("SelectBtpeerImageByUUID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SelectBtpeerImageByUUID() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelectNextBtpeerImage(t *testing.T) {
	type args struct {
		config *labapi.RaspiosCrosBtpeerImageConfig
	}
	tests := []struct {
		name    string
		args    args
		want    *labapi.RaspiosCrosBtpeerImageConfig_OSImage
		wantErr bool
	}{
		{
			"nil config",
			args{
				config: nil,
			},
			nil,
			true,
		},
		{
			"empty config",
			args{
				config: &labapi.RaspiosCrosBtpeerImageConfig{},
			},
			nil,
			true,
		},
		{
			"next and current undefined",
			args{
				config: &labapi.RaspiosCrosBtpeerImageConfig{
					Images: []*labapi.RaspiosCrosBtpeerImageConfig_OSImage{
						testImageA,
						testImageB,
						testImageC,
					},
				},
			},
			nil,
			true,
		},
		{
			"next undefined and falls back to current",
			args{
				config: &labapi.RaspiosCrosBtpeerImageConfig{
					Images: []*labapi.RaspiosCrosBtpeerImageConfig_OSImage{
						testImageA,
						testImageB,
						testImageC,
					},
					CurrentImageUuid: testImageB.Uuid,
				},
			},
			testImageB,
			false,
		},
		{
			"current and next defined and missing",
			args{
				config: &labapi.RaspiosCrosBtpeerImageConfig{
					Images: []*labapi.RaspiosCrosBtpeerImageConfig_OSImage{
						testImageC,
					},
					CurrentImageUuid: testImageA.Uuid,
					NextImageUuid:    testImageB.Uuid,
				},
			},
			nil,
			true,
		},
		{
			"current and next defined",
			args{
				config: &labapi.RaspiosCrosBtpeerImageConfig{
					Images: []*labapi.RaspiosCrosBtpeerImageConfig_OSImage{
						testImageA,
						testImageB,
						testImageC,
					},
					CurrentImageUuid: testImageA.Uuid,
					NextImageUuid:    testImageB.Uuid,
				},
			},
			testImageB,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SelectNextBtpeerImage(tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("SelectNextBtpeerImage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SelectNextBtpeerImage() got = %v, want %v", got, tt.want)
			}
		})
	}
}
