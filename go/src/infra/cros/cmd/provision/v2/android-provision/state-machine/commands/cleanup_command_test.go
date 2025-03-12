// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/smartystreets/goconvey/convey"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/test/provision/v2/android-provision/common"
	"go.chromium.org/chromiumos/test/provision/v2/android-provision/service"
	mock_common_utils "go.chromium.org/chromiumos/test/provision/v2/mock-common-utils"
	"go.chromium.org/luci/common/testing/localonly"
)

func TestCleanupCommand(t *testing.T) {
	localonly.Because(t, "b/402551644")
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	Convey("CleanupCommand", t, func() {
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

		Convey("Execute - OSInstall", func() {
			cmd.ctx = context.WithValue(cmd.ctx, common.StageCtxKey, common.OSInstall)
			log, _ := common.SetUpLog(provisionDir)
			associatedHost.EXPECT().DeleteDirectory(gomock.Any(), gomock.Eq("/tmp_DutAndroidProductOut")).Times(1)
			So(cmd.Execute(log), ShouldBeNil)
		})
		Convey("Execute - PackageInstall", func() {
			cmd.ctx = context.WithValue(cmd.ctx, common.StageCtxKey, common.PackageInstall)
			log, _ := common.SetUpLog(provisionDir)
			associatedHost.EXPECT().DeleteDirectory(gomock.Any(), gomock.Eq("/tmp/instanceId")).Times(1)
			So(cmd.Execute(log), ShouldBeNil)
		})
		Convey("Execute - PostInstall", func() {
			cmd.ctx = context.WithValue(cmd.ctx, common.StageCtxKey, common.PostInstall)
			log, _ := common.SetUpLog(provisionDir)
			So(cmd.Execute(log), ShouldBeNil)
			_, err := os.Stat(svc.ProvisionDir)
			So(os.IsNotExist(err), ShouldBeTrue)
		})
		Convey("Revert", func() {
			So(cmd.Revert(), ShouldBeNil)
		})
		Convey("GetErrorMessage", func() {
			So(cmd.GetErrorMessage(), ShouldEqual, "failed to cleanup temp files")
		})
		Convey("GetStatus - OSInstall", func() {
			cmd.ctx = context.WithValue(cmd.ctx, common.StageCtxKey, common.OSInstall)
			So(cmd.GetStatus(), ShouldEqual, api.InstallResponse_STATUS_PROVISIONING_FAILED)
		})
		Convey("GetStatus - PackageInstall", func() {
			cmd.ctx = context.WithValue(cmd.ctx, common.StageCtxKey, common.PackageInstall)
			So(cmd.GetStatus(), ShouldEqual, api.InstallResponse_STATUS_PROVISIONING_FAILED)
		})
		Convey("GetStatus - PostInstall", func() {
			cmd.ctx = context.WithValue(cmd.ctx, common.StageCtxKey, common.PostInstall)
			So(cmd.GetStatus(), ShouldEqual, api.InstallResponse_STATUS_POST_PROVISION_SETUP_FAILED)
		})
	})
}
