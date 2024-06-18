// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dolos

import (
	"context"
	"encoding/json"
	"fmt"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/tlw"
)

const (
	// User the doloscmd to query the Dolos status.
	dolosGetStatusCmdGlob = "/usr/bin/doloscmd get-status --serial %s"
)

// Eventually use the files generated from doloscmt.proto.
type dolosStatusResponse struct {
	Status string `json:"status"`
}

// setDolosStateExec calculate the current Dolos state and update UFS.
func setDolosStateExec(ctx context.Context, info *execs.ExecInfo) error {

	dolos := info.GetChromeos().GetDolos()
	if dolos == nil {
		return errors.Reason("set dolos state: not specified").Err()
	}
	previousState := info.GetChromeos().GetDolos().GetState()
	info.GetChromeos().GetDolos().State = tlw.Dolos_DOLOS_UNKNOWN

	resource := dolos.GetHostname()
	run := info.NewRunner(resource)
	output, err := run(ctx, info.GetExecTimeout(), fmt.Sprintf(dolosGetStatusCmdGlob, dolos.GetSerialCable()))
	if err != nil {
		return errors.Annotate(err, "set dolos state").Err()
	}

	var decoded dolosStatusResponse
	err = json.Unmarshal([]byte(output), &decoded)
	if err != nil {
		return errors.Annotate(err, "set dolos state").Err()
	}

	newState := decoded.Status
	log.Debugf(ctx, "Previous dolos state: %s", previousState)
	if v, ok := tlw.Dolos_State_value[newState]; ok {
		info.GetChromeos().GetDolos().State = tlw.Dolos_State(v)
		log.Infof(ctx, "Set dolos state to be: %s", newState)
		return nil
	}
	return errors.Reason("set dolos state: state is %q not found", newState).Err()
}

func init() {
	execs.Register("set_dolos_state", setDolosStateExec)
}
