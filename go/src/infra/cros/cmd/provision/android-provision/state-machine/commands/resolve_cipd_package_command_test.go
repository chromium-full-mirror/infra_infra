// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"os"
	"testing"

	"github.com/golang/mock/gomock"

	"go.chromium.org/chromiumos/config/go/test/api"
	luci_cipd "go.chromium.org/luci/cipd/client/cipd"
	luci_cipd_common "go.chromium.org/luci/cipd/common"
	"go.chromium.org/luci/common/testing/citest"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/cros/cmd/provision/android-provision/common"
	"go.chromium.org/infra/cros/cmd/provision/android-provision/common/cipd"
	"go.chromium.org/infra/cros/cmd/provision/android-provision/service"
	mock_common_utils "go.chromium.org/infra/cros/cmd/provision/mock-common-utils"
)

func TestResolveCIPDPackageCommand(t *testing.T) {
	citest.LocalOnlyBecause(t, "b/402551644")
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ftt.Run("ResolveCIPDPackageCommand", t, func(t *ftt.Test) {
		pkgProto := &api.CIPDPackage{
			VersionOneof: &api.CIPDPackage_InstanceId{
				InstanceId: "instanceId",
			},
			AndroidPackage: api.AndroidPackage_GMS_CORE,
		}
		associatedHost := mock_common_utils.NewMockServiceAdapterInterface(ctrl)
		svc, _ := service.NewAndroidServiceFromExistingConnection(
			associatedHost,
			"dutSerialNumber",
			nil,
			[]*api.CIPDPackage{pkgProto},
		)
		provisionPkg := svc.ProvisionPackages[0]
		provisionDir, _ := os.MkdirTemp("", "testCleanup")
		defer os.RemoveAll(provisionDir)

		cmd := NewResolveCIPDPackageCommand(context.Background(), svc)

		t.Run("Execute", func(t *ftt.Test) {
			log, _ := common.SetUpLog(provisionDir)
			mockCIPDClient := cipd.NewMockCIPDClientInterface(ctrl)
			cmd.cipd = mockCIPDClient
			t.Run("New Android Package", func(t *ftt.Test) {
				provisionPkg.CIPDPackage.PackageProto.Name = "cipd_path/cipd_package_name"
				pin := luci_cipd_common.Pin{PackageName: "resolved_cipd_package_name", InstanceID: "resolvedInstanceId"}
				tags := []luci_cipd.TagInfo{{Tag: "arch:arm64"}, {Tag: "build_type:prodrvc"}, {Tag: "dpi:alldpi"}, {Tag: "version_code:222615037"}}
				d := &luci_cipd.InstanceDescription{InstanceInfo: luci_cipd.InstanceInfo{Pin: pin}, Tags: tags}
				mockCIPDClient.EXPECT().Describe(gomock.Eq(pkgProto), gomock.Eq(true), gomock.Eq(false)).Return(d, nil).Times(1)
				assert.Loosely(t, cmd.Execute(log), should.BeNil)
				assert.Loosely(t, provisionPkg.CIPDPackage.PackageProto.Name, should.Equal("cipd_path/cipd_package_name"))
				assert.Loosely(t, provisionPkg.CIPDPackage.PackageName, should.Equal("resolved_cipd_package_name"))
				assert.Loosely(t, provisionPkg.CIPDPackage.InstanceId, should.Equal("resolvedInstanceId"))
				assert.Loosely(t, provisionPkg.CIPDPackage.VersionCode, should.Equal("222615037"))
			})
			t.Run("Resolve CIPD package name", func(t *ftt.Test) {
				provisionPkg.CIPDPackage.PackageProto.Name = ""
				pin := luci_cipd_common.Pin{PackageName: "resolved_cipd_package_name", InstanceID: "resolvedInstanceId"}
				tags := []luci_cipd.TagInfo{{Tag: "arch:arm64"}, {Tag: "build_type:prodrvc"}, {Tag: "dpi:alldpi"}, {Tag: "version_code:222615037"}}
				d := &luci_cipd.InstanceDescription{InstanceInfo: luci_cipd.InstanceInfo{Pin: pin}, Tags: tags}
				versionArgs := []string{"-s", "dutSerialNumber", "shell", "getprop", "ro.build.version.release"}
				associatedHost.EXPECT().RunCmd(gomock.Any(), gomock.Eq("adb"), versionArgs).Return("12", nil).Times(1)
				mockCIPDClient.EXPECT().Describe(gomock.Eq(pkgProto), gomock.Eq(true), gomock.Eq(false)).Return(d, nil).Times(1)
				assert.Loosely(t, cmd.Execute(log), should.BeNil)
				assert.Loosely(t, provisionPkg.CIPDPackage.PackageProto.Name, should.Equal("chromiumos/infra/skylab/third_party/gmscore/gmscore_prodsc_arm64_alldpi_release_apk"))
				assert.Loosely(t, provisionPkg.CIPDPackage.PackageName, should.Equal("resolved_cipd_package_name"))
				assert.Loosely(t, provisionPkg.CIPDPackage.InstanceId, should.Equal("resolvedInstanceId"))
				assert.Loosely(t, provisionPkg.CIPDPackage.VersionCode, should.Equal("222615037"))
			})
		})
		t.Run("Revert", func(t *ftt.Test) {
			assert.Loosely(t, cmd.Revert(), should.BeNil)
		})
		t.Run("GetErrorMessage", func(t *ftt.Test) {
			assert.Loosely(t, cmd.GetErrorMessage(), should.Equal("failed to resolve CIPD package"))
		})
		t.Run("GetStatus", func(t *ftt.Test) {
			assert.Loosely(t, cmd.GetStatus(), should.Equal(api.InstallResponse_STATUS_CIPD_PACKAGE_LOOKUP_FAILED))
		})
	})
}
