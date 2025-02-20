// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bootstrap

import (
	"context"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"

	"go.chromium.org/infra/chromium/bootstrapper/clients/cas"
	"go.chromium.org/infra/chromium/bootstrapper/clients/cipd"
	fakecas "go.chromium.org/infra/chromium/bootstrapper/clients/fakes/cas"
	fakecipd "go.chromium.org/infra/chromium/bootstrapper/clients/fakes/cipd"
)

func TestDownloadPackages(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	fakePackagesRoot := filepath.Join(t.TempDir(), "fake-packages-root")

	ftt.Run("DownloadPackages", t, func(t *ftt.Test) {

		exePkg := &fakecipd.Package{
			Refs:      map[string]string{},
			Instances: map[string]*fakecipd.PackageInstance{},
		}
		ctx := cipd.UseClientFactory(ctx, fakecipd.Factory(map[string]*fakecipd.Package{
			"fake-exe-package": exePkg,
		}))

		fakeCas := &fakecas.Instance{
			Blobs: map[string]bool{},
		}
		ctx = cas.UseCasClientFactory(ctx, fakecas.Factory(map[string]*fakecas.Instance{
			"fake-cas-instance": fakeCas,
		}))

		packageChannels := map[string]chan<- string{}

		t.Run("fails on nil input", func(t *ftt.Test) {
			exe, cmd, err := DownloadPackages(ctx, nil, fakePackagesRoot, nil)

			assert.Loosely(t, err, should.ErrLike("nil input provided"))
			assert.Loosely(t, exe, should.BeNil)
			assert.Loosely(t, cmd, should.BeNil)
		})

		t.Run("fails on empty CIPD root", func(t *ftt.Test) {
			exe, cmd, err := DownloadPackages(ctx, &Input{}, "", nil)

			assert.Loosely(t, err, should.ErrLike("empty packagesRoot provided"))
			assert.Loosely(t, exe, should.BeNil)
			assert.Loosely(t, cmd, should.BeNil)
		})

		t.Run("fails when provided channel for exe", func(t *ftt.Test) {
			packageChannels[ExeId] = make(chan string, 1)

			exe, cmd, err := DownloadPackages(ctx, &Input{}, fakePackagesRoot, packageChannels)

			assert.Loosely(t, err, should.ErrLike("channel provided for ExeId"))
			assert.Loosely(t, exe, should.BeNil)
			assert.Loosely(t, cmd, should.BeNil)
		})

		t.Run("fails when provided channel for unknown ID", func(t *ftt.Test) {
			packageChannels["foo"] = make(chan string, 1)

			exe, cmd, err := DownloadPackages(ctx, &Input{}, fakePackagesRoot, packageChannels)

			assert.Loosely(t, err, should.ErrLike("channel provided for unknown package ID foo"))
			assert.Loosely(t, exe, should.BeNil)
			assert.Loosely(t, cmd, should.BeNil)
		})

		t.Run("fails when provided an unbuffer channel", func(t *ftt.Test) {
			packageChannels[DepotToolsId] = make(chan string)

			exe, cmd, err := DownloadPackages(ctx, &Input{}, fakePackagesRoot, packageChannels)

			assert.Loosely(t, err, should.ErrLike("channel for package ID depot-tools is unbuffered"))
			assert.Loosely(t, exe, should.BeNil)
			assert.Loosely(t, cmd, should.BeNil)
		})

		t.Run("downloading exe from CIPD", func(t *ftt.Test) {
			input := &Input{
				propsProperties: &BootstrapPropertiesProperties{
					ConfigProject: &BootstrapPropertiesProperties_TopLevelProject_{
						TopLevelProject: &BootstrapPropertiesProperties_TopLevelProject{
							Repo: &GitilesRepo{
								Host:    "fake-host",
								Project: "fake-project",
							},
							Ref: "fake-ref",
						},
					},
					PropertiesFile: "fake/properties.json",
				},
				exeProperties: &BootstrapExeProperties{
					Exe: &buildbucketpb.Executable{
						CipdPackage: "fake-exe-package",
						CipdVersion: "fake-exe-version",
						Cmd:         []string{"fake-binary", "fake-arg1", "fake-arg2"},
					},
				},
			}

			t.Run("fails if ensuring packages fails", func(t *ftt.Test) {
				exePkg.Refs["fake-exe-version"] = ""

				exe, cmd, err := DownloadPackages(ctx, input, fakePackagesRoot, packageChannels)

				assert.Loosely(t, err, should.ErrLike("unknown version"))
				assert.Loosely(t, exe, should.BeNil)
				assert.Loosely(t, cmd, should.BeNil)
			})

			t.Run("returns exe info and command on success", func(t *ftt.Test) {
				exePkg.Refs["fake-exe-version"] = "fake-exe-instance"

				exe, cmd, err := DownloadPackages(ctx, input, fakePackagesRoot, packageChannels)

				assert.Loosely(t, err, should.BeNil)
				assert.That(t, exe, should.Match(mustParseBootstrappedExe(`{
					"cipd": {
						"server": "https://chrome-infra-packages.appspot.com",
						"package": "fake-exe-package",
						"requested_version": "fake-exe-version",
						"actual_version": "fake-exe-instance"
					},
					"cmd": [
						"fake-binary",
						"fake-arg1",
						"fake-arg2"
					]
				}`)))
				assert.Loosely(t, cmd, should.Match([]string{filepath.Join(fakePackagesRoot, "cipd", "exe", "fake-binary"), "fake-arg1", "fake-arg2"}))
			})

			t.Run("downloads depot_tools for dependent project", func(t *ftt.Test) {
				input.propsProperties.ConfigProject = &BootstrapPropertiesProperties_DependencyProject_{
					DependencyProject: &BootstrapPropertiesProperties_DependencyProject{
						TopLevelRepo: &GitilesRepo{
							Host:    "fake-top-level-host",
							Project: "fake-top-level-project",
						},
						TopLevelRef: "fake-top-level-ref",
						ConfigRepo: &GitilesRepo{
							Host:    "fake-config-host",
							Project: "fake-config-project",
						},
						ConfigRepoLocator: &BootstrapPropertiesProperties_DependencyProject_ConfigRepoPath{
							ConfigRepoPath: "path/to/config/repo",
						},
					},
				}
				depotToolsCh := make(chan string, 1)
				packageChannels[DepotToolsId] = depotToolsCh

				_, _, err := DownloadPackages(ctx, input, fakePackagesRoot, packageChannels)

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, len(depotToolsCh), should.Equal(1))
				depotToolsPackagePath := <-depotToolsCh
				assert.Loosely(t, depotToolsPackagePath, should.Equal(filepath.Join(fakePackagesRoot, "cipd", "depot-tools")))
			})

		})

		t.Run("downloading exe from CAS", func(t *ftt.Test) {
			input := &Input{
				propsProperties: &BootstrapPropertiesProperties{
					ConfigProject: &BootstrapPropertiesProperties_TopLevelProject_{
						TopLevelProject: &BootstrapPropertiesProperties_TopLevelProject{
							Repo: &GitilesRepo{
								Host:    "fake-host",
								Project: "fake-project",
							},
							Ref: "fake-ref",
						},
					},
					PropertiesFile: "fake/properties.json",
				},
				exeProperties: &BootstrapExeProperties{
					Exe: &buildbucketpb.Executable{
						CipdPackage: "fake-exe-package",
						CipdVersion: "fake-exe-version",
						Cmd:         []string{"fake-binary", "fake-arg1", "fake-arg2"},
					},
				},
				casRecipeBundle: &apipb.CASReference{
					CasInstance: "fake-cas-instance",
					Digest: &apipb.Digest{
						Hash:      "fake-cas-hash",
						SizeBytes: 42,
					},
				},
			}

			t.Run("fails if downloading from CAS fails", func(t *ftt.Test) {
				fakeCas.Blobs["fake-cas-hash"] = false

				exe, cmd, err := DownloadPackages(ctx, input, fakePackagesRoot, packageChannels)

				assert.Loosely(t, err, should.ErrLike("hash fake-cas-hash does not identify any blobs"))
				assert.Loosely(t, exe, should.BeNil)
				assert.Loosely(t, cmd, should.BeNil)
			})

			t.Run("returns exe info and command on success", func(t *ftt.Test) {
				exe, cmd, err := DownloadPackages(ctx, input, fakePackagesRoot, packageChannels)

				assert.Loosely(t, err, should.BeNil)
				assert.That(t, exe, should.Match(mustParseBootstrappedExe(`{
					"cas": {
						"cas_instance": "fake-cas-instance",
						"digest": {
							"hash": "fake-cas-hash",
							"size_bytes": 42
						}
					},
					"cmd": [
						"fake-binary",
						"fake-arg1",
						"fake-arg2"
					]
				}`)))
				assert.Loosely(t, cmd, should.Match([]string{filepath.Join(fakePackagesRoot, "cas", "fake-binary"), "fake-arg1", "fake-arg2"}))
			})

			t.Run("downloads depot_tools for dependent project", func(t *ftt.Test) {
				input.propsProperties.ConfigProject = &BootstrapPropertiesProperties_DependencyProject_{
					DependencyProject: &BootstrapPropertiesProperties_DependencyProject{
						TopLevelRepo: &GitilesRepo{
							Host:    "fake-top-level-host",
							Project: "fake-top-level-project",
						},
						TopLevelRef: "fake-top-level-ref",
						ConfigRepo: &GitilesRepo{
							Host:    "fake-config-host",
							Project: "fake-config-project",
						},
						ConfigRepoLocator: &BootstrapPropertiesProperties_DependencyProject_ConfigRepoPath{
							ConfigRepoPath: "path/to/config/repo",
						},
					},
				}
				depotToolsCh := make(chan string, 1)
				packageChannels[DepotToolsId] = depotToolsCh

				_, _, err := DownloadPackages(ctx, input, fakePackagesRoot, packageChannels)

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, len(depotToolsCh), should.Equal(1))
				depotToolsPackagePath := <-depotToolsCh
				assert.Loosely(t, depotToolsPackagePath, should.Equal(filepath.Join(fakePackagesRoot, "cipd", "depot-tools")))
			})

		})

	})

}

func mustParseBootstrappedExe(msg string) *BootstrappedExe {
	var data BootstrappedExe
	if err := protojson.Unmarshal([]byte(msg), &data); err != nil {
		panic(err)
	}
	return &data
}
