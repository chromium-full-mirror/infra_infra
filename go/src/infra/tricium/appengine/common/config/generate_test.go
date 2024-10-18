// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"fmt"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/convey"
	"go.chromium.org/luci/common/testing/truth/should"
	admin "infra/tricium/api/admin/v1"
	tricium "infra/tricium/api/v1"
)

const (
	platform = tricium.Platform_UBUNTU
)

func TestGenerate(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		sc := &tricium.ServiceConfig{
			BuildbucketServerHost: "cr-buildbucket-dev.appspot.com",
			Platforms: []*tricium.Platform_Details{
				{
					Name:       platform,
					Dimensions: []string{"pool:Chrome", "os:Ubuntu13.04"},
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
		wrapperAnalyzer := "Wrapper"
		pc := &tricium.ProjectConfig{
			Functions: []*tricium.Function{
				{
					Type:     tricium.Function_ANALYZER,
					Name:     wrapperAnalyzer,
					Needs:    tricium.Data_GIT_FILE_DETAILS,
					Provides: tricium.Data_RESULTS,
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
				},
			},
			Selections: []*tricium.Selection{
				{
					Function: wrapperAnalyzer,
					Platform: platform,
				},
			},
		}

		t.Run("Correct selection generates workflow", func(t *ftt.Test) {
			wf, err := Generate(sc, pc, []*tricium.Data_File{}, "refs/1234/2", "https://chromium-review.googlesource.com/infra")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, len(wf.Workers), should.Equal(1))
		})
	})
}

func TestIncludeFunction(t *testing.T) {
	ftt.Run("No paths means function is included", t, func(t *ftt.Test) {
		ok, err := includeFunction(&tricium.Function{
			Type:        tricium.Function_ANALYZER,
			PathFilters: []string{"*.cc", "*.cpp"},
		}, nil)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, ok, should.BeTrue)
	})

	ftt.Run("No path filters means function is included", t, func(t *ftt.Test) {
		ok, err := includeFunction(&tricium.Function{
			Type: tricium.Function_ANALYZER,
		}, []*tricium.Data_File{
			{Path: "README.md"},
			{Path: "path/foo.cc"},
		})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, ok, should.BeTrue)
	})

	ftt.Run("Analyzer is included when any path matches filter", t, func(t *ftt.Test) {
		ok, err := includeFunction(&tricium.Function{
			Type:        tricium.Function_ANALYZER,
			PathFilters: []string{"*.cc", "*.cpp"},
		}, []*tricium.Data_File{
			{Path: "README.md"},
			{Path: "path/foo.cc"},
		})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, ok, should.BeTrue)
	})

	ftt.Run("Analyzer function is not included when there is no match", t, func(t *ftt.Test) {
		ok, err := includeFunction(&tricium.Function{
			Type:        tricium.Function_ANALYZER,
			PathFilters: []string{"*.cc", "*.cpp"},
		}, []*tricium.Data_File{
			{Path: "whitespace.txt"},
		})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, ok, should.BeFalse)
	})
}

func TestCreateWorker(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		analyzer := "wrapper"
		gitRef := "refs/1234/2"
		gitURL := "https://chromium-review.googlesource.com/infra"
		selection := &tricium.Selection{
			Function: analyzer,
			Platform: platform,
		}
		dimension := "pool:Default"
		sc := &tricium.ServiceConfig{
			Platforms: []*tricium.Platform_Details{
				{
					Name:       platform,
					Dimensions: []string{dimension},
				},
			},
		}

		t.Run("Correctly creates recipe-based worker", func(t *ftt.Test) {
			f := &tricium.Function{
				Name:     analyzer,
				Needs:    tricium.Data_GIT_FILE_DETAILS,
				Provides: tricium.Data_RESULTS,
				Impls: []*tricium.Impl{
					{
						ProvidesForPlatform: platform,
						RuntimePlatform:     platform,
						Impl: &tricium.Impl_Recipe{
							Recipe: &tricium.Recipe{
								Project: "chromium",
								Bucket:  "try",
								Builder: "analysis",
							},
						},
					},
				},
			}
			w, err := createWorker(selection, sc, f, gitRef, gitURL)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, w.Name, should.Equal(fmt.Sprintf("%s_%s", analyzer, platform)))
			assert.Loosely(t, w.Needs, should.Equal(f.Needs))
			assert.Loosely(t, w.Provides, should.Equal(f.Provides))
			assert.Loosely(t, w.ProvidesForPlatform, should.Equal(platform))
			wi := w.Impl.(*admin.Worker_Recipe)
			if wi == nil {
				fail(t, "Incorrect worker type")
			}
			//lint:ignore SA5011 fail will stop the test
			assert.Loosely(t, wi.Recipe.Project, should.Equal("chromium"))
		})
	})
}

func fail(t testing.TB, str string) {
	assert.Loosely(t, nil, convey.Adapt(func(a interface{}, b ...interface{}) string { return str })(nil))
}
