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

	"go.chromium.org/infra/cros/recovery/internal/execs/wifirouter/ssh"
	"go.chromium.org/infra/cros/recovery/internal/execs/wifirouter/ssh/mocks"
	"go.chromium.org/infra/cros/recovery/tlw"
)

func Test_remoteFileContentsMatch(t *testing.T) {
	type args struct {
		remoteFilePath       string
		matchRegex           string
		expectFileExistsCall bool
		mockFileExistsResult bool
		expectCatFileCall    bool
		mockCatFileResult    string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			"file does not exist",
			args{
				"some_file.txt",
				"",
				true,
				false,
				false,
				"",
			},
			false,
			false,
		},
		{
			"bad regex",
			args{
				"some_file.txt",
				"???",
				true,
				true,
				false,
				"",
			},
			false,
			true,
		},
		{
			"empty file matches simple regex",
			args{
				"some_file.txt",
				".*",
				true,
				true,
				true,
				"",
			},
			true,
			false,
		},
		{
			"gale match regex",
			args{
				"/etc/lsb-release",
				lsbReleaseMatchIfGale,
				true,
				true,
				true,
				"SOME_VALUE=\nCHROMEOS_RELEASE_BOARD=gale\nOTHER_VALUE=asd\n",
			},
			true,
			false,
		},
		{
			"openwrt match regex",
			args{
				deviceInfoFilePath,
				deviceInfoMatchIfOpenWrt,
				true,
				true,
				true,
				"SOME_VALUE=\nDEVICE_MANUFACTURER='OpenWrt'\nOTHER_VALUE=asd\n",
			},
			true,
			false,
		},
		{
			"ubuntu match regex",
			args{
				"/etc/lsb-release",
				lsbReleaseMatchIfUbuntu,
				true,
				true,
				true,
				"SOME_VALUE=\nDISTRIB_ID=Ubuntu\nOTHER_VALUE=asd\n",
			},
			true,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockRunner := mocks.NewMockRunner(ctrl)
			if tt.args.expectFileExistsCall {
				var fileExistsRunResult ssh.RunResult
				if tt.args.mockFileExistsResult {
					fileExistsRunResult = &tlw.RunResult{
						ExitCode: 0,
					}
				} else {
					fileExistsRunResult = &tlw.RunResult{
						ExitCode: 1,
					}
				}
				mockRunner.EXPECT().
					RunForResult(gomock.Any(), gomock.Any(), false, "test", "-f", gomock.Eq(tt.args.remoteFilePath)).
					Return(fileExistsRunResult)
			}
			if tt.args.expectCatFileCall {
				mockRunner.EXPECT().
					Run(gomock.Any(), gomock.Any(), "cat", gomock.Eq(tt.args.remoteFilePath)).
					Return(tt.args.mockCatFileResult, nil)
			}
			got, err := remoteFileContentsMatch(context.Background(), mockRunner, tt.args.remoteFilePath, tt.args.matchRegex)
			if (err != nil) != tt.wantErr {
				t.Errorf("remoteFileContentsMatch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("remoteFileContentsMatch() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_replaceInvalidWifiRouterFeatures(t *testing.T) {
	tests := []struct {
		name            string
		featuresInitial []labapi.WifiRouterFeature
		featuresAfter   []labapi.WifiRouterFeature
	}{
		{
			"empty list",
			[]labapi.WifiRouterFeature{},
			[]labapi.WifiRouterFeature{},
		},
		{
			"nil list",
			nil,
			nil,
		},
		{
			"all valid",
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX,
			},
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX,
			},
		},
		{
			"single invalid",
			[]labapi.WifiRouterFeature{
				-1,
			},
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_INVALID,
			},
		},
		{
			"mixed valid and invalid",
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
				1234124,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX,
				-1,
				-254,
			},
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_INVALID,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_INVALID,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_INVALID,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replaceInvalidWifiRouterFeatures(tt.featuresInitial)
			if !reflect.DeepEqual(tt.featuresInitial, tt.featuresAfter) {
				t.Errorf("replaceInvalidWifiRouterFeatures() got = %v, want %v", tt.featuresInitial, tt.featuresAfter)
			}
		})
	}
}

func Test_removeDuplicateWifiRouterFeatures(t *testing.T) {
	tests := []struct {
		name        string
		featuresArg []labapi.WifiRouterFeature
		want        []labapi.WifiRouterFeature
	}{
		{
			"empty list",
			[]labapi.WifiRouterFeature{},
			nil,
		},
		{
			"nil list",
			nil,
			nil,
		},
		{
			"no duplicates",
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_UNKNOWN,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_INVALID,
			},
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_UNKNOWN,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_INVALID,
			},
		},
		{
			"with duplicates",
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_UNKNOWN,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_INVALID,
			},
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_UNKNOWN,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_INVALID,
			},
		},
		{
			"with unknown value duplicate",
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature(1235679),
				labapi.WifiRouterFeature(1235679),
				labapi.WifiRouterFeature(652325),
			},
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature(1235679),
				labapi.WifiRouterFeature(652325),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := removeDuplicateWifiRouterFeatures(tt.featuresArg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("removeDuplicateWifiRouterFeatures() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_SortWifiRouterFeaturesByName(t *testing.T) {
	tests := []struct {
		name            string
		featuresInitial []labapi.WifiRouterFeature
		featuresAfter   []labapi.WifiRouterFeature
	}{
		{
			"empty list",
			[]labapi.WifiRouterFeature{},
			[]labapi.WifiRouterFeature{},
		},
		{
			"nil list",
			nil,
			nil,
		},
		{
			"already sorted",
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_INVALID,
			},
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_INVALID,
			},
		},
		{
			"named sort",
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_INVALID,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
			},
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_INVALID,
			},
		},
		{
			"unknown name value sort",
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature(99901),
				labapi.WifiRouterFeature(99902),
				labapi.WifiRouterFeature(99900),
			},
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature(99900),
				labapi.WifiRouterFeature(99901),
				labapi.WifiRouterFeature(99902),
			},
		},
		{
			"mixed sort",
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature(99900),
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_INVALID,
				labapi.WifiRouterFeature(99901),
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
				labapi.WifiRouterFeature(99902),
			},
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_INVALID,
				labapi.WifiRouterFeature(99900),
				labapi.WifiRouterFeature(99901),
				labapi.WifiRouterFeature(99902),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SortWifiRouterFeaturesByName(tt.featuresInitial)
			if !reflect.DeepEqual(tt.featuresInitial, tt.featuresAfter) {
				t.Errorf("SortWifiRouterFeaturesByName() got = %v, want %v", tt.featuresInitial, tt.featuresAfter)
			}
		})
	}
}

func Test_collectCommonWifiRouterFeatures(t *testing.T) {
	type args struct {
		featureSets      [][]labapi.WifiRouterFeature
		excludedFeatures []labapi.WifiRouterFeature
	}
	tests := []struct {
		name string
		args args
		want []labapi.WifiRouterFeature
	}{
		{
			"nil feature sets",
			args{
				nil,
				nil,
			},
			nil,
		},
		{
			"no feature sets",
			args{
				[][]labapi.WifiRouterFeature{},
				nil,
			},
			nil,
		},
		{
			"single empty feature set",
			args{
				[][]labapi.WifiRouterFeature{
					{},
				},
				nil,
			},
			[]labapi.WifiRouterFeature{},
		},
		{
			"single non-empty feature set",
			args{
				[][]labapi.WifiRouterFeature{
					{
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
					},
				},
				nil,
			},
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
			},
		},
		{
			"two feature sets, second is empty",
			args{
				[][]labapi.WifiRouterFeature{
					{
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
					},
					{},
				},
				nil,
			},
			[]labapi.WifiRouterFeature{},
		},
		{
			"two feature sets with no common features",
			args{
				[][]labapi.WifiRouterFeature{
					{
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_BE,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
					},
					{
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_B,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_G,
					},
				},
				nil,
			},
			[]labapi.WifiRouterFeature{},
		},
		{
			"two feature sets with some common features",
			args{
				[][]labapi.WifiRouterFeature{
					{
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_BE,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
					},
					{
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_B,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_G,
					},
				},
				nil,
			},
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
			},
		},
		{
			"three feature sets with some common features",
			args{
				[][]labapi.WifiRouterFeature{
					{
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_BE,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_N,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
					},
					{
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_N,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_B,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_G,
					},
					{
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_B,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_N,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_G,
					},
				},
				nil,
			},
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_N,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
			},
		},
		{
			"two feature sets with some excluded common features",
			args{
				[][]labapi.WifiRouterFeature{
					{
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_BE,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
					},
					{
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_BE,
						labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
					},
				},
				[]labapi.WifiRouterFeature{
					labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
					labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
				},
			},
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_BE,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collectCommonWifiRouterFeatures(tt.args.featureSets, tt.args.excludedFeatures)
			SortWifiRouterFeaturesByName(got)
			SortWifiRouterFeaturesByName(tt.want)
			if !(len(tt.want) == 0 && len(got) == 0) && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collectCommonWifiRouterFeatures() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCollectOverallTestbedWifiRouterFeatures(t *testing.T) {
	type args struct {
		routers []*tlw.WifiRouterHost
	}
	tests := []struct {
		name string
		args args
		want []labapi.WifiRouterFeature
	}{
		{
			"no routers",
			args{
				nil,
			},
			nil,
		},
		{
			"one router with no features",
			args{
				[]*tlw.WifiRouterHost{
					{
						Features: nil,
					},
				},
			},
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_UNKNOWN,
			},
		},
		{
			"one router with an unknown feature",
			args{
				[]*tlw.WifiRouterHost{
					{
						Features: []labapi.WifiRouterFeature{
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_UNKNOWN,
						},
					},
				},
			},
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_UNKNOWN,
			},
		},
		{
			"one router with an unknown feature and another router with valid, common features",
			args{
				[]*tlw.WifiRouterHost{
					{
						Features: []labapi.WifiRouterFeature{
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_UNKNOWN,
						},
					},
					{
						Features: []labapi.WifiRouterFeature{
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
						},
					},
				},
			},
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_UNKNOWN,
			},
		},
		{
			"one router with valid features",
			args{
				[]*tlw.WifiRouterHost{
					{
						Features: []labapi.WifiRouterFeature{
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_BE,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
						},
					},
				},
			},
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_BE,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
			},
		},
		{
			"two routers with some common features",
			args{
				[]*tlw.WifiRouterHost{
					{
						Features: []labapi.WifiRouterFeature{
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_BE,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_N,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
						},
					},
					{
						Features: []labapi.WifiRouterFeature{
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_N,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_B,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_G,
						},
					},
				},
			},
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_N,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
			},
		},
		{
			"two routers with no common features",
			args{
				[]*tlw.WifiRouterHost{
					{
						Features: []labapi.WifiRouterFeature{
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_BE,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
						},
					},
					{
						Features: []labapi.WifiRouterFeature{
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_N,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_B,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_G,
						},
					},
				},
			},
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_INVALID,
			},
		},
		{
			"two routers with common features and one having an invalid feature",
			args{
				[]*tlw.WifiRouterHost{
					{
						Features: []labapi.WifiRouterFeature{
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_BE,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_N,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_INVALID,
						},
					},
					{
						Features: []labapi.WifiRouterFeature{
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_N,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_B,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_G,
						},
					},
				},
			},
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_N,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_INVALID,
			},
		},
		{
			"two routers with common features and both having an invalid feature",
			args{
				[]*tlw.WifiRouterHost{
					{
						Features: []labapi.WifiRouterFeature{
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_BE,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_N,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_INVALID,
						},
					},
					{
						Features: []labapi.WifiRouterFeature{
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_N,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_B,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_G,
							labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_INVALID,
						},
					},
				},
			},
			[]labapi.WifiRouterFeature{
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_N,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
				labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_INVALID,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CollectOverallTestbedWifiRouterFeatures(tt.args.routers)
			SortWifiRouterFeaturesByName(got)
			SortWifiRouterFeaturesByName(tt.want)
			if !(len(tt.want) == 0 && len(got) == 0) && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CollectOverallTestbedWifiRouterFeatures() = %v, want %v", got, tt.want)
			}
		})
	}
}
