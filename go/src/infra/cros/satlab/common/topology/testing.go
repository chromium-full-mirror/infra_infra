// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package topology

import (
	"fmt"
	"os/exec"
	"strings"

	"go.chromium.org/infra/cros/satlab/common/utils/executor"
)

// FakeCommanderWithCommandError creates fake command executor which will
// return error on requested command.
// If no command will be passed the function, the successful fake executor
// will be returned.
func FakeCommanderWithCommandError(onCommand string) *executor.FakeCommander {
	return &executor.FakeCommander{
		FakeFn: func(in *exec.Cmd) ([]byte, error) {
			cmd := strings.Join(in.Args, " ")
			if onCommand != "" && strings.Contains(cmd, onCommand) {
				return nil, fmt.Errorf("error on command: %s, the full command is: %s", onCommand, cmd)
			}
			return []byte(""), nil
		},
	}
}
