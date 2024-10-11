// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/tricium/api/v1"
)

func TestValidate(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		functionName := "FunctionName"
		platform := tricium.Platform_UBUNTU
		sd := &tricium.ServiceConfig{
			BuildbucketServerHost: "cr-buildbucket-dev.appspot.com",
			Platforms: []*tricium.Platform_Details{
				{
					Name:       platform,
					HasRuntime: true,
				},
			},
			DataDetails: []*tricium.Data_TypeDetails{
				{
					Type:               tricium.Data_GIT_FILE_DETAILS,
					IsPlatformSpecific: false,
				},
				{
					Type:               tricium.Data_RESULTS,
					IsPlatformSpecific: true,
				},
			},
		}
		functions := []*tricium.Function{
			{
				Type:     tricium.Function_ANALYZER,
				Name:     functionName,
				Needs:    tricium.Data_GIT_FILE_DETAILS,
				Provides: tricium.Data_RESULTS,
				Impls: []*tricium.Impl{
					{
						RuntimePlatform:     platform,
						ProvidesForPlatform: platform,
						Impl: &tricium.Impl_Recipe{
							Recipe: &tricium.Recipe{
								Project: "infra",
								Bucket:  "try",
								Builder: "analysis",
							},
						},
					},
				},
			},
		}

		t.Run("Supported function platform OK", func(t *ftt.Test) {
			err := Validate(sd, &tricium.ProjectConfig{
				Functions: functions,
				Selections: []*tricium.Selection{
					{
						Function: functionName,
						Platform: platform,
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("Non-supported function platform causes error", func(t *ftt.Test) {
			err := Validate(sd, &tricium.ProjectConfig{
				Functions: functions,
				Selections: []*tricium.Selection{
					{
						Function: functionName,
						Platform: tricium.Platform_WINDOWS,
					},
				},
			})
			assert.Loosely(t, err, should.NotBeNil)
		})
	})
}

func TestMergeFunctions(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		functionName := "Analyzer"
		platform := tricium.Platform_UBUNTU
		sc := &tricium.ServiceConfig{
			Platforms: []*tricium.Platform_Details{
				{
					Name:       platform,
					HasRuntime: true,
				},
			},
			DataDetails: []*tricium.Data_TypeDetails{
				{
					Type:               tricium.Data_GIT_FILE_DETAILS,
					IsPlatformSpecific: false,
				},
				{
					Type:               tricium.Data_RESULTS,
					IsPlatformSpecific: true,
				},
			},
		}

		t.Run("Project function def without service def must have data deps", func(t *ftt.Test) {
			_, err := mergeFunction(functionName, sc, nil, &tricium.Function{
				Type: tricium.Function_ANALYZER,
				Name: functionName,
			})
			assert.Loosely(t, err, should.NotBeNil)
		})

		t.Run("Service function def must have data deps", func(t *ftt.Test) {
			_, err := mergeFunction(functionName, sc, &tricium.Function{
				Type: tricium.Function_ANALYZER,
				Name: functionName,
			}, nil)
			assert.Loosely(t, err, should.NotBeNil)
		})

		t.Run("No service function config is OK", func(t *ftt.Test) {
			_, err := mergeFunction(functionName, sc, nil, &tricium.Function{
				Type:     tricium.Function_ANALYZER,
				Name:     functionName,
				Needs:    tricium.Data_GIT_FILE_DETAILS,
				Provides: tricium.Data_RESULTS,
			})
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("No project function config is OK", func(t *ftt.Test) {
			_, err := mergeFunction(functionName, sc, &tricium.Function{
				Type:     tricium.Function_ANALYZER,
				Name:     functionName,
				Needs:    tricium.Data_GIT_FILE_DETAILS,
				Provides: tricium.Data_RESULTS,
			}, nil)
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("Change of service data deps not allowed", func(t *ftt.Test) {
			_, err := mergeFunction(functionName, sc, &tricium.Function{
				Type:     tricium.Function_ANALYZER,
				Name:     functionName,
				Needs:    tricium.Data_GIT_FILE_DETAILS,
				Provides: tricium.Data_RESULTS,
			}, &tricium.Function{
				Type:     tricium.Function_ISOLATOR,
				Name:     functionName,
				Provides: tricium.Data_FILES,
			})
			assert.Loosely(t, err, should.NotBeNil)
		})

		t.Run("Neither service nor function config not OK", func(t *ftt.Test) {
			_, err := mergeFunction(functionName, sc, nil, nil)
			assert.Loosely(t, err, should.NotBeNil)
		})

		t.Run("Project details override service details", func(t *ftt.Test) {
			user := "someone"
			comp := "someonesComp"
			a, err := mergeFunction(functionName, sc, &tricium.Function{
				Type:              tricium.Function_ANALYZER,
				Name:              functionName,
				Needs:             tricium.Data_GIT_FILE_DETAILS,
				Provides:          tricium.Data_RESULTS,
				PathFilters:       []string{"*\\.py", "*\\.pypy"},
				Owner:             "emso",
				MonorailComponent: "compA",
				Impls: []*tricium.Impl{
					{
						ProvidesForPlatform: platform,
						RuntimePlatform:     platform,
						Impl: &tricium.Impl_Recipe{
							Recipe: &tricium.Recipe{
								Project: "infra",
								Bucket:  "try",
								Builder: "analysis",
							},
						},
					},
				},
			}, &tricium.Function{
				Type:              tricium.Function_ANALYZER,
				Name:              functionName,
				PathFilters:       []string{"*\\.py"},
				Owner:             user,
				MonorailComponent: comp,
				Impls: []*tricium.Impl{
					{
						ProvidesForPlatform: platform,
						RuntimePlatform:     platform,
						Impl: &tricium.Impl_Recipe{
							Recipe: &tricium.Recipe{
								Project: "infra",
								Bucket:  "try",
								Builder: "analysis",
							},
						},
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, a, should.NotBeNil)
			assert.Loosely(t, a.Owner, should.Equal(user))
			assert.Loosely(t, a.MonorailComponent, should.Equal(comp))
			assert.Loosely(t, len(a.PathFilters), should.Equal(1))
			assert.Loosely(t, len(a.Impls), should.Equal(1))
		})
	})
}

func TestMergeImpls(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		si := []*tricium.Impl{
			{
				ProvidesForPlatform: tricium.Platform_UBUNTU,
			},
			{
				ProvidesForPlatform: tricium.Platform_WINDOWS,
			},
		}
		pi := []*tricium.Impl{
			{
				ProvidesForPlatform: tricium.Platform_WINDOWS,
			},
			{
				ProvidesForPlatform: tricium.Platform_MAC,
			},
		}
		t.Run("Merges impls with override", func(t *ftt.Test) {
			mi := mergeImpls(si, pi)
			assert.Loosely(t, len(mi), should.Equal(3))
		})
	})
}
