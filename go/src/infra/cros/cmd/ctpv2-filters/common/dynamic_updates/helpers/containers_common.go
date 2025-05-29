// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package helpers

import (
	"fmt"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/builders"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/common"
)

func NewCrosDutContainer(deviceId *common.DeviceIdentifier) *builders.ContainerBuilder {
	dutContainerBuilder := builders.NewContainerBuilder(
		deviceId.GetCrosDutServer(), common.CrosDut, "", "", "",
	)
	dutContainerBuilder.SetCustomTemplate(&api.Template{
		Container: &api.Template_CrosDut{
			CrosDut: &api.CrosDutTemplate{},
		},
	})
	dutContainerBuilder.SetDynamicDeps(map[string]string{
		"crosDut.cacheServer": common.CacheServer,
		"crosDut.dutAddress":  deviceId.GetDevice("dutServer"),
	})

	return dutContainerBuilder
}

func NewServoNexusContainer(deviceId *common.DeviceIdentifier) *builders.ContainerBuilder {
	servoTaskId := common.NewTaskIdentifier(common.ServoNexus).AddDeviceId(deviceId)
	servoContainerBuilder := builders.NewContainerBuilder(
		servoTaskId.Id, common.ServoNexus, "",
		"/tmp/servod", fmt.Sprintf("%s server -server_port 0", common.CrosServod),
	)
	servoContainerBuilder.Network = "adbnet"

	tlsVars := []string{"DOCKER_CERT_PATH", "DOCKER_HOST", "DOCKER_TLS_VERIFY"}
	servoContainerBuilder.Envs = tlsVars

	return servoContainerBuilder
}

func NewCrosProvisionContainer(ContainerId *common.TaskIdentifier) *builders.ContainerBuilder {
	provisionContainerBuilder := builders.NewContainerBuilder(
		ContainerId.Id, common.CrosProvision, "",
		"/tmp/provisionservice", fmt.Sprintf("%s server -port 0", common.CrosProvision))

	return provisionContainerBuilder
}

func NewAndroidProvisionContainer(taskId *common.TaskIdentifier) *builders.ContainerBuilder {
	container := builders.NewContainerBuilder(
		taskId.Id, "android-provision", "",
		"/tmp/provision",
		"android-provision server -port 0",
	)

	return container
}

func NewCacheServerContainer() *builders.ContainerBuilder {
	container := builders.NewContainerBuilder(
		common.CacheServer, "",
		"us-docker.pkg.dev/cros-registry/test-services/cacheserver:prod", "", "",
	)
	container.SetCustomTemplate(&api.Template{
		Container: &api.Template_CacheServer{
			CacheServer: &api.CacheServerTemplate{
				ApplicationDefaultCredentials: &api.CacheServerTemplate_ServiceAccountKeyfile{
					ServiceAccountKeyfile: "/creds/service_accounts/service-account-chromeos.json",
				},
			},
		},
	})

	return container
}
