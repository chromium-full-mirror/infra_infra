// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package executors

import (
	"infra/cros/cmd/common_lib/interfaces"
)

// All supported executor types.
const (
	AndroidDutExecutorType         interfaces.ExecutorType = "AndroidDutExecutor"
	AndroidProvisionExecutorType   interfaces.ExecutorType = "AndroidProvisionExecutor"
	InvServiceExecutorType         interfaces.ExecutorType = "InvServiceExecutor"
	CacheServerExecutorType        interfaces.ExecutorType = "CacheServerExecutor"
	CrosDutExecutorType            interfaces.ExecutorType = "CrosDutExecutor"
	CrosDutVmExecutorType          interfaces.ExecutorType = "CrosDutVmExecutor"
	CrosProvisionExecutorType      interfaces.ExecutorType = "CrosProvisionExecutor"
	CrosVMProvisionExecutorType    interfaces.ExecutorType = "CrosVMProvisionExecutor"
	CrosTestExecutorType           interfaces.ExecutorType = "CrosTestExecutor"
	CrosTestFinderExecutorType     interfaces.ExecutorType = "CrosTestFinderExecutor"
	CrosGcsPublishExecutorType     interfaces.ExecutorType = "CrosGcsPublishExecutor"
	CrosTkoPublishExecutorType     interfaces.ExecutorType = "CrosTkoPublishExecutor"
	CrosRdbPublishExecutorType     interfaces.ExecutorType = "CrosRdbPublishExecutor"
	CrosPublishExecutorType        interfaces.ExecutorType = "CrosPublishExecutor"
	SshTunnelExecutorType          interfaces.ExecutorType = "SshTunnelExecutor"
	GenericProvisionExecutorType   interfaces.ExecutorType = "GenericProvisionExecutor"
	GenericTestsExecutorType       interfaces.ExecutorType = "GenericTestsExecutor"
	GenericPostProcessExecutorType interfaces.ExecutorType = "GenericPostProcessExecutor"
	GenericPublishExecutorType     interfaces.ExecutorType = "GenericPublishExecutor"
	GenericServiceExecutorType     interfaces.ExecutorType = "GenericServiceExecutor"

	// For testing purpose only
	NoExecutorType interfaces.ExecutorType = "NoExecutor"
)
