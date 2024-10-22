// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executors

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/cros/cmd/common_lib/containers"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
	"infra/cros/cmd/cros_test_runner/internal/mocked_services"
)

func TestAndroidProvisionServiceStart(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	ftt.Run("Android provision service start fails without starting ctr", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewGenericProvisionTemplatedContainer("android-provision", "container/image/path", ctr)
		exec := NewAndroidProvisionExecutor(cont)
		err := exec.Start(ctx)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Android provision service start fails on failing StartTemplatedContainer", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		mocked_client := mocked_services.NewMockCrosToolRunnerContainerServiceClient(ctrl)
		ctr.CtrClient = mocked_client
		getMockedStartTemplatedContainer(mocked_client).Return(nil, fmt.Errorf("some error"))
		cont := containers.NewGenericProvisionTemplatedContainer("android-provision", "container/image/path", ctr)
		exec := NewAndroidProvisionExecutor(cont)
		err := exec.Start(ctx)
		assert.Loosely(t, err, should.NotBeNil)
	})
}
