// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tricium

import (
	"context"
	"testing"

	"go.chromium.org/luci/auth/identity"
	"go.chromium.org/luci/common/logging/memlogger"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/impl/memory"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"
)

func TestLookupRepoDetails(t *testing.T) {

	pc := &ProjectConfig{
		Repos: []*RepoDetails{
			{
				Source: &RepoDetails_GitRepo{
					GitRepo: &GitRepo{
						Url: "https://github.com/google/gitiles.git",
					},
				},
			},
			{
				Source: &RepoDetails_GerritProject{
					GerritProject: &GerritProject{
						Host:    "chromium.googlesource.com",
						Project: "infra/infra",
						GitUrl:  "https://chromium.googlesource.com/infra/infra.git",
					},
				},
			},
		},
	}

	ftt.Run("Matches GerritProject when URL matches", t, func(t *ftt.Test) {
		request := &AnalyzeRequest{
			Source: &AnalyzeRequest_GerritRevision{
				GerritRevision: &GerritRevision{
					GitUrl: "https://chromium.googlesource.com/infra/infra.git",
					GitRef: "refs/changes/97/12397/1",
				},
			},
		}
		assert.Loosely(t, LookupRepoDetails(pc, request), should.Equal(pc.Repos[1]))
	})

	ftt.Run("Matches GitRepo when URL matches", t, func(t *ftt.Test) {
		request := &AnalyzeRequest{
			Source: &AnalyzeRequest_GitCommit{
				GitCommit: &GitCommit{
					Url: "https://github.com/google/gitiles.git",
					Ref: "refs/heads/master",
				},
			},
		}
		assert.Loosely(t, LookupRepoDetails(pc, request), should.Equal(pc.Repos[0]))
	})

	ftt.Run("Returns nil when no repo is found", t, func(t *ftt.Test) {
		request := &AnalyzeRequest{
			Source: &AnalyzeRequest_GerritRevision{
				GerritRevision: &GerritRevision{
					GitUrl: "https://foo.googlesource.com/bar",
					GitRef: "refs/changes/97/197/2",
				},
			},
		}
		assert.Loosely(t, LookupRepoDetails(pc, request), should.BeNil)
	})
}

func TestCanRequest(t *testing.T) {
	ctx := memory.Use(memlogger.Use(context.Background()))

	okACLGroup := "tricium-playground-requesters"
	okACLUser := "user:ok@example.com"
	pc := &ProjectConfig{
		Acls: []*Acl{
			{
				Role:  Acl_REQUESTER,
				Group: okACLGroup,
			},
			{
				Role:     Acl_REQUESTER,
				Identity: okACLUser,
			},
		},
	}

	ftt.Run("Only users in OK ACL group can request", t, func(t *ftt.Test) {
		ctx = auth.WithState(ctx, &authtest.FakeState{
			Identity:       "user:abc@example.com",
			IdentityGroups: []string{okACLGroup},
		})
		ok, err := CanRequest(ctx, pc)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, ok, should.BeTrue)
	})

	ftt.Run("User with OK ACL can request", t, func(t *ftt.Test) {
		ctx = auth.WithState(ctx, &authtest.FakeState{
			Identity: identity.Identity(okACLUser),
		})
		ok, err := CanRequest(ctx, pc)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, ok, should.BeTrue)
	})

	ftt.Run("Anonymous users cannot request", t, func(t *ftt.Test) {
		ctx = auth.WithState(ctx, &authtest.FakeState{
			Identity: identity.AnonymousIdentity,
		})
		ok, err := CanRequest(ctx, pc)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, ok, should.BeFalse)
	})
}

func TestLookupFunction(t *testing.T) {

	functions := []*Function{
		{
			Name: "Pylint",
			Type: Function_ANALYZER,
		},
	}

	ftt.Run("Known function is known", t, func(t *ftt.Test) {
		assert.Loosely(t, LookupFunction(functions, "Pylint"), should.Match(functions[0]))
	})

	ftt.Run("Unknown function is unknown", t, func(t *ftt.Test) {
		assert.Loosely(t, LookupFunction(functions, "blabla"), should.BeNil)
	})
}

func TestSupportsPlatform(t *testing.T) {
	analyzer := &Function{
		Type: Function_ANALYZER,
		Name: "PyLint",
		Impls: []*Impl{
			{
				ProvidesForPlatform: Platform_WINDOWS,
			},
			{
				ProvidesForPlatform: Platform_UBUNTU,
			},
		},
	}

	ftt.Run("Supported platform is supported", t, func(t *ftt.Test) {
		assert.Loosely(t, SupportsPlatform(analyzer, Platform_UBUNTU), should.BeTrue)
	})

	ftt.Run("Unsupported platform is not supported", t, func(t *ftt.Test) {
		assert.Loosely(t, SupportsPlatform(analyzer, Platform_MAC), should.BeFalse)
	})

	ftt.Run("ANY platform always supported", t, func(t *ftt.Test) {
		assert.Loosely(t, SupportsPlatform(analyzer, Platform_ANY), should.BeTrue)
	})
}

func TestLookupImplForPlatform(t *testing.T) {
	implForLinux := &Impl{ProvidesForPlatform: Platform_LINUX}
	implForMac := &Impl{ProvidesForPlatform: Platform_MAC}
	analyzer := &Function{
		Impls: []*Impl{
			implForLinux,
			implForMac,
		},
	}

	ftt.Run("Impl for known platform is returned", t, func(t *ftt.Test) {
		i := LookupImplForPlatform(analyzer, Platform_LINUX)
		assert.Loosely(t, i, should.Equal(implForLinux))
	})

	ftt.Run("Impl for any platform returns first", t, func(t *ftt.Test) {
		// In this case, there is no implementation in
		// the list that is explicitly for any platform;
		// we return the first implementation.
		i := LookupImplForPlatform(analyzer, Platform_ANY)
		assert.Loosely(t, i, should.Equal(implForLinux))
	})

	ftt.Run("Impl for unknown platform returns nil", t, func(t *ftt.Test) {
		i := LookupImplForPlatform(analyzer, Platform_WINDOWS)
		assert.Loosely(t, i, should.BeNil)
	})

	implForAny := &Impl{ProvidesForPlatform: Platform_ANY}
	analyzer = &Function{
		Impls: []*Impl{
			implForLinux,
			implForAny,
		},
	}

	ftt.Run("Impl for 'any' platform is used if present", t, func(t *ftt.Test) {
		// In this case, there is an implementation in
		// the list that is explicitly for any platform;
		// we return the 'any' implementation.
		i := LookupImplForPlatform(analyzer, Platform_ANY)
		assert.Loosely(t, i, should.Equal(implForAny))
	})
}

func TestLookupPlatform(t *testing.T) {
	platform := Platform_UBUNTU
	sc := &ServiceConfig{Platforms: []*Platform_Details{{Name: platform}}}

	ftt.Run("Known platform is returned", t, func(t *ftt.Test) {
		p := LookupPlatform(sc, platform)
		assert.Loosely(t, p, should.NotBeNil)
	})

	ftt.Run("Unknown platform returns nil", t, func(t *ftt.Test) {
		p := LookupPlatform(sc, Platform_WINDOWS)
		assert.Loosely(t, p, should.BeNil)
	})
}

func TestValidateFunction(t *testing.T) {

	sc := &ServiceConfig{
		Platforms: []*Platform_Details{
			{
				Name:       Platform_LINUX,
				Dimensions: []string{"pool:Default"},
				HasRuntime: true,
			},
			{
				Name:       Platform_IOS,
				Dimensions: []string{"pool:Default"},
				HasRuntime: false,
			},
		},
	}

	ftt.Run("Function with all required fields is valid", t, func(t *ftt.Test) {
		f := &Function{
			Type:     Function_ANALYZER,
			Name:     "PyLint",
			Needs:    Data_FILES,
			Provides: Data_RESULTS,
		}
		assert.Loosely(t, ValidateFunction(f, sc), should.BeNil)
	})

	ftt.Run("Function names must not be non-empty", t, func(t *ftt.Test) {
		f := &Function{
			Type:     Function_ANALYZER,
			Name:     "",
			Needs:    Data_FILES,
			Provides: Data_RESULTS,
		}
		assert.Loosely(t, ValidateFunction(f, sc), should.NotBeNil)
	})

	ftt.Run("Function names must not contain underscore", t, func(t *ftt.Test) {
		f := &Function{
			Type:     Function_ANALYZER,
			Name:     "Py_Lint",
			Needs:    Data_FILES,
			Provides: Data_RESULTS,
		}
		assert.Loosely(t, ValidateFunction(f, sc), should.NotBeNil)
	})

	ftt.Run("Function without type is invalid", t, func(t *ftt.Test) {
		f := &Function{
			Name:     "PyLint",
			Needs:    Data_FILES,
			Provides: Data_RESULTS,
		}
		assert.Loosely(t, ValidateFunction(f, sc), should.NotBeNil)
	})

	ftt.Run("Function without name is invalid", t, func(t *ftt.Test) {
		f := &Function{
			Type:     Function_ANALYZER,
			Needs:    Data_FILES,
			Provides: Data_RESULTS,
		}
		assert.Loosely(t, ValidateFunction(f, sc), should.NotBeNil)
	})

	ftt.Run("Analyzer function must return results", t, func(t *ftt.Test) {
		f := &Function{
			Type:     Function_ANALYZER,
			Name:     "ConfusedAnalyzer",
			Needs:    Data_FILES,
			Provides: Data_GIT_FILE_DETAILS,
		}
		assert.Loosely(t, ValidateFunction(f, sc), should.NotBeNil)
		f.Provides = Data_RESULTS
		assert.Loosely(t, ValidateFunction(f, sc), should.BeNil)
	})

	ftt.Run("Function with impl without platforms is invalid", t, func(t *ftt.Test) {
		f := &Function{
			Type:  Function_ANALYZER,
			Name:  "PyLint",
			Impls: []*Impl{{}},
		}
		assert.Loosely(t, ValidateFunction(f, sc), should.NotBeNil)
	})
}

func TestValidateImpl(t *testing.T) {

	sc := &ServiceConfig{
		Platforms: []*Platform_Details{
			{
				Name:       Platform_UBUNTU,
				Dimensions: []string{"pool:Default"},
				HasRuntime: true,
			},
			{
				Name:       Platform_ANDROID,
				HasRuntime: false,
			},
		},
	}

	anyType := &Data_TypeDetails{
		IsPlatformSpecific: false,
	}

	ftt.Run("Impl must have a recipe specified", t, func(t *ftt.Test) {
		impl := &Impl{
			RuntimePlatform: Platform_UBUNTU,
		}
		assert.Loosely(t, validateImpl(impl, sc, anyType, anyType), should.NotBeNil)
	})
}
