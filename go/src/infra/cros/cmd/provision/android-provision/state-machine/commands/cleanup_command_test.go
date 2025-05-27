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
	"go.chromium.org/luci/common/testing/citest"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/cros/cmd/provision/android-provision/common"
	"go.chromium.org/infra/cros/cmd/provision/android-provision/service"
	mock_common_utils "go.chromium.org/infra/cros/cmd/provision/mock-common-utils"
)

func TestCleanupCommand(t *testing.T) {
	citest.LocalOnlyBecause(t, "b/402551644")
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ftt.Run("CleanupCommand", t, func(t *ftt.Test) {
		associatedHost := mock_common_utils.NewMockServiceAdapterInterface(ctrl)
		pkgProto := &api.CIPDPackage{
			Name: "cipd_path/cipd_package_name",
			VersionOneof: &api.CIPDPackage_InstanceId{
				InstanceId: "instanceId",
			},
			AndroidPackage: api.AndroidPackage_GMS_CORE,
		}
		apkFile := &service.PkgFile{
			Name:    "apkName.apk",
			GsPath:  "gsPath",
			DutPath: "/tmp/instanceId/apkName.apk",
		}
		svc, _ := service.NewAndroidServiceFromExistingConnection(
			associatedHost,
			"dutSerialNumber",
			&api.AndroidOsImage{LocationOneof: &api.AndroidOsImage_OsVersion{OsVersion: "10"}},
			[]*api.CIPDPackage{pkgProto},
		)
		svc.ProvisionPackages[0].APKFile = apkFile
		provisionDir, _ := os.MkdirTemp("", "testCleanup")
		defer os.RemoveAll(provisionDir)
		svc.ProvisionDir = provisionDir
		svc.OS.ImagePath.GsPath = "gs://gs_bucket/folder/image"
		svc.OS.ImagePath.DutAndroidProductOut = "/tmp_DutAndroidProductOut"
		cmd := NewCleanupCommand(context.Background(), svc)

		t.Run("Execute - OSInstall", func(t *ftt.Test) {
			cmd.ctx = context.WithValue(cmd.ctx, common.StageCtxKey, common.OSInstall)
			log, _ := common.SetUpLog(provisionDir)
			associatedHost.EXPECT().DeleteDirectory(gomock.Any(), gomock.Eq("/tmp_DutAndroidProductOut")).Times(1)
			assert.Loosely(t, cmd.Execute(log), should.BeNil)
		})
		t.Run("Execute - PackageInstall", func(t *ftt.Test) {
			cmd.ctx = context.WithValue(cmd.ctx, common.StageCtxKey, common.PackageInstall)
			log, _ := common.SetUpLog(provisionDir)
			associatedHost.EXPECT().DeleteDirectory(gomock.Any(), gomock.Eq("/tmp/instanceId")).Times(1)
			assert.Loosely(t, cmd.Execute(log), should.BeNil)
		})
		t.Run("Execute - PostInstall", func(t *ftt.Test) {
			cmd.ctx = context.WithValue(cmd.ctx, common.StageCtxKey, common.PostInstall)
			log, _ := common.SetUpLog(provisionDir)
			assert.Loosely(t, cmd.Execute(log), should.BeNil)
			_, err := os.Stat(svc.ProvisionDir)
			assert.Loosely(t, os.IsNotExist(err), should.BeTrue)
		})
		t.Run("Revert", func(t *ftt.Test) {
			assert.Loosely(t, cmd.Revert(), should.BeNil)
		})
		t.Run("GetErrorMessage", func(t *ftt.Test) {
			assert.Loosely(t, cmd.GetErrorMessage(), should.Equal("failed to cleanup temp files"))
		})
		t.Run("GetStatus - OSInstall", func(t *ftt.Test) {
			cmd.ctx = context.WithValue(cmd.ctx, common.StageCtxKey, common.OSInstall)
			assert.Loosely(t, cmd.GetStatus(), should.Equal(api.InstallResponse_STATUS_PROVISIONING_FAILED))
		})
		t.Run("GetStatus - PackageInstall", func(t *ftt.Test) {
			cmd.ctx = context.WithValue(cmd.ctx, common.StageCtxKey, common.PackageInstall)
			assert.Loosely(t, cmd.GetStatus(), should.Equal(api.InstallResponse_STATUS_PROVISIONING_FAILED))
		})
		t.Run("GetStatus - PostInstall", func(t *ftt.Test) {
			cmd.ctx = context.WithValue(cmd.ctx, common.StageCtxKey, common.PostInstall)
			assert.Loosely(t, cmd.GetStatus(), should.Equal(api.InstallResponse_STATUS_POST_PROVISION_SETUP_FAILED))
		})
	})
}
