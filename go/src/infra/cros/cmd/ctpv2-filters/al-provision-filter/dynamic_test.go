// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

func TestGetTestType_OSTestType(t *testing.T) {
	req := &api.InternalTestplan{
		SuiteInfo: &api.SuiteInfo{
			SuiteMetadata: &api.SuiteMetadata{
				ExecutionMetadata: &api.ExecutionMetadata{
					Args: []*api.Arg{
						{
							Flag:  "test-type",
							Value: string(common.OSTestType),
						},
					},
				},
			},
		},
	}
	if got := getTestType(req); got != common.OSTestType {
		t.Errorf("getTestType() = %v, want %v", got, common.OSTestType)
	}
}

func TestGetTestType_KernelTestType(t *testing.T) {
	req := &api.InternalTestplan{
		SuiteInfo: &api.SuiteInfo{
			SuiteMetadata: &api.SuiteMetadata{
				ExecutionMetadata: &api.ExecutionMetadata{
					Args: []*api.Arg{
						{
							Flag:  "test-type",
							Value: string(common.KernelTestType),
						},
					},
				},
			},
		},
	}
	if got := getTestType(req); got != common.KernelTestType {
		t.Errorf("getTestType() = %v, want %v", got, common.KernelTestType)
	}
}

func TestGetTestType_DefaultToOS(t *testing.T) {
	reqNoTestTypeArg := &api.InternalTestplan{
		SuiteInfo: &api.SuiteInfo{
			SuiteMetadata: &api.SuiteMetadata{
				ExecutionMetadata: &api.ExecutionMetadata{
					Args: []*api.Arg{
						{
							Flag:  "some-other-flag",
							Value: "some-value",
						},
					},
				},
			},
		},
	}
	if got := getTestType(reqNoTestTypeArg); got != common.OSTestType {
		t.Errorf("getTestType() with no test-type = %v, want %v", got, common.OSTestType)
	}

	reqUnknownTestTypeValue := &api.InternalTestplan{
		SuiteInfo: &api.SuiteInfo{
			SuiteMetadata: &api.SuiteMetadata{
				ExecutionMetadata: &api.ExecutionMetadata{
					Args: []*api.Arg{
						{
							Flag:  "test-type",
							Value: "XYZ",
						},
					},
				},
			},
		},
	}
	if got := getTestType(reqUnknownTestTypeValue); got != common.OSTestType {
		t.Errorf("getTestType() with an unknown test-type value = %v, want %v", got, common.OSTestType)
	}

	reqNoArgs := &api.InternalTestplan{
		SuiteInfo: &api.SuiteInfo{
			SuiteMetadata: &api.SuiteMetadata{
				ExecutionMetadata: &api.ExecutionMetadata{},
			},
		},
	}
	if got := getTestType(reqNoArgs); got != common.OSTestType {
		t.Errorf("getTestType() with no args = %v, want %v", got, common.OSTestType)
	}
}
