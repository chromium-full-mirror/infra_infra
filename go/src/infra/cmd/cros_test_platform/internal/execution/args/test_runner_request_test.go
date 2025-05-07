// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package args contains the logic for assembling all data required for
// creating an individual task request.
package args

import (
	"context"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"

	buildapi "go.chromium.org/chromiumos/infra/proto/go/chromite/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/steps"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func defaultTest(t testing.TB, tests map[string]*skylab_test_runner.Request_Test) *skylab_test_runner.Request_Test {
	assert.Loosely(t, tests["original_test"], should.NotBeNil, truth.LineContext())
	return tests["original_test"]
}

func TestSoftwareDependencies(t *testing.T) {
	cases := []struct {
		Tag  string
		Deps []*test_platform.Request_Params_SoftwareDependency
	}{
		{
			Tag: "Chrome OS build",
			Deps: []*test_platform.Request_Params_SoftwareDependency{
				{
					Dep: &test_platform.Request_Params_SoftwareDependency_ChromeosBuild{
						ChromeosBuild: "foo",
					},
				},
			},
		},
		{
			Tag: "RO firmware",
			Deps: []*test_platform.Request_Params_SoftwareDependency{
				{
					Dep: &test_platform.Request_Params_SoftwareDependency_RoFirmwareBuild{
						RoFirmwareBuild: "foo",
					},
				},
			},
		},
		{
			Tag: "RW firmware",
			Deps: []*test_platform.Request_Params_SoftwareDependency{
				{
					Dep: &test_platform.Request_Params_SoftwareDependency_RwFirmwareBuild{
						RwFirmwareBuild: "foo",
					},
				},
			},
		},
		{
			Tag: "lacros",
			Deps: []*test_platform.Request_Params_SoftwareDependency{
				{
					Dep: &test_platform.Request_Params_SoftwareDependency_LacrosGcsPath{
						LacrosGcsPath: "gs://some-bucket/some-build",
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Tag, func(t *testing.T) {
			g := Generator{
				Invocation: basicInvocation(),
				Params: &test_platform.Request_Params{
					SoftwareDependencies: c.Deps,
				},
			}
			got, err := g.testRunnerRequest(context.Background())
			if err != nil {
				t.Fatalf("g.testRunnerRequest() returned error: %s", err)
			}
			assert.That(t, got.GetPrejob().GetSoftwareDependencies(), should.Match(c.Deps))
		})
	}
}

func TestClientTest(t *testing.T) {
	ftt.Run("Given a client test", t, func(t *ftt.Test) {
		ctx := context.Background()
		var inv steps.EnumerationResponse_AutotestInvocation
		setExecutionEnvironment(&inv, buildapi.AutotestTest_EXECUTION_ENVIRONMENT_CLIENT)
		t.Run("when generating a test runner request", func(t *ftt.Test) {
			g := Generator{
				Invocation: &inv,
				Params:     &test_platform.Request_Params{},
			}
			got, err := g.testRunnerRequest(ctx)
			test := defaultTest(t, got.Tests)
			assert.Loosely(t, err, should.BeNil)
			t.Run("it should be marked as such.", func(t *ftt.Test) {
				assert.Loosely(t, test, should.NotBeNil)
				assert.Loosely(t, test.GetAutotest(), should.NotBeNil)
				assert.Loosely(t, test.GetAutotest().IsClientTest, should.Equal(true))
			})
		})
	})
}

func TestServerTest(t *testing.T) {
	ftt.Run("Given a server test", t, func(t *ftt.Test) {
		ctx := context.Background()
		var inv steps.EnumerationResponse_AutotestInvocation
		setExecutionEnvironment(&inv, buildapi.AutotestTest_EXECUTION_ENVIRONMENT_SERVER)
		t.Run("when generating a test runner request", func(t *ftt.Test) {
			g := Generator{
				Invocation: &inv,
				Params:     &test_platform.Request_Params{},
			}
			got, err := g.testRunnerRequest(ctx)
			test := defaultTest(t, got.Tests)
			assert.Loosely(t, err, should.BeNil)
			t.Run("it should be marked as such.", func(t *ftt.Test) {
				assert.Loosely(t, test, should.NotBeNil)
				assert.Loosely(t, test.GetAutotest(), should.NotBeNil)
				assert.Loosely(t, test.GetAutotest().IsClientTest, should.Equal(false))
			})
		})
	})
}

func TestUnspecifiedTestEnvironment(t *testing.T) {
	ftt.Run("Given a test that does not specify an environment", t, func(t *ftt.Test) {
		ctx := context.Background()
		var inv steps.EnumerationResponse_AutotestInvocation
		setTestName(&inv, "foo-test")
		t.Run("the test runner request generation fails.", func(t *ftt.Test) {
			g := Generator{
				Invocation: &inv,
				Params:     &test_platform.Request_Params{},
			}
			_, err := g.testRunnerRequest(ctx)
			assert.Loosely(t, err, should.NotBeNil)
		})
	})
}

func TestTestName(t *testing.T) {
	ftt.Run("Given a test", t, func(t *ftt.Test) {
		ctx := context.Background()
		inv := basicInvocation()
		setTestName(inv, "foo-test")
		t.Run("when generating a test runner request", func(t *ftt.Test) {
			g := Generator{
				Invocation: inv,
				Params:     &test_platform.Request_Params{},
			}
			got, err := g.testRunnerRequest(ctx)
			assert.Loosely(t, err, should.BeNil)
			test := defaultTest(t, got.Tests)
			t.Run("the test name is populated correctly.", func(t *ftt.Test) {
				assert.Loosely(t, test, should.NotBeNil)
				assert.Loosely(t, test.GetAutotest(), should.NotBeNil)
				assert.Loosely(t, test.GetAutotest().Name, should.Equal("foo-test"))
			})
		})
	})
}

func TestTestArgs(t *testing.T) {
	ftt.Run("Given a request that specifies test args", t, func(t *ftt.Test) {
		ctx := context.Background()
		inv := basicInvocation()
		setTestArgs(inv, "foo=bar baz=qux")
		t.Run("when generating a test runner request", func(t *ftt.Test) {
			g := Generator{
				Invocation: inv,
				Params:     &test_platform.Request_Params{},
			}
			got, err := g.testRunnerRequest(ctx)
			assert.Loosely(t, err, should.BeNil)
			test := defaultTest(t, got.Tests)
			t.Run("the test args are propagated correctly.", func(t *ftt.Test) {
				assert.Loosely(t, test, should.NotBeNil)
				assert.Loosely(t, test.GetAutotest(), should.NotBeNil)
				assert.Loosely(t, test.GetAutotest().TestArgs, should.Equal("foo=bar baz=qux"))
			})
		})
	})
}

func TestParamsTestArgs(t *testing.T) {
	ftt.Run("Given a request that specifies additional test args", t, func(t *ftt.Test) {
		ctx := context.Background()
		inv := basicInvocation()
		setTestArgs(inv, "foo=bar baz=qux")
		var params test_platform.Request_Params
		setParamsTestArgs(&params, "waldo", "fred")
		t.Run("when generating a test runner request", func(t *ftt.Test) {
			g := Generator{
				Invocation: inv,
				Params:     &params,
			}
			got, err := g.testRunnerRequest(ctx)
			assert.Loosely(t, err, should.BeNil)
			test := defaultTest(t, got.Tests)
			t.Run("the test args are propagated correctly.", func(t *ftt.Test) {
				assert.Loosely(t, test, should.NotBeNil)
				assert.Loosely(t, test.GetAutotest(), should.NotBeNil)
				assert.Loosely(t, test.GetAutotest().TestArgs, should.Equal("foo=bar baz=qux waldo=fred"))
			})
		})
	})
}

// TODO(b/200273237): Remove test case once experiment has concluded.
func TestParamsTestArgsResultdbSettings(t *testing.T) {
	ftt.Run("Given a request that specifies resultdb_settings as an additional test arg", t, func(t *ftt.Test) {
		ctx := context.Background()
		inv := basicInvocation()
		setTestArgs(inv, "foo=bar baz=qux")
		var params test_platform.Request_Params
		setParamsTestArgs(&params, "resultdb_settings", "abc")
		t.Run("when generating a test runner request", func(t *ftt.Test) {
			g := Generator{
				Invocation: inv,
				Params:     &params,
			}
			got, err := g.testRunnerRequest(ctx)
			assert.Loosely(t, err, should.BeNil)
			test := defaultTest(t, got.Tests)
			t.Run("the resultdb_settings are only propogated when the experiment is on.", func(t *ftt.Test) {
				assert.Loosely(t, test, should.NotBeNil)
				assert.Loosely(t, test.GetAutotest(), should.NotBeNil)
				assert.Loosely(t, test.GetAutotest().TestArgs, should.Equal("foo=bar baz=qux"))
				g.Experiments = append(g.Experiments, "chromeos.cros_test_platform.add_resultdb_settings")
				got, err = g.testRunnerRequest(ctx)
				test = defaultTest(t, got.Tests)
				assert.Loosely(t, test.GetAutotest().TestArgs, should.Equal("foo=bar baz=qux resultdb_settings=abc"))
			})
		})
	})
}

func TestTestLevelKeyval(t *testing.T) {
	ftt.Run("Given a keyval inside the test invocation", t, func(t *ftt.Test) {
		ctx := context.Background()
		inv := basicInvocation()
		setTestKeyval(inv, "key", "test-value")
		t.Run("when generating a test runner request", func(t *ftt.Test) {
			g := Generator{
				Invocation: inv,
				Params:     &test_platform.Request_Params{},
			}
			got, err := g.testRunnerRequest(ctx)
			assert.Loosely(t, err, should.BeNil)
			test := defaultTest(t, got.Tests)
			t.Run("the keyval is propagated.", func(t *ftt.Test) {
				assert.Loosely(t, test, should.NotBeNil)
				assert.Loosely(t, test.GetAutotest(), should.NotBeNil)
				assert.Loosely(t, test.GetAutotest().Keyvals, should.NotBeNil)
				assert.Loosely(t, test.GetAutotest().Keyvals["key"], should.Equal("test-value"))
			})
		})
	})
}

func TestRequestLevelKeyval(t *testing.T) {
	ftt.Run("Given request-wide keyval", t, func(t *ftt.Test) {
		ctx := context.Background()
		inv := basicInvocation()
		setTestKeyval(inv, "key", "test-value")
		t.Run("when generating a test runner request", func(t *ftt.Test) {
			g := Generator{
				Invocation: inv,
				Params:     &test_platform.Request_Params{},
			}
			got, err := g.testRunnerRequest(ctx)
			assert.Loosely(t, err, should.BeNil)
			test := defaultTest(t, got.Tests)
			t.Run("the keyval is propagated.", func(t *ftt.Test) {
				assert.Loosely(t, test, should.NotBeNil)
				assert.Loosely(t, test.GetAutotest(), should.NotBeNil)
				assert.Loosely(t, test.GetAutotest().Keyvals, should.NotBeNil)
				assert.Loosely(t, test.GetAutotest().Keyvals["key"], should.Equal("test-value"))
			})
		})
	})
}

func TestKeyvalOverride(t *testing.T) {
	ftt.Run("Given keyvals with the same key in the invocation and request", t, func(t *ftt.Test) {
		ctx := context.Background()
		inv := basicInvocation()
		setTestKeyval(inv, "ambiguous-key", "test-value")
		var params test_platform.Request_Params
		setRequestKeyval(&params, "ambiguous-key", "request-value")
		t.Run("when generating a test runner request", func(t *ftt.Test) {
			g := Generator{
				Invocation: inv,
				Params:     &params,
			}
			got, err := g.testRunnerRequest(ctx)
			assert.Loosely(t, err, should.BeNil)
			test := defaultTest(t, got.Tests)
			t.Run("the keyval from the request takes precedence.", func(t *ftt.Test) {
				assert.Loosely(t, test, should.NotBeNil)
				assert.Loosely(t, test.GetAutotest(), should.NotBeNil)
				assert.Loosely(t, test.GetAutotest().Keyvals, should.NotBeNil)
				assert.Loosely(t, test.GetAutotest().Keyvals["ambiguous-key"], should.Equal("request-value"))
			})
		})
	})
}

func TestConstructedDisplayName(t *testing.T) {
	ftt.Run("Given a request does not specify a display name", t, func(t *ftt.Test) {
		ctx := context.Background()
		inv := basicInvocation()
		setTestName(inv, "foo-name")
		var params test_platform.Request_Params
		setBuild(&params, "foo-build")
		setRequestKeyval(&params, "suite", "foo-suite")
		t.Run("when generating a test runner request", func(t *ftt.Test) {
			g := Generator{
				Invocation: inv,
				Params:     &params,
			}
			got, err := g.testRunnerRequest(ctx)
			assert.Loosely(t, err, should.BeNil)
			test := defaultTest(t, got.Tests)
			t.Run("the display name is generated correctly.", func(t *ftt.Test) {
				assert.Loosely(t, test, should.NotBeNil)
				assert.Loosely(t, test.GetAutotest(), should.NotBeNil)
				assert.Loosely(t, test.GetAutotest().DisplayName, should.Equal("foo-build/foo-suite/foo-name"))
				assert.Loosely(t, test.GetAutotest().Keyvals, should.NotBeNil)
				assert.Loosely(t, test.GetAutotest().Keyvals["label"], should.Equal("foo-build/foo-suite/foo-name"))
			})
		})
	})
}

func TestExplicitDisplayName(t *testing.T) {
	ftt.Run("Given a request that specifies a display name", t, func(t *ftt.Test) {
		ctx := context.Background()
		inv := basicInvocation()
		setTestName(inv, "basic-name")
		setDisplayName(inv, "fancy-name")
		t.Run("when generating a test runner request", func(t *ftt.Test) {
			g := Generator{
				Invocation: inv,
				Params:     &test_platform.Request_Params{},
			}
			got, err := g.testRunnerRequest(ctx)
			assert.Loosely(t, err, should.BeNil)
			test := defaultTest(t, got.Tests)
			t.Run("the display name is propagated correctly.", func(t *ftt.Test) {
				assert.Loosely(t, test, should.NotBeNil)
				assert.Loosely(t, test.GetAutotest(), should.NotBeNil)
				assert.Loosely(t, test.GetAutotest().DisplayName, should.Equal("fancy-name"))
				assert.Loosely(t, test.GetAutotest().Keyvals, should.NotBeNil)
				assert.Loosely(t, test.GetAutotest().Keyvals["label"], should.Equal("fancy-name"))
			})
		})
	})
}

func TestParentIDKeyval(t *testing.T) {
	ftt.Run("Given parent task ID", t, func(t *ftt.Test) {
		ctx := context.Background()
		t.Run("when generating a test runner request", func(t *ftt.Test) {
			g := Generator{
				Invocation:   basicInvocation(),
				Params:       &test_platform.Request_Params{},
				ParentTaskID: "foo-id",
			}
			got, err := g.testRunnerRequest(ctx)
			assert.Loosely(t, err, should.BeNil)
			test := defaultTest(t, got.Tests)
			t.Run("the corresponding keyval is populated.", func(t *ftt.Test) {
				assert.Loosely(t, test, should.NotBeNil)
				assert.Loosely(t, test.GetAutotest(), should.NotBeNil)
				assert.Loosely(t, test.GetAutotest().Keyvals, should.NotBeNil)
				assert.Loosely(t, test.GetAutotest().Keyvals["parent_job_id"], should.Equal("foo-id"))
			})
		})
	})
}

func TestBuildKeyval(t *testing.T) {
	ftt.Run("Given a build", t, func(t *ftt.Test) {
		ctx := context.Background()
		var params test_platform.Request_Params
		setBuild(&params, "foo-build")
		t.Run("when generating a test runner request", func(t *ftt.Test) {
			g := Generator{
				Invocation: basicInvocation(),
				Params:     &params,
			}
			got, err := g.testRunnerRequest(ctx)
			assert.Loosely(t, err, should.BeNil)
			test := defaultTest(t, got.Tests)
			t.Run("the corresponding keyval is populated.", func(t *ftt.Test) {
				assert.Loosely(t, test, should.NotBeNil)
				assert.Loosely(t, test.GetAutotest(), should.NotBeNil)
				assert.Loosely(t, test.GetAutotest().Keyvals, should.NotBeNil)
				assert.Loosely(t, test.GetAutotest().Keyvals["build"], should.Equal("foo-build"))
			})
		})
	})
}

func TestDeadline(t *testing.T) {
	ftt.Run("Given a request that specifies a deadline", t, func(t *ftt.Test) {
		ctx := context.Background()
		ts, _ := time.Parse(time.RFC3339, "2020-02-27T12:47:42Z")
		t.Run("when generating a test runner request", func(t *ftt.Test) {
			g := Generator{
				Invocation: basicInvocation(),
				Params:     &test_platform.Request_Params{},
				Deadline:   ts,
			}
			got, err := g.testRunnerRequest(ctx)
			assert.Loosely(t, err, should.BeNil)
			t.Run("the deadline is set correctly.", func(t *ftt.Test) {
				assert.Loosely(t, ptypes.TimestampString(got.Deadline), should.Equal("2020-02-27T12:47:42Z"))
			})
		})
	})
}

func TestNoDeadline(t *testing.T) {
	ftt.Run("Given a request that does not specify a deadline", t, func(t *ftt.Test) {
		ctx := context.Background()
		t.Run("when generating a test runner request", func(t *ftt.Test) {
			g := Generator{
				Invocation: basicInvocation(),
				Params:     &test_platform.Request_Params{},
			}
			got, err := g.testRunnerRequest(ctx)
			assert.Loosely(t, err, should.BeNil)
			t.Run("the deadline should not be set.", func(t *ftt.Test) {
				assert.Loosely(t, got.Deadline, should.BeNil)
			})
		})
	})
}

func TestParentUID(t *testing.T) {
	ftt.Run("Given a parent UID", t, func(t *ftt.Test) {
		ctx := context.Background()
		t.Run("when generating a test runner request", func(t *ftt.Test) {
			g := Generator{
				Invocation:       basicInvocation(),
				Params:           &test_platform.Request_Params{},
				ParentTaskID:     "foo-id",
				ParentRequestUID: "foo-uid",
			}
			got, err := g.testRunnerRequest(ctx)
			assert.Loosely(t, err, should.BeNil)
			t.Run("the parent UID is propagated correctly.", func(t *ftt.Test) {
				assert.Loosely(t, got.ParentRequestUid, should.Equal("foo-uid"))
			})
		})
	})
}

func TestParentBuildID(t *testing.T) {
	ftt.Run("Given a parent UID", t, func(t *ftt.Test) {
		ctx := context.Background()
		t.Run("when generating a test runner request", func(t *ftt.Test) {
			g := Generator{
				Invocation:    basicInvocation(),
				Params:        &test_platform.Request_Params{},
				ParentBuildID: 43,
			}
			got, err := g.testRunnerRequest(ctx)
			assert.Loosely(t, err, should.BeNil)
			t.Run("the parent build ID is propagated correctly.", func(t *ftt.Test) {
				assert.Loosely(t, got.ParentBuildId, should.Equal(43))
			})
		})
	})
}
