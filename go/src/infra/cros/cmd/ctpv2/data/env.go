// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package data

import (
	"github.com/google/uuid"

	"go.chromium.org/chromiumos/infra/proto/go/test_platform/config"

	"go.chromium.org/infra/libs/skylab/worker"
)

// env implements the worker.Environment interface.
type env struct {
	luciProject string
	logdogHost  string
}

// LUCIProject implements worker.Environment interface.
func (e *env) LUCIProject() string {
	return e.luciProject
}

// LogDogHost implements worker.Environment interface.
func (e *env) LogDogHost() string {
	return e.logdogHost
}

// GenerateLogPrefix implements worker.Environment interface.
func (e *env) GenerateLogPrefix() string {
	return "skylab/" + uuid.New().String()
}

func Wrap(c *config.Config_SkylabWorker) worker.Environment {
	return &env{
		logdogHost:  c.LogDogHost,
		luciProject: c.LuciProject,
	}
}
