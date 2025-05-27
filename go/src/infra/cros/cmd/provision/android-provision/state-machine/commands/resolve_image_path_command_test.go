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
	"go.chromium.org/luci/common/testing/citest"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/cros/cmd/provision/android-provision/common"
	"go.chromium.org/infra/cros/cmd/provision/android-provision/service"
	mock_common_utils "go.chromium.org/infra/cros/cmd/provision/mock-common-utils"
)

func TestResolveImagePathCommand(t *testing.T) {
	citest.LocalOnlyBecause(t, "b/402551644")
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ftt.Run("ResolveImagePathCommand", t, func(t *ftt.Test) {
		associatedHost := mock_common_utils.NewMockServiceAdapterInterface(ctrl)
		pkgProto := &api.CIPDPackage{
			AndroidPackage: api.AndroidPackage_GMS_CORE,
		}
		svc, _ := service.NewAndroidServiceFromExistingConnection(
			associatedHost,
			"dutSerialNumber",
			&api.AndroidOsImage{LocationOneof: &api.AndroidOsImage_OsVersion{OsVersion: "12"}},
			[]*api.CIPDPackage{pkgProto},
		)
		provisionDir, _ := os.MkdirTemp("", "testCleanup")
		defer os.RemoveAll(provisionDir)

		cmd := NewResolveImagePathCommand(context.Background(), svc)

		t.Run("Execute", func(t *ftt.Test) {
			svc.DUT.Board = "barbet"
			svc.OS.BuildInfo = &service.OsBuildInfo{Id: "QD4A.200805.003"}
			log, _ := common.SetUpLog(provisionDir)
			expectedGSPath := "gs://android-provisioning-images/SQ3A.220705.003.A1/barbet/"
			assert.Loosely(t, cmd.Execute(log), should.BeNil)
			assert.Loosely(t, svc.OS.ImagePath.GsPath, should.Equal(expectedGSPath))
		})
		t.Run("Execute - DUT has the same build", func(t *ftt.Test) {
			svc.DUT.Board = "barbet"
			svc.OS.BuildInfo = &service.OsBuildInfo{Id: "SQ3A.220705.003.A1"}
			log, _ := common.SetUpLog(provisionDir)
			expectedGSPath := ""
			assert.Loosely(t, cmd.Execute(log), should.BeNil)
			assert.Loosely(t, svc.OS.ImagePath.GsPath, should.Equal(expectedGSPath))
		})
		t.Run("Execute - missing board build", func(t *ftt.Test) {
			svc.DUT.Board = "next_board"
			log, _ := common.SetUpLog(provisionDir)
			assert.Loosely(t, cmd.Execute(log), should.NotBeNil)
		})
		t.Run("Revert", func(t *ftt.Test) {
			assert.Loosely(t, cmd.Revert(), should.BeNil)
		})
		t.Run("GetErrorMessage", func(t *ftt.Test) {
			assert.Loosely(t, cmd.GetErrorMessage(), should.Equal("failed to resolve GS image path"))
		})
		t.Run("GetStatus", func(t *ftt.Test) {
			assert.Loosely(t, cmd.GetStatus(), should.Equal(api.InstallResponse_STATUS_PRE_PROVISION_SETUP_FAILED))
		})
	})
}
