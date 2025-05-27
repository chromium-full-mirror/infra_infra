// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/mock/gomock"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/testing/citest"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/cros/cmd/provision/android-provision/common"
	"go.chromium.org/infra/cros/cmd/provision/android-provision/service"
	mock_common_utils "go.chromium.org/infra/cros/cmd/provision/mock-common-utils"
)

func TestFlashOsCommand(t *testing.T) {
	citest.LocalOnlyBecause(t, "b/402551644")
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ftt.Run("FlashOsCommandCommand", t, func(t *ftt.Test) {
		associatedHost := mock_common_utils.NewMockServiceAdapterInterface(ctrl)
		svc, _ := service.NewAndroidServiceFromExistingConnection(
			associatedHost,
			"dutSerialNumber",
			nil,
			nil,
		)
		provisionDir, _ := os.MkdirTemp("", "testCleanup")
		defer os.RemoveAll(provisionDir)
		log, _ := common.SetUpLog(provisionDir)
		cmd := NewFlashOsCommand(context.Background(), svc)

		t.Run("Execute", func(t *ftt.Test) {
			svc.OS = &service.AndroidOS{
				ImagePath: &service.ImagePath{
					DutAndroidProductOut: provisionDir,
					GsPath:               "gs://gs_bucket/folder/image",
					Files:                []string{"abcd/bootloader-model-123456789.img", "radio-model-123456789.img", "model-img-123456789.zip"},
				},
			}
			bootloaderArgs := []string{"-s", "dutSerialNumber", "flash", "bootloader", provisionDir + "/abcd/bootloader-model-123456789.img"}
			radioArgs := []string{"-s", "dutSerialNumber", "flash", "radio", provisionDir + "/radio-model-123456789.img"}
			updateArgs := []string{"-s", "dutSerialNumber", "update", provisionDir + "/model-img-123456789.zip"}
			rebootArgs := []string{"-s", "dutSerialNumber", "reboot", "bootloader"}
			waitArgs := []string{"devices", "|", "grep", "-sw", "dutSerialNumber", "|", "awk", "'{print $2}'"}
			fetchBuildIdArgs := []string{"-s", "dutSerialNumber", "shell", "getprop", "ro.build.id"}
			fetchOSIncrementalVersionArgs := []string{"-s", "dutSerialNumber", "shell", "getprop", "ro.build.version.incremental"}
			fetchOSReleaseVersionArgs := []string{"-s", "dutSerialNumber", "shell", "getprop", "ro.build.version.release"}
			tmpDirPath := filepath.Join(svc.OS.ImagePath.DutAndroidProductOut, "/tmp")

			gomock.InOrder(
				associatedHost.EXPECT().RunCmd(gomock.Any(), gomock.Eq("fastboot"), gomock.Eq(bootloaderArgs)).Return("any string", nil).Times(1),
				associatedHost.EXPECT().RunCmd(gomock.Any(), gomock.Eq("fastboot"), gomock.Eq(rebootArgs)).Return("any string", nil).Times(1),
				associatedHost.EXPECT().RunCmd(gomock.Any(), gomock.Eq("fastboot"), gomock.Eq(waitArgs)).Return("fastboot", nil).Times(1),
				associatedHost.EXPECT().RunCmd(gomock.Any(), gomock.Eq("fastboot"), gomock.Eq(radioArgs)).Return("any string", nil).Times(1),
				associatedHost.EXPECT().RunCmd(gomock.Any(), gomock.Eq("fastboot"), gomock.Eq(rebootArgs)).Return("any string", nil).Times(1),
				associatedHost.EXPECT().RunCmd(gomock.Any(), gomock.Eq("fastboot"), gomock.Eq(waitArgs)).Return("fastboot", nil).Times(1),
				associatedHost.EXPECT().CreateDirectories(gomock.Any(), []string{tmpDirPath}),
				associatedHost.EXPECT().RunCmd(gomock.Any(), gomock.Eq("TMPDIR="+tmpDirPath+" fastboot"), gomock.Eq(updateArgs)).Return("any string", nil).Times(1),
				associatedHost.EXPECT().RunCmd(gomock.Any(), gomock.Eq("adb"), gomock.Eq(waitArgs)).Return("device", nil).Times(3),
				associatedHost.EXPECT().RunCmd(gomock.Any(), gomock.Eq("adb"), gomock.Eq(fetchBuildIdArgs)).Return("any string", nil).Times(1),
				associatedHost.EXPECT().RunCmd(gomock.Any(), gomock.Eq("adb"), gomock.Eq(fetchOSIncrementalVersionArgs)).Return("1234567890", nil).Times(1),
				associatedHost.EXPECT().RunCmd(gomock.Any(), gomock.Eq("adb"), gomock.Eq(fetchOSReleaseVersionArgs)).Return("10", nil).Times(1),
			)
			assert.Loosely(t, cmd.Execute(log), should.BeNil)
		})
		t.Run("Execute - Nothing to provision", func(t *ftt.Test) {
			log, _ := common.SetUpLog(provisionDir)
			svc.OS = nil
			associatedHost.EXPECT().RunCmd(gomock.Any(), gomock.Eq("fastboot"), gomock.Any()).Times(0)
			assert.Loosely(t, cmd.Execute(log), should.BeNil)
		})
		t.Run("Revert", func(t *ftt.Test) {
			assert.Loosely(t, cmd.Revert(), should.BeNil)
		})
		t.Run("GetErrorMessage", func(t *ftt.Test) {
			assert.Loosely(t, cmd.GetErrorMessage(), should.Equal("failed to flash Android OS"))
		})
		t.Run("GetStatus", func(t *ftt.Test) {
			assert.Loosely(t, cmd.GetStatus(), should.Equal(api.InstallResponse_STATUS_PROVISIONING_FAILED))
		})
	})
}
