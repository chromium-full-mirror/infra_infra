// Copyright 2022 The Chromium Authors
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
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/cros/cmd/provision/android-provision/common"
	"go.chromium.org/infra/cros/cmd/provision/android-provision/common/gsstorage"
	"go.chromium.org/infra/cros/cmd/provision/android-provision/service"
	mock_common_utils "go.chromium.org/infra/cros/cmd/provision/mock-common-utils"
)

func TestUploadApkToGsCommand(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ftt.Run("UploadApkToGsCommand", t, func(t *ftt.Test) {
		pkgProto := &api.CIPDPackage{
			Name: "cipd_path/cipd_package_name",
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
		// Default apkName.
		apkName := "gmscore_prodsc_arm64_alldpi_release.apk"
		// Create provision dir and cleanup.
		provisionDir, _ := os.MkdirTemp("", "testCleanup")
		defer os.RemoveAll(provisionDir)
		// Create InstanceId dir.
		d := filepath.Join(provisionDir, "instanceId")
		err := os.Mkdir(d, 0755)
		if err != nil {
			t.Fatalf("TestUploadApkToGsCommand Failure: %v", err)
		}
		apkPath := filepath.Join(d, apkName)
		// Create apk file.
		_, err = os.Create(apkPath)
		if err != nil {
			t.Fatalf("TestUploadApkToGsCommand Failure: %v", err)
		}
		svc.ProvisionDir = provisionDir
		provisionPkg := svc.ProvisionPackages[0]
		cipdPkg := &service.CIPDPackage{
			PackageProto: pkgProto,
			FilePath:     filepath.Join(provisionDir, "/instanceId/cipd_package_name.zip"),
			PackageName:  "cipd_package_name",
			InstanceId:   "instanceId",
			VersionCode:  "1234567890",
		}
		provisionPkg.CIPDPackage = cipdPkg
		mockGsClient := gsstorage.NewMockGsClient(ctrl)
		cmd := NewUploadAPKToGSCommand(context.Background(), svc)
		cmd.gs = mockGsClient
		t.Run("Execute", func(t *ftt.Test) {
			log, _ := common.SetUpLog(provisionDir)
			t.Run("Upload Android package", func(t *ftt.Test) {
				gsPath := "gs://android-provisioning-apks/instanceId/" + apkName
				fetchOSReleaseVersionArgs := []string{"-s", "dutSerialNumber", "shell", "getprop", "ro.build.version.release"}
				associatedHost.EXPECT().RunCmd(gomock.Any(), gomock.Eq("adb"), gomock.Eq(fetchOSReleaseVersionArgs)).Return("12", nil).Times(1)
				mockGsClient.EXPECT().Upload(gomock.Eq(context.Background()), gomock.Eq(apkPath), gomock.Eq("instanceId/"+apkName)).Return(nil).Times(1)
				assert.Loosely(t, cmd.Execute(log), should.BeNil)
				assert.Loosely(t, provisionPkg.APKFile.Name, should.Equal(apkName))
				assert.Loosely(t, provisionPkg.APKFile.GsPath, should.Equal(gsPath))
			})
		})
		t.Run("Revert", func(t *ftt.Test) {
			assert.Loosely(t, cmd.Revert(), should.BeNil)
		})
		t.Run("GetErrorMessage", func(t *ftt.Test) {
			assert.Loosely(t, cmd.GetErrorMessage(), should.Equal("failed to extract APK file"))
		})
		t.Run("GetStatus", func(t *ftt.Test) {
			assert.Loosely(t, cmd.GetStatus(), should.Equal(api.InstallResponse_STATUS_GS_UPLOAD_FAILED))
		})
	})
}
