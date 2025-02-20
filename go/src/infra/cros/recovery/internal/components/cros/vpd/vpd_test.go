// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vpd

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	"go.chromium.org/infra/cros/recovery/internal/components/mocks"
)

func TestSet(t *testing.T) {
	testCases := []struct {
		name  string
		key   string
		val   string
		calls []string
	}{
		{"t1", "k", "v", []string{"vpd -s k=v"}},
		{"t2", "K2", "V2", []string{"vpd -s K2=V2"}},
	}
	for _, tc := range testCases {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		hostAccess := mocks.NewMockHostAccess(ctrl)
		for _, ec := range tc.calls {
			hostAccess.EXPECT().Run(ctx, time.Minute, ec).Return(nil, nil).Times(1)
		}
		err := Set(ctx, hostAccess, time.Minute, tc.key, tc.val)
		if err != nil {
			t.Errorf("%q got error even not expected", tc.name)
		}
	}
}

type runResponse struct {
	StdOut   string
	StdErr   string
	ExitCode int32
}

func (r *runResponse) GetExitCode() int32 { return r.ExitCode }
func (r *runResponse) GetStdout() string  { return r.StdOut }
func (r *runResponse) GetStderr() string  { return r.StdErr }

func TestRead(t *testing.T) {
	testCases := []struct {
		name      string
		key       string
		expect    string
		expectErr bool
		calls     map[string]string
	}{
		{"t1", "k", "v", false, map[string]string{
			"vpd -g k": "v",
		}},
		{"t1", "K2", "V2", false, map[string]string{
			"vpd -g K2": "V2",
		}},
	}
	for _, tc := range testCases {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		hostAccess := mocks.NewMockHostAccess(ctrl)
		for call, stdout := range tc.calls {
			res := &runResponse{
				StdOut: stdout,
			}
			hostAccess.EXPECT().Run(ctx, time.Minute, call).Return(res, nil).Times(1)
		}
		got, err := Read(ctx, hostAccess, time.Minute, tc.key)
		if err != nil {
			t.Errorf("%q got error even not expected", tc.name)
		} else if got != tc.expect {
			t.Errorf("%q result is different got: %q, expected: %q error even not expected", tc.name, got, tc.expect)
		}
	}
}

func TestReadRO(t *testing.T) {
	testCases := []struct {
		name      string
		key       string
		expect    string
		expectErr bool
		calls     map[string]string
	}{
		{"t1", "k", "v", false, map[string]string{
			"vpd -i RO_VPD -g k": "v",
		}},
		{"t1", "K2", "V2", false, map[string]string{
			"vpd -i RO_VPD -g K2": "V2",
		}},
	}
	for _, tc := range testCases {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		hostAccess := mocks.NewMockHostAccess(ctrl)
		for call, stdout := range tc.calls {
			res := &runResponse{
				StdOut: stdout,
			}
			hostAccess.EXPECT().Run(ctx, time.Minute, call).Return(res, nil).Times(1)
		}
		got, err := ReadRO(ctx, hostAccess, time.Minute, tc.key)
		if err != nil {
			t.Errorf("%q got error even not expected", tc.name)
		} else if got != tc.expect {
			t.Errorf("%q result is different got: %q, expected: %q error even not expected", tc.name, got, tc.expect)
		}
	}
}
