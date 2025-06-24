// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"

	"go.chromium.org/luci/auth/integration/authtest"
	"go.chromium.org/luci/auth/integration/localauth"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	log "go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/common/system/environ"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/lucictx"

	"go.chromium.org/infra/tools/kitchen/third_party/recipe_engine"
)

func TestCook(t *testing.T) {
	// TODO(crbug.com/904533): Running tests that use git in parallel may be
	// causing issues on Windows.
	//
	// t.Parallel()

	ftt.Run("cook", t, func(t *ftt.Test) {
		cook := cmdCook.CommandRun().(*cookRun)

		t.Run("updateEnv", func(t *ftt.Test) {
			tdir, err := os.MkdirTemp("", "kitchen-test-")
			assert.Loosely(t, err, should.BeNil)
			defer os.RemoveAll(tdir)

			cook.TempDir = tdir
			expected := filepath.Join(tdir, "t")

			env := environ.New(nil)
			assert.Loosely(t, cook.updateEnv(env), should.BeNil)
			assert.Loosely(t, env.Map(), should.Resemble(map[string]string{
				"TEMPDIR":             expected,
				"TMPDIR":              expected,
				"TEMP":                expected,
				"TMP":                 expected,
				"MAC_CHROMIUM_TMPDIR": expected,
			}))
		})

		t.Run("run", func(t *ftt.Test) {
			// Setup context.
			c := context.Background()
			cfg := gologger.LoggerConfig{
				Format: "[%{level:.0s} %{time:2006-01-02 15:04:05}] %{message}",
				Out:    os.Stderr,
			}
			c = cfg.Use(c)
			logCfg := log.Config{
				Level: log.Info,
			}
			c = logCfg.Set(c)

			// Setup fake auth.
			fakeAuth := localauth.Server{
				TokenGenerators: map[string]localauth.TokenGenerator{
					"recipe_acc": &authtest.FakeTokenGenerator{
						Email: "recipe@example.com",
					},
					"system_acc": &authtest.FakeTokenGenerator{
						Email: "system@example.com",
					},
				},
				DefaultAccountID: "recipe_acc",
			}
			la, err := fakeAuth.Start(c)
			assert.Loosely(t, err, should.BeNil)
			defer fakeAuth.Stop(c)
			c = lucictx.SetLocalAuth(c, la)

			// Setup tempdir.
			tdir, err := os.MkdirTemp("", "kitchen-test-")
			assert.Loosely(t, err, should.BeNil)
			defer os.RemoveAll(tdir)

			// OS X has symlinks in its TempDir return values by default.
			tdir, err = filepath.EvalSymlinks(tdir)
			assert.Loosely(t, err, should.BeNil)

			// Prepare paths
			recipeRepoDir := filepath.Join(tdir, "recipe_repo")
			resultFilePath := filepath.Join(tdir, "result.json")
			workdirPath := filepath.Join(tdir, "k")
			kitchenTempDir := filepath.Join(tdir, "tmp")
			cacheDirPath := filepath.Join(tdir, "cache-dir")

			run := func(mockRecipeResult *recipe_engine.Result, recipeExitCode int, withResultDBContext bool) (*buildbucketpb.Build, int) {
				// Prepare recipe dir.
				assert.Loosely(t, setupRecipeRepo(c, recipeRepoDir), should.BeNil)

				// Kitchen works relative to its cwd
				cwd, err := os.Getwd()
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, os.Chdir(tdir), should.BeNil)
				defer os.Chdir(cwd)

				// Set lucictx.
				ctx := c
				if withResultDBContext {
					ctx = lucictx.SetResultDB(c, &lucictx.ResultDB{
						Hostname: "test.results.cr.dev",
						CurrentInvocation: &lucictx.ResultDBInvocation{
							Name:        "invocations/build:1",
							UpdateToken: "UpdateToken",
						},
					})
				}

				// Mock recipes.py result
				mockedRecipeResultPath := filepath.Join(tdir, "expected_result.json")
				m := jsonpb.Marshaler{}
				f, err := os.Create(mockedRecipeResultPath)
				assert.Loosely(t, err, should.BeNil)
				defer f.Close()
				err = m.Marshal(f, mockRecipeResult)
				assert.Loosely(t, err, should.BeNil)

				// Prepare arguments
				recipeInputPath := filepath.Join(tdir, "recipe_input.json")
				resultDBProperty := `
					"resultdb": {
						"invocation": "invocations/build:1",
						"hostname":   "test.results.cr.dev"
					}
				`
				if withResultDBContext {
					resultDBProperty = ""
				}
				propertiesJSON := fmt.Sprintf(`{
					"recipe_mock_cfg": {
						"input_path": %s,
						"exitCode": %d,
						"mocked_result_path": %s
					},
					"$recipe_engine/buildbucket": {
						"build": {
							"infra": {
								%s
							}
						}
					},
					"$kitchen": {
						"git_auth": true,
						"emulate_gce": true
					}
				}`, strconv.Quote(recipeInputPath), recipeExitCode, strconv.Quote(mockedRecipeResultPath), resultDBProperty)
				args := []string{
					"-recipe", "kitchen_test",
					"-properties", string(propertiesJSON),
					"-checkout-dir", recipeRepoDir,
					"-temp-dir", kitchenTempDir,
					"-cache-dir", cacheDirPath,
					"-logdog-annotation-url", "logdog://logdog.example.com/chromium/prefix/+/annotations",
					"-logdog-null-output",
					"-output-result-json", resultFilePath,
					"-recipe-result-byte-limit", "500000",
					"-luci-system-account", "system_acc",
				}

				env := environ.System()
				env.Set("SWARMING_TASK_ID", "task")
				env.Set("SWARMING_BOT_ID", "bot")

				// Cook.
				err = cook.Flags.Parse(args)
				assert.Loosely(t, err, should.BeNil)
				result, outputExitCode := cook.run(ctx, nil, env)

				// Log results
				t.Logf("cook result:\n%s\n", proto.MarshalTextString(result))

				// Check parsed kitchen own properties.
				assert.Loosely(t, cook.kitchenProps, should.Resemble(&kitchenProperties{
					GitAuth:      true,
					EmulateGCE:   true,
					DockerAuth:   true,
					FirebaseAuth: false,
				}))

				// Check recipes.py input.
				recipeInputFile, err := os.ReadFile(recipeInputPath)
				assert.Loosely(t, err, should.BeNil)
				type recipeInput struct {
					Args       []string
					Properties map[string]any
				}
				var actualRecipeInput recipeInput
				err = json.Unmarshal(recipeInputFile, &actualRecipeInput)
				assert.Loosely(t, err, should.BeNil)
				expectedInputProperties := map[string]any{
					"bot_id":      "bot",
					"path_config": "generic",
					"$recipe_engine/path": map[string]any{
						"cache_dir": cacheDirPath,
						"temp_dir":  filepath.Join(kitchenTempDir, "rt"),
					},
					"$recipe_engine/buildbucket": map[string]any{
						"build": map[string]any{
							"infra": map[string]any{
								"resultdb": map[string]any{
									"hostname":   "test.results.cr.dev",
									"invocation": "invocations/build:1",
								},
							},
						},
					},
				}
				assert.Loosely(t, actualRecipeInput, should.Resemble(recipeInput{
					Args: []string{
						filepath.Join(recipeRepoDir, "recipes"),
						"run",
						"--properties-file", filepath.Join(kitchenTempDir, "rr", "properties.json"),
						"--workdir", workdirPath,
						"--output-result-json", filepath.Join(kitchenTempDir, "recipe-result.json"),
						"kitchen_test",
					},
					Properties: expectedInputProperties,
				}))

				return result, outputExitCode
			}

			t.Run("recipe success", func(t *ftt.Test) {
				recipeResult := &recipe_engine.Result{
					OneofResult: &recipe_engine.Result_JsonResult{
						JsonResult: `{"foo": "bar"}`,
					},
				}
				result, exitCode := run(recipeResult, 0, false)
				assert.Loosely(t, exitCode, should.BeZero)
				assert.Loosely(t, result.Status, should.Equal(buildbucketpb.Status_SUCCESS))
			})
			t.Run("recipe step failed", func(t *ftt.Test) {
				recipeResult := &recipe_engine.Result{
					OneofResult: &recipe_engine.Result_Failure{
						Failure: &recipe_engine.Failure{
							HumanReason: "step failed",
							FailureType: &recipe_engine.Failure_Failure{
								Failure: &recipe_engine.StepFailure{
									Step: "bot_update",
								},
							},
						},
					},
				}
				result, exitCode := run(recipeResult, 1, false)
				assert.Loosely(t, exitCode, should.Equal(1))
				assert.Loosely(t, result.Status, should.Equal(buildbucketpb.Status_FAILURE))
				assert.Loosely(t, result.SummaryMarkdown, should.Equal(recipeResult.GetFailure().HumanReason))
			})
			t.Run("long summary markdown", func(t *ftt.Test) {
				recipeResult := &recipe_engine.Result{
					OneofResult: &recipe_engine.Result_Failure{
						Failure: &recipe_engine.Failure{
							HumanReason: strings.Repeat("step failed ", 600),
							FailureType: &recipe_engine.Failure_Failure{
								Failure: &recipe_engine.StepFailure{
									Step: "bot_update",
								},
							},
						},
					},
				}
				result, exitCode := run(recipeResult, 1, false)
				assert.Loosely(t, exitCode, should.Equal(1))
				assert.Loosely(t, result.Status, should.Equal(buildbucketpb.Status_FAILURE))
				expectedSummary := recipeResult.GetFailure().HumanReason[:maxSummaryLength-3] + "..."
				assert.Loosely(t, result.SummaryMarkdown, should.Equal(expectedSummary))
			})
			t.Run("recipe success with resultdb context", func(t *ftt.Test) {
				recipeResult := &recipe_engine.Result{
					OneofResult: &recipe_engine.Result_JsonResult{
						JsonResult: `{"foo": "bar"}`,
					},
				}
				result, exitCode := run(recipeResult, 0, true)
				assert.Loosely(t, exitCode, should.BeZero)
				assert.Loosely(t, result.Status, should.Equal(buildbucketpb.Status_SUCCESS))
			})
		})
	})
}

func setupRecipeRepo(c context.Context, targetDir string) error {
	if err := copyDir(targetDir, filepath.Join("testdata", "recipe_repo")); err != nil {
		return err
	}
	return nil
}

func copyDir(dest, src string) error {
	return filepath.Walk(src, func(srcPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if filepath.Base(srcPath) == ".recipe_deps" {
			return filepath.SkipDir
		}
		relPath, err := filepath.Rel(src, srcPath)
		if err != nil {
			return err
		}
		destPath := filepath.Join(dest, relPath)
		if info.IsDir() {
			return os.Mkdir(destPath, 0700)
		}

		data, err := os.ReadFile(srcPath)
		if err != nil {
			return err
		}
		return os.WriteFile(destPath, data, info.Mode())
	})
}
