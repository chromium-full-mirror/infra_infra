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
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/cros/cmd/provision/android-provision/common"
	"go.chromium.org/infra/cros/cmd/provision/android-provision/service"
	mock_common_utils "go.chromium.org/infra/cros/cmd/provision/mock-common-utils"
)

func TestInstallAPKCommand(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ftt.Run("InstallAPKCommand", t, func(t *ftt.Test) {
		associatedHost := mock_common_utils.NewMockServiceAdapterInterface(ctrl)
		pkgProto := &api.CIPDPackage{
			AndroidPackage: api.AndroidPackage_GMS_CORE,
		}
		svc, _ := service.NewAndroidServiceFromExistingConnection(
			associatedHost,
			"dutSerialNumber",
			nil,
			[]*api.CIPDPackage{pkgProto},
		)
		provisionPkg := svc.ProvisionPackages[0]
		provisionPkg.APKFile = &service.PkgFile{
			Name:    "apkName.apk",
			GsPath:  "gs_path",
			DutPath: "/tmp/instanceId/apkName.apk",
		}
		provisionPkg.AndroidPackage = &service.AndroidPackage{
			PackageName: common.GMSCorePackageName,
			VersionCode: "224312037",
		}
		provisionDir, _ := os.MkdirTemp("", "testCleanup")
		defer os.RemoveAll(provisionDir)

		cmd := NewInstallAPKCommand(context.Background(), svc)

		t.Run("Execute", func(t *ftt.Test) {
			log, _ := common.SetUpLog(provisionDir)
			installArgs := []string{"-s", "dutSerialNumber", "install", "-r", "-d", "-g", "/tmp/instanceId/apkName.apk"}
			versionArgs := []string{"-s", "dutSerialNumber", "shell", "dumpsys", "package", common.GMSCorePackageName, "|", "grep", "versionCode", "|", "sort", "-r", "|", "head", "-n", "1"}
			gomock.InOrder(
				associatedHost.EXPECT().RunCmd(gomock.Any(), gomock.Eq("adb"), gomock.Eq(installArgs)).Return("", nil).Times(1),
				associatedHost.EXPECT().RunCmd(gomock.Any(), gomock.Eq("adb"), gomock.Eq(versionArgs)).Return("versionCode=9876543210 minSdk=30 targetSdk=33", nil),
			)
			assert.Loosely(t, cmd.Execute(log), should.BeNil)
			assert.Loosely(t, provisionPkg.AndroidPackage.UpdatedVersionCode, should.Match("9876543210"))
		})
		t.Run("Revert", func(t *ftt.Test) {
			assert.Loosely(t, cmd.Revert(), should.BeNil)
		})
		t.Run("GetErrorMessage", func(t *ftt.Test) {
			assert.Loosely(t, cmd.GetErrorMessage(), should.Equal("failed to install APK"))
		})
		t.Run("GetStatus", func(t *ftt.Test) {
			assert.Loosely(t, cmd.GetStatus(), should.Equal(api.InstallResponse_STATUS_PROVISIONING_FAILED))
		})
	})
}
