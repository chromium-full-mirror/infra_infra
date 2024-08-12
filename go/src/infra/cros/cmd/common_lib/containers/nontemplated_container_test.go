// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package containers

import (
	"context"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"go.chromium.org/chromiumos/config/go/test/api"

	"infra/cros/cmd/common_lib/tools/crostoolrunner"
)

func TestNonTemplatedInitialize(t *testing.T) {
	t.Parallel()

	Convey("Initialize_correct_type", t, func() {
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewNonTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		gotContType := cont.GetContainerType()
		So(gotContType, ShouldNotBeNil)
		So(gotContType, ShouldEqual, wantContType)
	})

	Convey("Initialize_success", t, func() {
		ctx := context.Background()
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewNonTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		err := cont.Initialize(ctx, nil)
		So(err, ShouldBeNil)
	})
}

func TestNonTemplatedStartContainer(t *testing.T) {
	t.Parallel()

	Convey("StartContainer_empty_req", t, func() {
		ctx := context.Background()
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewNonTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		resp, err := cont.StartContainer(ctx)
		So(err, ShouldNotBeNil)
		So(resp, ShouldBeNil)
	})

	Convey("StartContainer_empty_req", t, func() {
		ctx := context.Background()
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewNonTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		cont.StartContainerReq = &api.StartContainerRequest{}
		resp, err := cont.StartContainer(ctx)
		So(err, ShouldNotBeNil)
		So(resp, ShouldBeNil)
	})
}
