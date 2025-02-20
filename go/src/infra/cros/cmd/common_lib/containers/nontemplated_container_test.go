// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package containers

import (
	"context"
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/cros/cmd/common_lib/tools/crostoolrunner"
)

func TestNonTemplatedInitialize(t *testing.T) {
	t.Parallel()

	ftt.Run("Initialize_correct_type", t, func(t *ftt.Test) {
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewNonTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		gotContType := cont.GetContainerType()
		assert.Loosely(t, gotContType, should.NotEqual(""))
		assert.Loosely(t, gotContType, should.Equal(wantContType))
	})

	ftt.Run("Initialize_success", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewNonTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		err := cont.Initialize(ctx, nil)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestNonTemplatedStartContainer(t *testing.T) {
	t.Parallel()

	ftt.Run("StartContainer_empty_req", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewNonTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		resp, err := cont.StartContainer(ctx)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("StartContainer_empty_req", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewNonTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		cont.StartContainerReq = &api.StartContainerRequest{}
		resp, err := cont.StartContainer(ctx)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})
}
