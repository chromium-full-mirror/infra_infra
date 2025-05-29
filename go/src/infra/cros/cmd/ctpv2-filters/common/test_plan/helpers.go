// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package test_plan

import (
	"log"
	"strconv"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"
)

const (
	alRunKey = "is_al_run"
)

func SuiteExecutionMetadataArgValue(req *api.InternalTestplan, flag string) string {
	for _, arg := range req.GetSuiteInfo().GetSuiteMetadata().GetExecutionMetadata().GetArgs() {
		if strings.EqualFold(arg.GetFlag(), flag) {
			return arg.GetValue()
		}
	}
	return ""
}

func IsAlRun(req *api.InternalTestplan) bool {
	alRunValue := SuiteExecutionMetadataArgValue(req, alRunKey)
	if alRunValue == "" {
		return false
	}
	alRun, err := strconv.ParseBool(alRunValue)
	if err != nil {
		log.Printf("Unable to get value of %s due to: %q, defaulting to false", alRunKey, err)
		return false
	}
	return alRun
}
