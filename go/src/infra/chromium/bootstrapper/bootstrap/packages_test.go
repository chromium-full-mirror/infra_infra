// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bootstrap

import (
	"context"
	"path/filepath"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	. "go.chromium.org/luci/common/testing/assertions"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"

	"infra/chromium/bootstrapper/clients/cas"
	"infra/chromium/bootstrapper/clients/cipd"
	fakecas "infra/chromium/bootstrapper/clients/fakes/cas"
	fakecipd "infra/chromium/bootstrapper/clients/fakes/cipd"
)

func TestDownloadPackages(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	fakePackagesRoot := filepath.Join(t.TempDir(), "fake-packages-root")

	Convey("DownloadPackages", t, func() {

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

		Convey("fails on nil input", func() {
			exe, cmd, err := DownloadPackages(ctx, nil, fakePackagesRoot, nil)

			So(err, ShouldErrLike, "nil input provided")
			So(exe, ShouldBeNil)
			So(cmd, ShouldBeNil)
		})

		Convey("fails on empty CIPD root", func() {
			exe, cmd, err := DownloadPackages(ctx, &Input{}, "", nil)

			So(err, ShouldErrLike, "empty packagesRoot provided")
			So(exe, ShouldBeNil)
			So(cmd, ShouldBeNil)
		})

		Convey("fails when provided channel for exe", func() {
			packageChannels[ExeId] = make(chan string, 1)

			exe, cmd, err := DownloadPackages(ctx, &Input{}, fakePackagesRoot, packageChannels)

			So(err, ShouldErrLike, "channel provided for ExeId")
			So(exe, ShouldBeNil)
			So(cmd, ShouldBeNil)
		})

		Convey("fails when provided channel for unknown ID", func() {
			packageChannels["foo"] = make(chan string, 1)

			exe, cmd, err := DownloadPackages(ctx, &Input{}, fakePackagesRoot, packageChannels)

			So(err, ShouldErrLike, "channel provided for unknown package ID foo")
			So(exe, ShouldBeNil)
			So(cmd, ShouldBeNil)
		})

		Convey("fails when provided an unbuffer channel", func() {
			packageChannels[DepotToolsId] = make(chan string)

			exe, cmd, err := DownloadPackages(ctx, &Input{}, fakePackagesRoot, packageChannels)

			So(err, ShouldErrLike, "channel for package ID depot-tools is unbuffered")
			So(exe, ShouldBeNil)
			So(cmd, ShouldBeNil)
		})

		Convey("downloading exe from CIPD", func() {
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

			Convey("fails if ensuring packages fails", func() {
				exePkg.Refs["fake-exe-version"] = ""

				exe, cmd, err := DownloadPackages(ctx, input, fakePackagesRoot, packageChannels)

				So(err, ShouldErrLike, "unknown version")
				So(exe, ShouldBeNil)
				So(cmd, ShouldBeNil)
			})

			Convey("returns exe info and command on success", func() {
				exePkg.Refs["fake-exe-version"] = "fake-exe-instance"

				exe, cmd, err := DownloadPackages(ctx, input, fakePackagesRoot, packageChannels)

				So(err, ShouldBeNil)
				So(exe, ShouldResembleProtoJSON, `{
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
				}`)
				So(cmd, ShouldResemble, []string{filepath.Join(fakePackagesRoot, "cipd", "exe", "fake-binary"), "fake-arg1", "fake-arg2"})
			})

			Convey("downloads depot_tools for dependent project", func() {
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

				So(err, ShouldBeNil)
				So(len(depotToolsCh), ShouldEqual, 1)
				depotToolsPackagePath := <-depotToolsCh
				So(depotToolsPackagePath, ShouldEqual, filepath.Join(fakePackagesRoot, "cipd", "depot-tools"))
			})

		})

		Convey("downloading exe from CAS", func() {
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

			Convey("fails if downloading from CAS fails", func() {
				fakeCas.Blobs["fake-cas-hash"] = false

				exe, cmd, err := DownloadPackages(ctx, input, fakePackagesRoot, packageChannels)

				So(err, ShouldErrLike, "hash fake-cas-hash does not identify any blobs")
				So(exe, ShouldBeNil)
				So(cmd, ShouldBeNil)
			})

			Convey("returns exe info and command on success", func() {
				exe, cmd, err := DownloadPackages(ctx, input, fakePackagesRoot, packageChannels)

				So(err, ShouldBeNil)
				So(exe, ShouldResembleProtoJSON, `{
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
				}`)
				So(cmd, ShouldResemble, []string{filepath.Join(fakePackagesRoot, "cas", "fake-binary"), "fake-arg1", "fake-arg2"})
			})

			Convey("downloads depot_tools for dependent project", func() {
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

				So(err, ShouldBeNil)
				So(len(depotToolsCh), ShouldEqual, 1)
				depotToolsPackagePath := <-depotToolsCh
				So(depotToolsPackagePath, ShouldEqual, filepath.Join(fakePackagesRoot, "cipd", "depot-tools"))
			})

		})

	})

}
