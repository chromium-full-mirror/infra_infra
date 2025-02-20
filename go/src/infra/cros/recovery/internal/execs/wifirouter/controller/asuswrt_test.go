// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/recovery/internal/execs/wifirouter/ssh/mocks"
	"go.chromium.org/infra/cros/recovery/tlw"
)

func TestAsusWrtRouterController_Features(t *testing.T) {
	type fields struct {
		state *tlw.AsusWrtRouterControllerState
	}
	tests := []struct {
		name    string
		fields  fields
		want    []labapi.WifiRouterFeature
		wantErr bool
	}{
		{
			"no state",
			fields{},
			nil,
			true,
		},
		{
			"no model",
			fields{
				state: &tlw.AsusWrtRouterControllerState{},
			},
			nil,
			true,
		},
		{
			"no features for model",
			fields{
				state: &tlw.AsusWrtRouterControllerState{
					AsusModel: "fake model name",
				},
			},
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_UNKNOWN,
			},
			false,
		},
		{
			"got features for model",
			fields{
				state: &tlw.AsusWrtRouterControllerState{
					AsusModel: "RT-AX92U",
				},
			},
			asuswrtModelToFeatures["RT-AX92U"],
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &AsusWrtRouterController{
				state: tt.fields.state,
			}
			got, err := c.Features()
			if (err != nil) != tt.wantErr {
				t.Errorf("Features() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Features() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_readNvramValueByKey(t *testing.T) {
	type args struct {
		nvramKey            string
		mockNvramShowStdout string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			"key not in response",
			args{
				"anything",
				"",
			},
			"",
			true,
		},
		{
			"key in response",
			args{
				"some_key",
				"some_key=abc",
			},
			"abc",
			false,
		},
		{
			"correct key parsed",
			args{
				"some_key",
				"otherkey=\nanother=asdsd\nmore=some_key\nsome_key=abc\nfoo=bar",
			},
			"abc",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockSSHRunner := mocks.NewMockRunner(ctrl)
			mockSSHRunner.
				EXPECT().
				Run(gomock.Any(), gomock.Any(), nvramCmd, "show").
				Return(tt.args.mockNvramShowStdout, nil)
			got, err := readNvramValueByKey(context.Background(), mockSSHRunner, tt.args.nvramKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("readNvramValueByKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("readNvramValueByKey() got = %v, want %v", got, tt.want)
			}
		})
	}
}
