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
	"go.chromium.org/infra/cros/cmd/provision/android-provision/common/zip"
	"go.chromium.org/infra/cros/cmd/provision/android-provision/service"
	mock_common_utils "go.chromium.org/infra/cros/cmd/provision/mock-common-utils"
)

func TestExtractZipCommand(t *testing.T) {
	citest.LocalOnlyBecause(t, "b/402551644")
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ftt.Run("ExtractZipCommand", t, func(t *ftt.Test) {
		pkgProto := &api.CIPDPackage{
			Name: "cipd_path/cipd_package_name",
			VersionOneof: &api.CIPDPackage_InstanceId{
				InstanceId: "instanceId",
			},
			AndroidPackage: api.AndroidPackage_GMS_CORE,
		}
		cipdPkg := &service.CIPDPackage{
			PackageProto: pkgProto,
			FilePath:     "/tmp/instanceId/cipd_package_name.zip",
			PackageName:  "cipd_package_name",
			InstanceId:   "instanceId",
			VersionCode:  "1234567890",
		}
		associatedHost := mock_common_utils.NewMockServiceAdapterInterface(ctrl)
		svc, _ := service.NewAndroidServiceFromExistingConnection(
			associatedHost,
			"",
			nil,
			[]*api.CIPDPackage{pkgProto},
		)
		provisionPkg := svc.ProvisionPackages[0]
		provisionPkg.CIPDPackage = cipdPkg
		provisionDir, _ := os.MkdirTemp("", "testCleanup")
		defer os.RemoveAll(provisionDir)
		svc.ProvisionDir = provisionDir
		svc.OS = &service.AndroidOS{
			ImagePath: &service.ImagePath{
				DutAndroidProductOut: "dutProvisionDir",
			}}
		ctx := context.Background()
		cmd := NewExtractZipCommand(ctx, svc)
		t.Run("Execute - PackageFetch", func(t *ftt.Test) {
			cmd.ctx = context.WithValue(cmd.ctx, common.StageCtxKey, common.PackageFetch)
			mockZipReader := zip.NewMockZipReaderInterface(ctrl)
			cmd.zip = mockZipReader
			log, _ := common.SetUpLog(provisionDir)

			mockZipReader.EXPECT().UnzipFile(gomock.Eq("/tmp/instanceId/cipd_package_name.zip"), gomock.Eq(provisionDir+"/instanceId")).Times(1)
			assert.Loosely(t, cmd.Execute(log), should.BeNil)
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
			assert.Loosely(t, cmd.GetErrorMessage(), should.Equal("failed to extract zip file"))
		})
		t.Run("GetStatus", func(t *ftt.Test) {
			assert.Loosely(t, cmd.GetStatus(), should.Equal(api.InstallResponse_STATUS_PRE_PROVISION_SETUP_FAILED))
		})
	})
}
