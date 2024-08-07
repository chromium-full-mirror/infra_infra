// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	. "go.chromium.org/luci/common/testing/assertions"

	"infra/chromium/bootstrapper/bootstrap"
	"infra/chromium/bootstrapper/clients/cipd"
	fakecipd "infra/chromium/bootstrapper/clients/fakes/cipd"
	fakegitiles "infra/chromium/bootstrapper/clients/fakes/gitiles"
	"infra/chromium/bootstrapper/clients/gitiles"
	. "infra/chromium/util"
)

func createInput(buildJson string) io.Reader {
	build := &buildbucketpb.Build{}
	PanicOnError(protojson.Unmarshal([]byte(buildJson), build))
	buildProtoBytes, err := proto.Marshal(build)
	PanicOnError(err)
	return bytes.NewReader(buildProtoBytes)
}

type reader struct {
	readFn func([]byte) (int, error)
}

func (r reader) Read(p []byte) (n int, err error) {
	return r.readFn(p)
}

func TestPerformBootstrap(t *testing.T) {
	t.Parallel()

	project := &fakegitiles.Project{
		Refs:      map[string]string{},
		Revisions: map[string]*fakegitiles.Revision{},
	}

	pkg := &fakecipd.Package{
		Refs:      map[string]string{},
		Instances: map[string]*fakecipd.PackageInstance{},
	}

	fakePackagesRoot := filepath.Join(t.TempDir(), "fake-packages-root")

	opts := options{
		outputPath:   "fake-output-path",
		packagesRoot: fakePackagesRoot,
	}

	ctx := context.Background()

	ctx = gitiles.UseGitilesClientFactory(ctx, fakegitiles.Factory(map[string]*fakegitiles.Host{
		"fake-host": {
			Projects: map[string]*fakegitiles.Project{
				"fake-project": project,
			},
		},
	}))

	ctx = cipd.UseClientFactory(ctx, fakecipd.Factory(map[string]*fakecipd.Package{
		"fake-package": pkg,
	}))

	Convey("performBootstrap", t, func() {

		Convey("fails if reading input fails", func() {
			input := reader{func(p []byte) (int, error) {
				return 0, errors.New("test read failure")
			}}

			cmd, exeInput, err := performBootstrap(ctx, input, opts)

			So(err, ShouldNotBeNil)
			So(cmd, ShouldBeNil)
			So(exeInput, ShouldBeNil)
		})

		Convey("fails if unmarshalling build fails", func() {
			input := strings.NewReader("invalid-proto")

			cmd, exeInput, err := performBootstrap(ctx, input, opts)

			So(err, ShouldNotBeNil)
			So(cmd, ShouldBeNil)
			So(exeInput, ShouldBeNil)
		})

		Convey("fails if bootstrap fails", func() {
			input := createInput(`{}`)

			cmd, exeInput, err := performBootstrap(ctx, input, opts)

			So(err, ShouldNotBeNil)
			So(cmd, ShouldBeNil)
			So(exeInput, ShouldBeNil)
		})

		input := createInput(`{
			"input": {
				"properties": {
					"$bootstrap/properties": {
						"top_level_project": {
							"repo": {
								"host": "fake-host",
								"project": "fake-project"
							},
							"ref": "fake-ref"
						},
						"properties_file": "fake-properties-file"
					},
					"$bootstrap/exe": {
						"exe": {
							"cipd_package": "fake-package",
							"cipd_version": "fake-version",
							"cmd": ["fake-exe"]
						}
					},
					"foo": "build-value"
				}
			}
		}`)

		Convey("fails if determining executable fails", func() {
			project.Refs["fake-ref"] = "fake-revision"
			project.Revisions["fake-revision"] = &fakegitiles.Revision{
				Files: map[string]*fakegitiles.PathObject{
					"fake-properties-file": fakegitiles.File(`{
						"foo": "bar"
					}`),
				},
			}
			pkg.Refs["fake-version"] = ""

			cmd, exeInput, err := performBootstrap(ctx, input, opts)

			So(err, ShouldNotBeNil)
			So(cmd, ShouldBeNil)
			So(exeInput, ShouldBeNil)
		})

		Convey("succeeds for valid input", func() {
			project.Refs["fake-ref"] = "fake-revision"
			project.Revisions["fake-revision"] = &fakegitiles.Revision{
				Files: map[string]*fakegitiles.PathObject{
					"fake-properties-file": fakegitiles.File(`{
						"foo": "builder-value"
					}`),
				},
			}
			pkg.Refs["fake-version"] = "fake-instance-id"

			cmd, exeInput, err := performBootstrap(ctx, input, opts)

			So(err, ShouldBeNil)
			So(cmd, ShouldResemble, []string{
				filepath.Join(opts.packagesRoot, "cipd", "exe", "fake-exe"),
				"--output",
				opts.outputPath,
			})
			build := &buildbucketpb.Build{}
			proto.Unmarshal(exeInput, build)
			So(build, ShouldResembleProtoJSON, `{
				"input": {
					"gitiles_commit": {
						"host": "fake-host",
						"project": "fake-project",
						"ref": "fake-ref",
						"id": "fake-revision"
					},
					"properties": {
						"$build/chromium_bootstrap": {
							"commits": [
								{
									"host": "fake-host",
									"project": "fake-project",
									"ref": "fake-ref",
									"id": "fake-revision"
								}
							],
							"exe": {
								"cipd": {
									"server": "https://chrome-infra-packages.appspot.com",
									"package": "fake-package",
									"requested_version": "fake-version",
									"actual_version": "fake-instance-id"
								},
								"cmd": ["fake-exe"]
							},
							"config_source": {
								"last_changed_commit": {
									"host": "fake-host",
									"project": "fake-project",
									"ref": "fake-ref",
									"id": "fake-revision"
								},
								"path": "fake-properties-file"
							}
					},
						"foo": "builder-value"
					}
				}
			}`)
		})

		Convey("succeeds for polymorphic with build properties prioritized over builder properties", func() {
			project.Refs["fake-ref"] = "fake-revision"
			project.Revisions["fake-revision"] = &fakegitiles.Revision{
				Files: map[string]*fakegitiles.PathObject{
					"fake-properties-file": fakegitiles.File(`{
						"foo": "builder-value"
					}`),
				},
			}
			opts.polymorphic = true

			_, exeInput, err := performBootstrap(ctx, input, opts)
			So(err, ShouldBeNil)
			build := &buildbucketpb.Build{}
			proto.Unmarshal(exeInput, build)
			So(build, ShouldResembleProtoJSON, `{
				"input": {
					"gitiles_commit": {
						"host": "fake-host",
						"project": "fake-project",
						"ref": "fake-ref",
						"id": "fake-revision"
					},
					"properties": {
						"$build/chromium_bootstrap": {
							"commits": [
								{
									"host": "fake-host",
									"project": "fake-project",
									"ref": "fake-ref",
									"id": "fake-revision"
								}
							],
							"exe": {
								"cipd": {
									"server": "https://chrome-infra-packages.appspot.com",
									"package": "fake-package",
									"requested_version": "fake-version",
									"actual_version": "fake-instance-id"
								},
								"cmd": ["fake-exe"]
							},
							"config_source": {
								"last_changed_commit": {
									"host": "fake-host",
									"project": "fake-project",
									"ref": "fake-ref",
									"id": "fake-revision"
								},
								"path": "fake-properties-file"
							}
						},
						"foo": "build-value"
					}
				}
			}`)
		})

		Convey("succeeds for properties-optional without $bootstrap/properties", func() {
			input := createInput(`{
				"input": {
					"properties": {
						"$bootstrap/exe": {
							"exe": {
								"cipd_package": "fake-package",
								"cipd_version": "fake-version",
								"cmd": ["fake-exe"]
							}
						}
					}
				}
			}`)
			opts.propertiesOptional = true

			cmd, exeInput, err := performBootstrap(ctx, input, opts)

			So(err, ShouldBeNil)
			So(cmd, ShouldResemble, []string{
				filepath.Join(opts.packagesRoot, "cipd", "exe", "fake-exe"),
				"--output",
				opts.outputPath,
			})
			build := &buildbucketpb.Build{}
			proto.Unmarshal(exeInput, build)
			So(build, ShouldResembleProtoJSON, `{
				"input": {
					"properties": {
						"$build/chromium_bootstrap": {
							"exe": {
								"cipd": {
									"server": "https://chrome-infra-packages.appspot.com",
									"package": "fake-package",
									"requested_version": "fake-version",
									"actual_version": "fake-instance-id"
								},
								"cmd": ["fake-exe"]
							}
						}
					}
				}
			}`)
		})

	})
}

func testBootstrapFn(bootstrapErr error) bootstrapFn {
	return func(ctx context.Context, input io.Reader, opts options) ([]string, []byte, error) {
		if bootstrapErr != nil {
			return nil, nil, bootstrapErr
		}
		return []string{"fake", "command"}, []byte("fake-contents"), nil
	}
}

func testExecuteCmdFn(cmdErr error) executeCmdFn {
	return func(ctx context.Context, cmd []string, input []byte) error {
		if cmdErr != nil {
			return cmdErr
		}
		return nil
	}
}

type buildUpdateRecord struct {
	build *buildbucketpb.Build
}

func testUpdateBuildFn(updateErr error) (*buildUpdateRecord, updateBuildFn) {
	update := &buildUpdateRecord{}
	return update, func(ctx context.Context, build *buildbucketpb.Build) error {
		update.build = build
		if updateErr != nil {
			return updateErr
		}
		return nil
	}
}

func TestBootstrapMain(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	Convey("bootstrapMain", t, func() {

		getOptions := func() options { return options{} }
		performBootstrap := testBootstrapFn(nil)
		execute := testExecuteCmdFn(nil)
		record, updateBuild := testUpdateBuildFn(nil)

		Convey("does not update build on success", func() {
			sleepDuration, err := bootstrapMain(ctx, getOptions, performBootstrap, execute, updateBuild)

			So(err, ShouldBeNil)
			So(sleepDuration, ShouldEqual, 0)
			So(record.build, ShouldBeNil)
		})

		Convey("does not update build on bootstrapped exe failure", func() {
			exeErr := &exec.ExitError{
				ProcessState: &os.ProcessState{},
				Stderr:       []byte("test exe failure"),
			}
			execute := testExecuteCmdFn(exeErr)

			sleepDuration, err := bootstrapMain(ctx, getOptions, performBootstrap, execute, updateBuild)

			So(err, ShouldErrLike, exeErr)
			So(sleepDuration, ShouldEqual, 0)
			So(record.build, ShouldBeNil)
		})

		Convey("updates build when failing to execute bootstrapped exe", func() {
			cmdErr := errors.New("test cmd execution failure")
			execute := testExecuteCmdFn(cmdErr)

			sleepDuration, err := bootstrapMain(ctx, getOptions, performBootstrap, execute, updateBuild)

			So(err, ShouldErrLike, cmdErr)
			So(sleepDuration, ShouldEqual, 0)
			So(record.build, ShouldResembleProtoJSON, `{
				"status": "INFRA_FAILURE",
				"summary_markdown": "<pre>test cmd execution failure</pre>"
			}`)
		})

		Convey("updates build on failure of non-bootstrapped exe process", func() {
			cmdErr := &exec.ExitError{
				ProcessState: &os.ProcessState{},
				Stderr:       []byte("test process failure"),
			}
			performBootstrap := testBootstrapFn(cmdErr)

			sleepDuration, err := bootstrapMain(ctx, getOptions, performBootstrap, execute, updateBuild)

			So(err, ShouldErrLike, cmdErr)
			So(sleepDuration, ShouldEqual, 0)
			So(record.build, ShouldResembleProtoJSON, fmt.Sprintf(`{
				"status": "INFRA_FAILURE",
				"summary_markdown": "<pre>%s</pre>"
			}`, cmdErr))
		})

		Convey("updates build for generic bootstrap failure", func() {
			bootstrapErr := errors.New("test bootstrap failure")
			performBootstrap := testBootstrapFn(bootstrapErr)

			sleepDuration, err := bootstrapMain(ctx, getOptions, performBootstrap, execute, updateBuild)

			So(err, ShouldErrLike, bootstrapErr)
			So(sleepDuration, ShouldEqual, 0)
			So(record.build, ShouldResembleProtoJSON, `{
				"status": "INFRA_FAILURE",
				"summary_markdown": "<pre>test bootstrap failure</pre>"
			}`)
		})

		Convey("updates build for patch rejected failure", func() {
			bootstrapErr := errors.New("test bootstrap failure")
			bootstrapErr = bootstrap.PatchRejected.Apply(bootstrapErr)
			performBootstrap := testBootstrapFn(bootstrapErr)

			sleepDuration, err := bootstrapMain(ctx, getOptions, performBootstrap, execute, updateBuild)

			So(err, ShouldErrLike, bootstrapErr)
			So(sleepDuration, ShouldEqual, 0)
			So(record.build, ShouldResembleProtoJSON, `{
				"status": "FAILURE",
				"summary_markdown": "<pre>Patch failure: See build stderr log. Try rebasing?</pre>",
				"output": {
					"properties": {
						"failure_type": "PATCH_FAILURE"
					}
				}
			}`)
		})

		Convey("returns sleep duration for sleep tagged error", func() {
			bootstrapErr := errors.New("test error")
			bootstrapErr = bootstrap.SleepBeforeExiting.With(20 * time.Second).Apply(bootstrapErr)
			performBootstrap := testBootstrapFn(bootstrapErr)

			sleepDuration, err := bootstrapMain(ctx, getOptions, performBootstrap, execute, updateBuild)

			So(err, ShouldErrLike, "test error")
			So(sleepDuration, ShouldEqual, 20*time.Second)
			So(record.build, ShouldResembleProtoJSON, `{
				"status": "INFRA_FAILURE",
				"summary_markdown": "<pre>test error</pre>"
			}`)
		})

		Convey("returns original error if updating build fails", func() {
			bootstrapErr := errors.New("test bootstrap failure")
			performBootstrap := testBootstrapFn(bootstrapErr)
			updateBuildErr := errors.New("test update build failure")
			_, updateBuild := testUpdateBuildFn(updateBuildErr)

			sleepDuration, err := bootstrapMain(ctx, getOptions, performBootstrap, execute, updateBuild)

			So(err, ShouldErrLike, bootstrapErr)
			So(sleepDuration, ShouldEqual, 0)
		})

	})
}
