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
	"go.chromium.org/infra/cros/cmd/provision/android-provision/common/gsstorage"
	"go.chromium.org/infra/cros/cmd/provision/android-provision/service"
	mock_common_utils "go.chromium.org/infra/cros/cmd/provision/mock-common-utils"
)

func TestCopyDataCommand(t *testing.T) {
	citest.LocalOnlyBecause(t, "b/402551644")
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ftt.Run("CopyDataCommand", t, func(t *ftt.Test) {
		associatedHost := mock_common_utils.NewMockServiceAdapterInterface(ctrl)
		pkgProto := &api.CIPDPackage{
			Name: "cipd_path/cipd_package_name",
			VersionOneof: &api.CIPDPackage_InstanceId{
				InstanceId: "instanceId",
			},
			AndroidPackage: api.AndroidPackage_GMS_CORE,
		}
		apkFile := &service.PkgFile{
			Name:   "apkName.apk",
			GsPath: "gsPath",
		}
		svc, _ := service.NewAndroidServiceFromExistingConnection(
			associatedHost,
			"dutSerialNumber",
			nil,
			[]*api.CIPDPackage{pkgProto},
		)
		provisionPkg := svc.ProvisionPackages[0]
		provisionPkg.CIPDPackage.InstanceId = "instanceId"
		provisionPkg.CIPDPackage.VersionCode = "versionCode"
		provisionPkg.APKFile = apkFile
		provisionDir, _ := os.MkdirTemp("", "testCleanup")
		defer os.RemoveAll(provisionDir)
		svc.OS = &service.AndroidOS{
			ImagePath: &service.ImagePath{
				DutAndroidProductOut: "dutProvisionDir",
			}}
		mockGsClient := gsstorage.NewMockGsClient(ctrl)
		cmd := NewCopyDataCommand(context.Background(), svc)
		cmd.gs = mockGsClient

		t.Run("Execute - copy package", func(t *ftt.Test) {
			log, _ := common.SetUpLog(provisionDir)
			cmd.ctx = context.WithValue(cmd.ctx, common.StageCtxKey, common.PackageFetch)
			gomock.InOrder(
				associatedHost.EXPECT().CopyData(gomock.Any(), "gsPath", "/tmp/instanceId/apkName.apk").Times(1),
			)
			assert.Loosely(t, provisionPkg.APKFile.DutPath, should.BeEmpty)
			assert.Loosely(t, cmd.Execute(log), should.BeNil)
			assert.Loosely(t, provisionPkg.APKFile.DutPath, should.Equal("/tmp/instanceId/apkName.apk"))
		})
		t.Run("Execute - copy os images from folder", func(t *ftt.Test) {
			svc.OS.ImagePath.GsPath = "gs://bucket/folder1/folder2/"
			cmd.ctx = context.WithValue(cmd.ctx, common.StageCtxKey, common.OSFetch)
			log, _ := common.SetUpLog(provisionDir)
			mockGsClient.EXPECT().ListFiles(gomock.Any(), gomock.Eq("folder1/folder2/"), gomock.Eq("/")).Return([]string{"bootloader.img", "radio.img", "smtg-img-2132123.zip"}, nil).Times(1)
			associatedHost.EXPECT().CopyData(gomock.Any(), gomock.Any(), gomock.Eq("/mnt/stateful_partition/android_provision/folder1/folder2/bootloader.img")).Times(1)
			associatedHost.EXPECT().CopyData(gomock.Any(), gomock.Any(), gomock.Eq("/mnt/stateful_partition/android_provision/folder1/folder2/radio.img")).Times(1)
			associatedHost.EXPECT().CopyData(gomock.Any(), gomock.Any(), gomock.Eq("/mnt/stateful_partition/android_provision/folder1/folder2/smtg-img-2132123.zip")).Times(1)
			assert.Loosely(t, cmd.Execute(log), should.BeNil)
			assert.Loosely(t, svc.OS.ImagePath.Files, should.Resemble([]string{"bootloader.img", "radio.img", "smtg-img-2132123.zip"}))
		})
		t.Run("Execute - undefined stage", func(t *ftt.Test) {
			cmd.ctx = context.WithValue(cmd.ctx, common.StageCtxKey, nil)
			log, _ := common.SetUpLog(provisionDir)
			assert.Loosely(t, cmd.Execute(log), should.NotBeNil)
		})
		t.Run("Revert", func(t *ftt.Test) {
			assert.Loosely(t, cmd.Revert(), should.BeNil)
		})
		t.Run("GetErrorMessage", func(t *ftt.Test) {
			assert.Loosely(t, cmd.GetErrorMessage(), should.Equal("failed to copy data"))
		})
		t.Run("GetStatus", func(t *ftt.Test) {
			assert.Loosely(t, cmd.GetStatus(), should.Equal(api.InstallResponse_STATUS_GS_DOWNLOAD_FAILED))
		})
	})
}
