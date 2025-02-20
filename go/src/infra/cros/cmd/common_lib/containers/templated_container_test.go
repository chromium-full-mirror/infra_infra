// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package containers

import (
	"context"
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/cros/cmd/common_lib/tools/crostoolrunner"
)

func TestCrosDutTemplate(t *testing.T) {
	t.Parallel()

	ftt.Run("Initialize_empty_template", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		err := cont.initializeCrosDutTemplate(ctx, nil)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Initialize_empty_cache_server", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		dutTemplate := &api.CrosDutTemplate{}
		err := cont.initializeCrosDutTemplate(ctx, dutTemplate)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Initialize_empty_dut_address", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		dutTemplate := &api.CrosDutTemplate{CacheServer: &labapi.IpEndpoint{}}
		err := cont.initializeCrosDutTemplate(ctx, dutTemplate)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Initialize_success", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		dutTemplate := &api.CrosDutTemplate{
			CacheServer: &labapi.IpEndpoint{},
			DutAddress:  &labapi.IpEndpoint{}}
		err := cont.initializeCrosDutTemplate(ctx, dutTemplate)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestCrosProvisionTemplate(t *testing.T) {
	t.Parallel()

	ftt.Run("Initialize_empty_template", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		err := cont.initializeCrosProvisionTemplate(ctx, nil)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Initialize_empty_input_req", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		provisionTemplate := &api.CrosProvisionTemplate{}
		err := cont.initializeCrosProvisionTemplate(ctx, provisionTemplate)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Initialize_success", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		provisionTemplate := &api.CrosProvisionTemplate{
			InputRequest: &api.CrosProvisionRequest{},
		}
		err := cont.initializeCrosProvisionTemplate(ctx, provisionTemplate)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestCacheServerTemplate(t *testing.T) {
	t.Parallel()

	createContainer := func() *TemplatedContainer {
		wantContType := CacheServerTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		return cont
	}

	ftt.Run("Initialize_empty_template", t, func(t *ftt.Test) {
		ctx := context.Background()
		cont := createContainer()
		err := cont.initializeCacheServerTemplate(ctx, nil)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Initialize_success", t, func(t *ftt.Test) {
		ctx := context.Background()
		cont := createContainer()
		template := &api.CacheServerTemplate{}
		err := cont.initializeCacheServerTemplate(ctx, template)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestCrosTestTemplate(t *testing.T) {
	t.Parallel()

	ftt.Run("Initialize_empty_template", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		err := cont.initializeCrosTestTemplate(ctx, nil)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Initialize_success", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		testTemplate := &api.CrosTestTemplate{}
		err := cont.initializeCrosTestTemplate(ctx, testTemplate)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestPostProcessTemplate(t *testing.T) {
	t.Parallel()

	ftt.Run("Initialize_empty_template", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantContType := PostProcessTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		err := cont.initializeCrosTestTemplate(ctx, nil)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Initialize_success", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantContType := PostProcessTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		postProcessTemplate := &api.PostProcessTemplate{}
		err := cont.initializePostProcessTemplate(ctx, postProcessTemplate)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestCrosPublishTemplate(t *testing.T) {
	t.Parallel()

	ftt.Run("Initialize_empty_template", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		err := cont.initializeCrosPublishTemplate(ctx, nil)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Initialize_publish_src_dir_missing", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		publishTemplate := &api.CrosPublishTemplate{PublishType: api.CrosPublishTemplate_PUBLISH_GCS}
		err := cont.initializeCrosPublishTemplate(ctx, publishTemplate)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Initialize_success", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		publishTemplate := &api.CrosPublishTemplate{PublishType: api.CrosPublishTemplate_PUBLISH_GCS, PublishSrcDir: "src/dir/loc"}
		err := cont.initializeCrosPublishTemplate(ctx, publishTemplate)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestTemplatedInitialize(t *testing.T) {
	t.Parallel()

	ftt.Run("Initialize_cros_dut", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		template := &api.Template{Container: &api.Template_CrosDut{}}
		err := cont.Initialize(ctx, template)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Initialize_cros_provision", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		template := &api.Template{Container: &api.Template_CrosProvision{}}
		err := cont.Initialize(ctx, template)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Initialize_cros_test", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		template := &api.Template{Container: &api.Template_CrosTest{}}
		err := cont.Initialize(ctx, template)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Initialize_post_process", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantContType := PostProcessTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		template := &api.Template{Container: &api.Template_PostProcess{}}
		err := cont.Initialize(ctx, template)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Initialize_cros_publish", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		template := &api.Template{Container: &api.Template_CrosPublish{}}
		err := cont.Initialize(ctx, template)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Initialize_cros_publish_success", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		template := &api.Template{Container: &api.Template_CrosPublish{CrosPublish: &api.CrosPublishTemplate{PublishType: api.CrosPublishTemplate_PUBLISH_GCS, PublishSrcDir: "src/dir/loc"}}}
		err := cont.Initialize(ctx, template)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestTemplatedStartContainer(t *testing.T) {
	t.Parallel()

	ftt.Run("StartContainer_empty_req", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		resp, err := cont.StartContainer(ctx)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("StartContainer_empty_ctr", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantContType := CrosProvisionTemplatedContainerType
		cont := NewTemplatedContainer(wantContType, "test-container", "host", "container-image", nil)
		cont.StartTemplatedContainerReq = &api.StartTemplatedContainerRequest{}
		resp, err := cont.StartContainer(ctx)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	ftt.Run("StartContainer_failure", t, func(t *ftt.Test) {
		ctx := context.Background()
		wantContType := CrosProvisionTemplatedContainerType
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := NewTemplatedContainer(wantContType, "test-container", "host", "container-image", ctr)
		cont.StartTemplatedContainerReq = &api.StartTemplatedContainerRequest{}
		resp, err := cont.StartContainer(ctx)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})
}
