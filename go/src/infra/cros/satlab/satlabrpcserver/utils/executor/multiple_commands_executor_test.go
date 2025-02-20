// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package executor

import (
	"os/exec"
	"testing"

	"github.com/google/go-cmp/cmp"

	"go.chromium.org/infra/cros/satlab/common/utils/executor"
)

func TestExecuteMultipleCommandShouldSuccess(t *testing.T) {
	cmd1 := exec.Command("cmd1")
	cmd2 := exec.Command("cmd2")
	commandsExecutor := New(cmd1, cmd2)
	res, err := commandsExecutor.Exec(&executor.FakeCommander{CmdOutput: "cmd1 out"})

	if err != nil {
		t.Errorf("execute multiple commands should success, got an {%v}", err)
		return
	}

	if diff := cmp.Diff(string(res), "cmd1 out"); diff != "" {
		t.Errorf("Expect cmd1 out, got {%v}", res)
	}
}
