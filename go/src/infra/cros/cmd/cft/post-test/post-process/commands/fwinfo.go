// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"log"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/cft/common/dutinteraction"
)

// GetFwInfo will gather fwinfo via crossystem. If any field is not found it will be returned with "".
func GetFwInfo(logger *log.Logger, dutClient api.DutServiceClient) (*api.GetFWInfoResponse, error) {
	roFWID, err := dutinteraction.RunCmd(context.Background(), "crossystem", []string{"ro_fwid"}, dutClient)
	if err != nil {
		logger.Printf("get ro_fwid cmd FAILED: %s\n", err)
		roFWID = ""
	}
	rwFWID, err := dutinteraction.RunCmd(context.Background(), "crossystem", []string{"fwid"}, dutClient)
	if err != nil {
		logger.Printf("get rw_fwid cmd FAILED: %s\n", err)
		rwFWID = ""

	}
	gsctoolOut, err := dutinteraction.RunCmd(context.Background(), "gsctool", []string{"-afM"}, dutClient)
	if err != nil {
		logger.Printf("gsctool -afM cmd FAILED: %s\n", err)
	}
	gscRO := ""
	gscRW := ""

	entries := strings.Split(gsctoolOut, "\n")
	for _, content := range entries {
		if strings.Contains(content, "RO_FW_VER") {
			gscRO = content[strings.LastIndex(content, "=")+1:]
		} else if strings.Contains(content, "RW_FW_VER") {
			gscRW = content[strings.LastIndex(content, "=")+1:]

		}
	}

	kVersion, err := dutinteraction.RunCmd(context.Background(), "uname", []string{"-r"}, dutClient)
	if err != nil {
		logger.Printf("get uname -r cmd FAILED: %s\n", err)
		kVersion = ""
	}
	kVersion = strings.ReplaceAll(kVersion, "\n", "")

	resp := &api.GetFWInfoResponse{RoFwid: roFWID, RwFwid: rwFWID, KernelVersion: kVersion, GscRo: gscRO, GscRw: gscRW}
	return resp, nil

}
