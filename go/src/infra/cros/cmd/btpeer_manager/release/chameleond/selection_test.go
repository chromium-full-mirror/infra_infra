// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chameleond

import (
	"testing"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
)

func TestSelectChameleondBundleByChameleondCommit(t *testing.T) {
	type args struct {
		config           *labapi.BluetoothPeerChameleondConfig
		chameleondCommit string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			"no bundles",
			args{
				&labapi.BluetoothPeerChameleondConfig{},
				"abc",
			},
			true,
		},
		{
			"no matching bundles",
			args{
				&labapi.BluetoothPeerChameleondConfig{
					Bundles: []*labapi.BluetoothPeerChameleondConfig_ChameleondBundle{
						{
							ChameleondCommit:     "def",
							ArchivePath:          "",
							MinDutReleaseVersion: "",
						},
					},
				},
				"abc",
			},
			true,
		},
		{
			"matching bundle",
			args{
				&labapi.BluetoothPeerChameleondConfig{
					Bundles: []*labapi.BluetoothPeerChameleondConfig_ChameleondBundle{
						{
							ChameleondCommit:     "abc",
							ArchivePath:          "",
							MinDutReleaseVersion: "",
						},
						{
							ChameleondCommit:     "def",
							ArchivePath:          "",
							MinDutReleaseVersion: "",
						},
						{
							ChameleondCommit:     "ghi",
							ArchivePath:          "",
							MinDutReleaseVersion: "",
						},
					},
				},
				"def",
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SelectChameleondBundleByChameleondCommit(tt.args.config, tt.args.chameleondCommit)
			if err != nil {
				if !tt.wantErr {
					t.Errorf("SelectChameleondBundleByChameleondCommit() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			if got.ChameleondCommit != tt.args.chameleondCommit {
				t.Errorf("SelectChameleondBundleByChameleondCommit() got commit = %v, want commit %v", got.ChameleondCommit, tt.args.chameleondCommit)
			}
		})
	}
}

func TestSelectChameleondBundleByCrosReleaseVersion(t *testing.T) {
	config1 := &labapi.BluetoothPeerChameleondConfig{
		Bundles: []*labapi.BluetoothPeerChameleondConfig_ChameleondBundle{
			{
				ChameleondCommit:     "a",
				ArchivePath:          "",
				MinDutReleaseVersion: "10",
			},
			{
				ChameleondCommit:     "b",
				ArchivePath:          "",
				MinDutReleaseVersion: "30",
			},
			{
				ChameleondCommit:     "c",
				ArchivePath:          "",
				MinDutReleaseVersion: "20",
			},
		},
	}
	config2 := &labapi.BluetoothPeerChameleondConfig{
		NextChameleondCommit: "b",
		Bundles: []*labapi.BluetoothPeerChameleondConfig_ChameleondBundle{
			{
				ChameleondCommit:     "a",
				ArchivePath:          "",
				MinDutReleaseVersion: "10",
			},
			{
				ChameleondCommit:     "b",
				ArchivePath:          "",
				MinDutReleaseVersion: "30",
			},
			{
				ChameleondCommit:     "c",
				ArchivePath:          "",
				MinDutReleaseVersion: "20",
			},
		},
	}
	type args struct {
		config                *labapi.BluetoothPeerChameleondConfig
		dutCrosReleaseVersion string
	}
	tests := []struct {
		name       string
		args       args
		wantCommit string
		wantErr    bool
	}{
		{
			"no matching",
			args{
				config1,
				"5",
			},
			"",
			true,
		},
		{
			"bad version",
			args{
				config1,
				"abcd",
			},
			"",
			true,
		},
		{
			"matching mid",
			args{
				config1,
				"25",
			},
			"c",
			false,
		},
		{
			"matching highest",
			args{
				config1,
				"4000",
			},
			"b",
			false,
		},
		{
			"matching highest excluding next",
			args{
				config2,
				"4000",
			},
			"c",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SelectChameleondBundleByCrosReleaseVersion(tt.args.config, tt.args.dutCrosReleaseVersion)
			if err != nil {
				if !tt.wantErr {
					t.Errorf("SelectChameleondBundleByCrosReleaseVersion() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			if got.ChameleondCommit != tt.wantCommit {
				t.Errorf("SelectChameleondBundleByCrosReleaseVersion() got commit = %v, want commit %v", got.ChameleondCommit, tt.wantCommit)
			}
		})
	}
}

func TestSelectChameleondBundleByNextCommit(t *testing.T) {
	type args struct {
		config *labapi.BluetoothPeerChameleondConfig
	}
	tests := []struct {
		name       string
		args       args
		wantCommit string
		wantErr    bool
	}{
		{
			"no bundles",
			args{
				&labapi.BluetoothPeerChameleondConfig{},
			},
			"",
			true,
		},
		{
			"no next bundle",
			args{
				&labapi.BluetoothPeerChameleondConfig{
					Bundles: []*labapi.BluetoothPeerChameleondConfig_ChameleondBundle{
						{
							ChameleondCommit:     "abc",
							ArchivePath:          "",
							MinDutReleaseVersion: "",
						},
						{
							ChameleondCommit:     "def",
							ArchivePath:          "",
							MinDutReleaseVersion: "",
						},
						{
							ChameleondCommit:     "ghi",
							ArchivePath:          "",
							MinDutReleaseVersion: "",
						},
					},
				},
			},
			"",
			true,
		},
		{
			"next bundle selected",
			args{
				&labapi.BluetoothPeerChameleondConfig{
					NextChameleondCommit: "def",
					Bundles: []*labapi.BluetoothPeerChameleondConfig_ChameleondBundle{
						{
							ChameleondCommit:     "abc",
							ArchivePath:          "",
							MinDutReleaseVersion: "",
						},
						{
							ChameleondCommit:     "def",
							ArchivePath:          "",
							MinDutReleaseVersion: "",
						},
						{
							ChameleondCommit:     "ghi",
							ArchivePath:          "",
							MinDutReleaseVersion: "",
						},
					},
				},
			},
			"def",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SelectChameleondBundleByNextCommit(tt.args.config)
			if err != nil {
				if !tt.wantErr {
					t.Errorf("SelectChameleondBundleByNextCommit() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			if got.ChameleondCommit != tt.wantCommit {
				t.Errorf("SelectChameleondBundleByNextCommit() got commit = %v, want commit %v", got.ChameleondCommit, tt.wantCommit)
			}
		})
	}
}

func TestSelectChameleondBundleForDut(t *testing.T) {
	configWithNoNextBundle := &labapi.BluetoothPeerChameleondConfig{
		Bundles: []*labapi.BluetoothPeerChameleondConfig_ChameleondBundle{
			{
				ChameleondCommit:     "a",
				ArchivePath:          "",
				MinDutReleaseVersion: "10",
			},
			{
				ChameleondCommit:     "b",
				ArchivePath:          "",
				MinDutReleaseVersion: "30",
			},
			{
				ChameleondCommit:     "c",
				ArchivePath:          "",
				MinDutReleaseVersion: "20",
			},
		},
	}
	configWithNextBundle := &labapi.BluetoothPeerChameleondConfig{
		NextChameleondCommit: "b",
		NextDutHosts: []string{
			"host1",
			"host2",
		},
		NextDutReleaseVersions: []string{
			"30",
			"40",
		},
		Bundles: []*labapi.BluetoothPeerChameleondConfig_ChameleondBundle{
			{
				ChameleondCommit:     "a",
				ArchivePath:          "",
				MinDutReleaseVersion: "10",
			},
			{
				ChameleondCommit:     "b",
				ArchivePath:          "",
				MinDutReleaseVersion: "30",
			},
			{
				ChameleondCommit:     "c",
				ArchivePath:          "",
				MinDutReleaseVersion: "20",
			},
		},
	}
	type args struct {
		config                *labapi.BluetoothPeerChameleondConfig
		dutHostname           string
		dutCrosReleaseVersion string
	}
	tests := []struct {
		name       string
		args       args
		wantCommit string
		wantErr    bool
	}{
		{
			"bad version",
			args{
				configWithNoNextBundle,
				"host1",
				"abc",
			},
			"",
			true,
		},
		{
			"no next bundle, select by version",
			args{
				configWithNoNextBundle,
				"host1",
				"15",
			},
			"a",
			false,
		},
		{
			"only host matches for next, select non-next by version",
			args{
				configWithNextBundle,
				"host1",
				"45",
			},
			"c",
			false,
		},
		{
			"only version matches for next, select non-next by version",
			args{
				configWithNextBundle,
				"host3",
				"40",
			},
			"c",
			false,
		},
		{
			"both host and version matches for next, select next version",
			args{
				configWithNextBundle,
				"host2",
				"40",
			},
			"b",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SelectChameleondBundleForDut(tt.args.config, tt.args.dutHostname, tt.args.dutCrosReleaseVersion)
			if err != nil {
				if !tt.wantErr {
					t.Errorf("SelectChameleondBundleForDut() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			if got.ChameleondCommit != tt.wantCommit {
				t.Errorf("SelectChameleondBundleForDut() got commit = %v, want commit %v", got.ChameleondCommit, tt.wantCommit)
			}
		})
	}
}
