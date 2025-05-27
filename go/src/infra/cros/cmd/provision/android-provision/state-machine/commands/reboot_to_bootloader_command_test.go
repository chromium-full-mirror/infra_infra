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

func TestRebootToBootloaderCommand(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ftt.Run("RebootToBootloaderCommand", t, func(t *ftt.Test) {
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
		cmd := NewRebootToBootloaderCommand(context.Background(), svc)

		t.Run("Execute", func(t *ftt.Test) {
			svc.OS = &service.AndroidOS{
				ImagePath: &service.ImagePath{
					GsPath: "gs://gs_bucket/folder/image",
				},
			}
			rebootArgs := []string{"-s", "dutSerialNumber", "reboot", "bootloader"}
			waitArgs := []string{"devices", "|", "grep", "-sw", "dutSerialNumber", "|", "awk", "'{print $2}'"}
			gomock.InOrder(
				associatedHost.EXPECT().RunCmd(gomock.Any(), gomock.Eq("adb"), gomock.Eq(rebootArgs)).Return("", nil).Times(1),
				associatedHost.EXPECT().RunCmd(gomock.Any(), gomock.Eq("fastboot"), gomock.Eq(waitArgs)).Return("fastboot", nil).Times(1),
			)
			assert.Loosely(t, cmd.Execute(log), should.BeNil)
		})
		t.Run("Execute - Nothing to provision", func(t *ftt.Test) {
			log, _ := common.SetUpLog(provisionDir)
			svc.OS = nil
			associatedHost.EXPECT().RunCmd(gomock.Any(), gomock.Eq("adb"), gomock.Any()).Times(0)
			associatedHost.EXPECT().RunCmd(gomock.Any(), gomock.Eq("fastboot"), gomock.Any()).Times(0)
			assert.Loosely(t, cmd.Execute(log), should.BeNil)
		})
		t.Run("Revert", func(t *ftt.Test) {
			assert.Loosely(t, cmd.Revert(), should.BeNil)
		})
		t.Run("GetErrorMessage", func(t *ftt.Test) {
			assert.Loosely(t, cmd.GetErrorMessage(), should.Equal("failed to reboot to bootloader"))
		})
		t.Run("GetStatus", func(t *ftt.Test) {
			assert.Loosely(t, cmd.GetStatus(), should.Equal(api.InstallResponse_STATUS_PROVISIONING_FAILED))
		})
	})
}
