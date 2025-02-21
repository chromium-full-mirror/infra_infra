// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bootstrap

import (
	"errors"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	. "go.chromium.org/infra/chromium/util"
)

func TestMultierror(t *testing.T) {
	t.Parallel()

	ftt.Run("multierror", t, func(t *ftt.Test) {

		t.Run("reports a contained error", func(t *ftt.Test) {
			m := &multierror{[]error{
				errors.New("foo error"),
			}}

			assert.Loosely(t, m, should.ErrLike("1 error occurred"))
			assert.Loosely(t, m, should.ErrLike("foo error"))
		})

		t.Run("reports all contained errors", func(t *ftt.Test) {
			m := &multierror{[]error{
				errors.New("foo error"),
				errors.New("bar error"),
				errors.New("baz error"),
			}}

			assert.Loosely(t, m, should.ErrLike("3 errors occurred"))
			assert.Loosely(t, m, should.ErrLike("foo error"))
			assert.Loosely(t, m, should.ErrLike("bar error"))
			assert.Loosely(t, m, should.ErrLike("baz error"))
		})

	})
}

type fakeValidatable struct {
	fn func(v *validator)
}

func (f *fakeValidatable) validate(v *validator) {
	if f.fn != nil {
		f.fn(v)
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	ftt.Run("validate", t, func(t *ftt.Test) {

		t.Run("calls validate on the validatable", func(t *ftt.Test) {
			called := false
			x := &fakeValidatable{func(v *validator) {
				called = true
			}}

			err := validate(x, "$test")

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, called, should.BeTrue)
		})

		t.Run("returns error if validator.errorf is called", func(t *ftt.Test) {

			t.Run("with ${} in format string replaced with validation context", func(t *ftt.Test) {
				x := &fakeValidatable{func(v *validator) {
					v.errorf("failure to validate ${}")
				}}

				err := validate(x, "$test")

				assert.Loosely(t, err, should.ErrLike("failure to validate $test"))

			})

			t.Run("with ${} in format arguments not replace with validation context", func(t *ftt.Test) {
				x := &fakeValidatable{func(v *validator) {
					v.errorf("failure to validate %s", "${}")
				}}

				err := validate(x, "$test")

				assert.Loosely(t, err, should.ErrLike("failure to validate ${}"))
			})

			t.Run("with ${} in format string replaced with updated validation context in nested validate call", func(t *ftt.Test) {
				x := &fakeValidatable{func(v *validator) {
					v.errorf("failure to validate ${}")
				}}
				y := &fakeValidatable{func(v *validator) {
					v.validate(x, "x")
				}}

				err := validate(y, "$test")

				assert.Loosely(t, err, should.ErrLike("failure to validate $test.x"))
			})

		})

	})
}

func createBootstrapPropertiesProperties(propsJson []byte) *BootstrapPropertiesProperties {
	props := &BootstrapPropertiesProperties{}
	PanicOnError(protojson.Unmarshal(propsJson, props))
	return props
}

func TestBootstrapPropertiesPropertiesValidation(t *testing.T) {
	t.Parallel()

	ftt.Run("validate", t, func(t *ftt.Test) {

		t.Run("fails for unset required top-level fields", func(t *ftt.Test) {
			props := createBootstrapPropertiesProperties([]byte("{}"))

			err := validate(props, "$test")

			assert.Loosely(t, err, should.ErrLike("none of the config_project fields in $test is set"))
			assert.Loosely(t, err, should.ErrLike("$test.properties_file is not set"))
		})

		t.Run("with a top level project", func(t *ftt.Test) {

			t.Run("fails for unset required fields in top_level_project", func(t *ftt.Test) {
				props := createBootstrapPropertiesProperties([]byte(`{
					"top_level_project": {}
				}`))

				err := validate(props, "$test")

				assert.Loosely(t, err, should.ErrLike("$test.top_level_project.repo is not set"))
				assert.Loosely(t, err, should.ErrLike("$test.top_level_project.ref is not set"))
			})

			t.Run("fails for unset required fields in top_level_project.repo", func(t *ftt.Test) {
				props := createBootstrapPropertiesProperties([]byte(`{
						"top_level_project": {
							"repo": {}
						}
					}`))

				err := validate(props, "$test")

				assert.Loosely(t, err, should.ErrLike("$test.top_level_project.repo.host is not set"))
				assert.Loosely(t, err, should.ErrLike("$test.top_level_project.repo.project is not set"))
			})

			t.Run("succeeds for valid properties", func(t *ftt.Test) {
				props := createBootstrapPropertiesProperties([]byte(`{
						"top_level_project": {
							"repo": {
								"host": "chromium.googlesource.com",
								"project": "top/level"
							},
							"ref": "refs/heads/top-level"
						},
						"properties_file": "infra/config/bucket/builder/properties.json"
					}`))

				err := validate(props, "$test")

				assert.Loosely(t, err, should.BeNil)
			})
		})

		t.Run("with a dependency project", func(t *ftt.Test) {

			t.Run("fails for unset required fields in dependency_project", func(t *ftt.Test) {
				props := createBootstrapPropertiesProperties([]byte(`{
						"dependency_project": {}
					}`))

				err := validate(props, "$test")

				assert.Loosely(t, err, should.ErrLike("$test.dependency_project.top_level_repo is not set"))
				assert.Loosely(t, err, should.ErrLike("$test.dependency_project.top_level_ref is not set"))
				assert.Loosely(t, err, should.ErrLike("$test.dependency_project.config_repo is not set"))
				assert.Loosely(t, err, should.ErrLike(
					"none of the config_repo_locator fields in $test.dependency_project is set"))
			})

			t.Run("fails for unset required fields in dependency_project.top_level_repo", func(t *ftt.Test) {
				props := createBootstrapPropertiesProperties([]byte(`{
						"dependency_project": {
							"top_level_repo": {}
						}
					}`))

				err := validate(props, "$test")

				assert.Loosely(t, err, should.ErrLike("$test.dependency_project.top_level_repo.host is not set"))
				assert.Loosely(t, err, should.ErrLike("$test.dependency_project.top_level_repo.project is not set"))
			})

			t.Run("fails for unset required fields in dependency_project.config_repo", func(t *ftt.Test) {
				props := createBootstrapPropertiesProperties([]byte(`{
						"dependency_project": {
							"config_repo": {}
						}
					}`))

				err := validate(props, "$test")

				assert.Loosely(t, err, should.ErrLike("$test.dependency_project.config_repo.host is not set"))
				assert.Loosely(t, err, should.ErrLike("$test.dependency_project.config_repo.project is not set"))
			})

			t.Run("succeeds for valid properties with config_repo_submodule_path", func(t *ftt.Test) {
				props := createBootstrapPropertiesProperties([]byte(`{
						"dependency_project": {
							"top_level_repo": {
								"host": "chromium.googlesource.com",
								"project": "top/level"
							},
							"top_level_ref": "refs/heads/top-level",
							"config_repo": {
								"host": "chromium.googlesource.com",
								"project": "dependency"
							},
							"config_repo_path": "path/to/dependency"
						},
						"properties_file": "infra/config/generated/builders/bucket/builder/properties.json"
					}`))

				err := validate(props, "$test")

				assert.Loosely(t, err, should.BeNil)
			})

			t.Run("succeeds for valid properties with config_repo_path", func(t *ftt.Test) {
				props := createBootstrapPropertiesProperties([]byte(`{
						"dependency_project": {
							"top_level_repo": {
								"host": "chromium.googlesource.com",
								"project": "top/level"
							},
							"top_level_ref": "refs/heads/top-level",
							"config_repo": {
								"host": "chromium.googlesource.com",
								"project": "dependency"
							},
							"config_repo_submodule_path": "submodule/path"
						},
						"properties_file": "infra/config/generated/builders/bucket/builder/properties.json"
					}`))

				err := validate(props, "$test")

				assert.Loosely(t, err, should.BeNil)
			})

		})

	})
}

func createBootstrapExeProperties(propsJson []byte) *BootstrapExeProperties {
	props := &BootstrapExeProperties{}
	PanicOnError(protojson.Unmarshal(propsJson, props))
	return props
}

func TestBootstrapExePropertiesValidation(t *testing.T) {
	t.Parallel()

	ftt.Run("validate", t, func(t *ftt.Test) {

		t.Run("fails for unset required top-level fields", func(t *ftt.Test) {
			props := createBootstrapExeProperties([]byte("{}"))

			err := validate(props, "$test")

			assert.Loosely(t, err, should.ErrLike("$test.exe is not set"))
		})

		t.Run("fails for unset required fields in exe", func(t *ftt.Test) {
			props := createBootstrapExeProperties([]byte(`{
				"exe": {}
			}`))

			err := validate(props, "$test")

			assert.Loosely(t, err, should.ErrLike("$test.exe.cipd_package is not set"))
			assert.Loosely(t, err, should.ErrLike("$test.exe.cipd_version is not set"))
			assert.Loosely(t, err, should.ErrLike("$test.exe.cmd is not set"))
		})

		t.Run("succeeds for valid properties", func(t *ftt.Test) {
			props := createBootstrapExeProperties([]byte(`{
				"exe": {
					"cipd_package": "fake-package",
					"cipd_version": "fake-version",
					"cmd": ["fake-cmd"]
				}
			}`))

			err := validate(props, "$test")

			assert.Loosely(t, err, should.BeNil)
		})

	})
}

func createBootstrapTriggerProperties(propsJson []byte) *BootstrapTriggerProperties {
	props := &BootstrapTriggerProperties{}
	PanicOnError(protojson.Unmarshal(propsJson, props))
	return props
}

func TestBootstrapTriggerPropertiesValidation(t *testing.T) {
	t.Parallel()

	ftt.Run("validate", t, func(t *ftt.Test) {

		t.Run("fails for unset required fields in commits", func(t *ftt.Test) {
			props := createBootstrapTriggerProperties([]byte(`{
				"commits": [
					{
						"project": "fake-project1",
						"ref": "fake-ref1"
					},
					{
						"host": "fake-host2",
						"ref": "fake-ref2"
					},
					{
						"host": "fake-host3",
						"project": "fake-project3"
					}
				]
			}`))

			err := validate(props, "$test")

			assert.Loosely(t, err, should.ErrLike("$test.commits[0].host is not set"))
			assert.Loosely(t, err, should.ErrLike("$test.commits[1].project is not set"))
			assert.Loosely(t, err, should.ErrLike("$test.commits[2] has neither ref nor id set"))
		})

		t.Run("succeeds for valid properties", func(t *ftt.Test) {
			props := createBootstrapTriggerProperties([]byte(`{
				"commits": [
					{
						"host": "fake-host1",
						"project": "fake-project1",
						"ref": "fake-ref1"
					},
					{
						"host": "fake-host2",
						"project": "fake-project2",
						"ref": "fake-ref2"
					},
					{
						"host": "fake-host3",
						"project": "fake-project3",
						"ref": "fake-ref3"
					},
					{
						"host": "fake-host4",
						"project": "fake-project4",
						"id": "fake-revision4"
					}
				]
			}`))

			err := validate(props, "$test")

			assert.Loosely(t, err, should.BeNil)
		})

	})
}
