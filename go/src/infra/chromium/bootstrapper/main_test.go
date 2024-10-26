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

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/logdog/client/butlerlib/streamclient"

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

	ftt.Run("performBootstrap", t, func(t *ftt.Test) {

		t.Run("fails if reading input fails", func(t *ftt.Test) {
			input := reader{func(p []byte) (int, error) {
				return 0, errors.New("test read failure")
			}}

			cmd, exeInput, err := performBootstrap(ctx, input, opts)

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, cmd, should.BeNil)
			assert.Loosely(t, exeInput, should.BeNil)
		})

		t.Run("fails if unmarshalling build fails", func(t *ftt.Test) {
			input := strings.NewReader("invalid-proto")

			cmd, exeInput, err := performBootstrap(ctx, input, opts)

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, cmd, should.BeNil)
			assert.Loosely(t, exeInput, should.BeNil)
		})

		t.Run("fails if bootstrap fails", func(t *ftt.Test) {
			input := createInput(`{}`)

			cmd, exeInput, err := performBootstrap(ctx, input, opts)

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, cmd, should.BeNil)
			assert.Loosely(t, exeInput, should.BeNil)
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

		t.Run("fails if determining executable fails", func(t *ftt.Test) {
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

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, cmd, should.BeNil)
			assert.Loosely(t, exeInput, should.BeNil)
		})

		t.Run("succeeds for valid input", func(t *ftt.Test) {
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

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, cmd, should.Resemble([]string{
				filepath.Join(opts.packagesRoot, "cipd", "exe", "fake-exe"),
				"--output",
				opts.outputPath,
			}))
			build := &buildbucketpb.Build{}
			proto.Unmarshal(exeInput, build)
			assert.That(t, build, should.Match(mustParseBBProto(`{
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
			}`)))
		})

		t.Run("succeeds for polymorphic with build properties prioritized over builder properties", func(t *ftt.Test) {
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
			assert.Loosely(t, err, should.BeNil)
			build := &buildbucketpb.Build{}
			proto.Unmarshal(exeInput, build)
			assert.That(t, build, should.Match(mustParseBBProto(`{
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
			}`)))
		})

		t.Run("succeeds for properties-optional without $bootstrap/properties", func(t *ftt.Test) {
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

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, cmd, should.Resemble([]string{
				filepath.Join(opts.packagesRoot, "cipd", "exe", "fake-exe"),
				"--output",
				opts.outputPath,
			}))
			build := &buildbucketpb.Build{}
			proto.Unmarshal(exeInput, build)
			assert.Loosely(t, build, should.Match(mustParseBBProto(`{
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
			}`)))
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

type buildUpdateRecords struct {
	builds []*buildbucketpb.Build
}

type datagramStream struct {
	errs    []error
	records *buildUpdateRecords
}

func (d *datagramStream) WriteDatagram(dg []byte) error {
	build := &buildbucketpb.Build{}
	PanicOnError(proto.Unmarshal(dg, build))

	d.records.builds = append(d.records.builds, build)

	var err error
	if len(d.errs) > 0 {
		err = d.errs[0]
		d.errs = d.errs[1:]
	}
	return err
}

func (d *datagramStream) Close() error {
	return nil
}

func testGetStreamFn(getStreamErr error, updateErrs ...error) (*buildUpdateRecords, getStreamFn) {
	records := &buildUpdateRecords{}

	getStream := func(ctx context.Context) (streamclient.DatagramStream, error) {
		if getStreamErr != nil {
			return nil, getStreamErr
		}
		stream := &datagramStream{
			errs:    updateErrs,
			records: records,
		}
		return stream, nil
	}

	return records, getStream
}

func TestBootstrapMain(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ftt.Run("bootstrapMain", t, func(t *ftt.Test) {

		getOptions := func() options { return options{} }
		performBootstrap := testBootstrapFn(nil)
		execute := testExecuteCmdFn(nil)
		records, getStream := testGetStreamFn(nil)

		t.Run("does not update build on success", func(t *ftt.Test) {
			sleepDuration, err := bootstrapMain(ctx, getOptions, performBootstrap, execute, getStream)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, sleepDuration, should.BeZero)
			assert.Loosely(t, records.builds, should.BeEmpty)
		})

		t.Run("does not update build on bootstrapped exe failure", func(t *ftt.Test) {
			exeErr := &exec.ExitError{
				ProcessState: &os.ProcessState{},
				Stderr:       []byte("test exe failure"),
			}
			execute := testExecuteCmdFn(exeErr)

			sleepDuration, err := bootstrapMain(ctx, getOptions, performBootstrap, execute, getStream)

			assert.Loosely(t, err, should.ErrLike(exeErr))
			assert.Loosely(t, sleepDuration, should.BeZero)
			assert.Loosely(t, records.builds, should.BeEmpty)
		})

		t.Run("updates build when failing to execute bootstrapped exe", func(t *ftt.Test) {
			cmdErr := errors.New("test cmd execution failure")
			execute := testExecuteCmdFn(cmdErr)

			sleepDuration, err := bootstrapMain(ctx, getOptions, performBootstrap, execute, getStream)

			assert.Loosely(t, err, should.ErrLike(cmdErr))
			assert.Loosely(t, sleepDuration, should.BeZero)
			assert.Loosely(t, len(records.builds), should.Equal(2))
			assert.That(t, records.builds[0], should.Match(mustParseBBProto(`{
				"output": {
					"status": "STARTED"
				}
			}`)))
			assert.That(t, records.builds[1], should.Match(mustParseBBProto(`{
				"summary_markdown": "<pre>test cmd execution failure</pre>",
				"output": {
					"status": "INFRA_FAILURE"
				}
			}`)))
		})

		t.Run("updates build on failure of non-bootstrapped exe process", func(t *ftt.Test) {
			cmdErr := &exec.ExitError{
				ProcessState: &os.ProcessState{},
				Stderr:       []byte("test process failure"),
			}
			performBootstrap := testBootstrapFn(cmdErr)

			sleepDuration, err := bootstrapMain(ctx, getOptions, performBootstrap, execute, getStream)

			assert.Loosely(t, err, should.ErrLike(cmdErr))
			assert.Loosely(t, sleepDuration, should.BeZero)
			assert.Loosely(t, len(records.builds), should.Equal(2))
			assert.That(t, records.builds[0], should.Match(mustParseBBProto(`{
				"output": {
					"status": "STARTED"
				}
			}`)))
			assert.Loosely(t, records.builds[1], should.Match(mustParseBBProto(fmt.Sprintf(`{
				"summary_markdown": "<pre>%s</pre>",
				"output": {
					"status": "INFRA_FAILURE"
				}
			}`, cmdErr))))
		})

		t.Run("updates build for generic bootstrap failure", func(t *ftt.Test) {
			bootstrapErr := errors.New("test bootstrap failure")
			performBootstrap := testBootstrapFn(bootstrapErr)

			sleepDuration, err := bootstrapMain(ctx, getOptions, performBootstrap, execute, getStream)

			assert.Loosely(t, err, should.ErrLike(bootstrapErr))
			assert.Loosely(t, sleepDuration, should.BeZero)
			assert.Loosely(t, len(records.builds), should.Equal(2))
			assert.That(t, records.builds[0], should.Match(mustParseBBProto(`{
				"output": {
					"status": "STARTED"
				}
			}`)))
			assert.That(t, records.builds[1], should.Match(mustParseBBProto(`{
				"summary_markdown": "<pre>test bootstrap failure</pre>",
				"output": {
					"status": "INFRA_FAILURE"
				}
			}`)))
		})

		t.Run("updates build for patch rejected failure", func(t *ftt.Test) {
			bootstrapErr := errors.New("test bootstrap failure")
			bootstrapErr = bootstrap.PatchRejected.Apply(bootstrapErr)
			performBootstrap := testBootstrapFn(bootstrapErr)

			sleepDuration, err := bootstrapMain(ctx, getOptions, performBootstrap, execute, getStream)

			assert.Loosely(t, err, should.ErrLike(bootstrapErr))
			assert.Loosely(t, sleepDuration, should.BeZero)
			assert.Loosely(t, len(records.builds), should.Equal(2))
			assert.That(t, records.builds[0], should.Match(mustParseBBProto(`{
				"output": {
					"status": "STARTED"
				}
			}`)))
			assert.Loosely(t, records.builds[1], should.Match(mustParseBBProto(`{
				"summary_markdown": "<pre>Patch failure: See build stderr log. Try rebasing?</pre>",
				"output": {
					"status": "FAILURE",
					"properties": {
						"failure_type": "PATCH_FAILURE"
					}
				}
			}`)))
		})

		t.Run("returns sleep duration for sleep tagged error", func(t *ftt.Test) {
			bootstrapErr := errors.New("test error")
			bootstrapErr = bootstrap.SleepBeforeExiting.With(20 * time.Second).Apply(bootstrapErr)
			performBootstrap := testBootstrapFn(bootstrapErr)

			sleepDuration, err := bootstrapMain(ctx, getOptions, performBootstrap, execute, getStream)

			assert.Loosely(t, err, should.ErrLike("test error"))
			assert.Loosely(t, sleepDuration, should.Equal(20*time.Second))
			assert.Loosely(t, len(records.builds), should.Equal(2))
			assert.That(t, records.builds[0], should.Match(mustParseBBProto(`{
				"output": {
					"status": "STARTED"
				}
			}`)))
			assert.That(t, records.builds[1], should.Match(mustParseBBProto(`{
				"summary_markdown": "<pre>test error</pre>",
				"output": {
					"status": "INFRA_FAILURE"
				}
			}`)))
		})

		t.Run("returns original error if getting stream fails", func(t *ftt.Test) {
			bootstrapErr := errors.New("test bootstrap failure")
			performBootstrap := testBootstrapFn(bootstrapErr)
			getStreamErr := errors.New("test get stream failure")
			_, getStream := testGetStreamFn(getStreamErr)

			sleepDuration, err := bootstrapMain(ctx, getOptions, performBootstrap, execute, getStream)

			assert.Loosely(t, err, should.ErrLike(bootstrapErr))
			assert.Loosely(t, sleepDuration, should.BeZero)
		})

		t.Run("returns original error if writing initial build fails", func(t *ftt.Test) {
			bootstrapErr := errors.New("test bootstrap failure")
			performBootstrap := testBootstrapFn(bootstrapErr)
			initialBuildErr := errors.New("test initial build failure")
			_, getStream := testGetStreamFn(nil, initialBuildErr)

			sleepDuration, err := bootstrapMain(ctx, getOptions, performBootstrap, execute, getStream)

			assert.Loosely(t, err, should.ErrLike(bootstrapErr))
			assert.Loosely(t, sleepDuration, should.BeZero)
		})

		t.Run("returns original error if updating build fails", func(t *ftt.Test) {
			bootstrapErr := errors.New("test bootstrap failure")
			performBootstrap := testBootstrapFn(bootstrapErr)
			updateBuildErr := errors.New("test update build failure")
			_, getStream := testGetStreamFn(nil, nil, updateBuildErr)

			sleepDuration, err := bootstrapMain(ctx, getOptions, performBootstrap, execute, getStream)

			assert.Loosely(t, err, should.ErrLike(bootstrapErr))
			assert.Loosely(t, sleepDuration, should.BeZero)
		})

	})
}

func mustParseBBProto(msg string) *buildbucketpb.Build {
	var data buildbucketpb.Build
	if err := protojson.Unmarshal([]byte(msg), &data); err != nil {
		panic(err)
	}
	return &data
}
