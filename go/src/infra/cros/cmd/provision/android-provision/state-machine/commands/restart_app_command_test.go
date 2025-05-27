// Copyright 2023 The Chromium Authors
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

func TestRestartAppCommand(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ftt.Run("RestartAppCommand", t, func(t *ftt.Test) {
		associatedHost := mock_common_utils.NewMockServiceAdapterInterface(ctrl)
		svc, _ := service.NewAndroidServiceFromExistingConnection(
			associatedHost,
			"dutSerialNumber",
			nil,
			[]*api.CIPDPackage{{}},
		)
		provisionPkg := svc.ProvisionPackages[0]
		provisionPkg.APKFile = &service.PkgFile{
			Name:    "apkName.apk",
			GsPath:  "gs_path",
			DutPath: "/tmp/instanceId/apkName.apk",
		}
		provisionPkg.AndroidPackage = &service.AndroidPackage{
			PackageName: common.GMSCorePackageName,
		}
		provisionDir, _ := os.MkdirTemp("", "testCleanup")
		defer os.RemoveAll(provisionDir)

		cmd := NewRestartAppCommand(context.Background(), svc)

		t.Run("Execute", func(t *ftt.Test) {
			provisionPkg.AndroidPackage.UpdatedVersionCode = "224312037"
			log, _ := common.SetUpLog(provisionDir)
			gomock.InOrder(
				associatedHost.EXPECT().RunCmd(gomock.Any(), gomock.Eq("adb"), gomock.Eq([]string{"-s", "dutSerialNumber", "shell", "am", "force-stop", "com.google.android.gms"})).Return("", nil).Times(1),
				associatedHost.EXPECT().RunCmd(gomock.Any(), gomock.Eq("adb"), gomock.Eq([]string{"-s", "dutSerialNumber", "shell", "am", "broadcast", "-a", "com.google.android.gms.INITIALIZE"})).Return("", nil).Times(1),
			)
			assert.Loosely(t, cmd.Execute(log), should.BeNil)
		})
		t.Run("Execute - nothing installed", func(t *ftt.Test) {
			provisionPkg.AndroidPackage.UpdatedVersionCode = ""
			log, _ := common.SetUpLog(provisionDir)
			associatedHost.EXPECT().RunCmd(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			assert.Loosely(t, cmd.Execute(log), should.BeNil)
		})
		t.Run("Revert", func(t *ftt.Test) {
			assert.Loosely(t, cmd.Revert(), should.BeNil)
		})
		t.Run("GetErrorMessage", func(t *ftt.Test) {
			assert.Loosely(t, cmd.GetErrorMessage(), should.Equal("failed to restart application"))
		})
		t.Run("GetStatus", func(t *ftt.Test) {
			assert.Loosely(t, cmd.GetStatus(), should.Equal(api.InstallResponse_STATUS_POST_PROVISION_SETUP_FAILED))
		})
	})
}
