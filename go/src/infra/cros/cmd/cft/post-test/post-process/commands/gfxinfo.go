// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/cft/common/dutinteraction"
)

// GetGfxInfo packages graphics/hardware_probe labels in the GfxLabels member of GetGfxInfoResponse
func GetGfxInfo(logger *log.Logger, dutClient api.DutServiceClient) (*api.GetGfxInfoResponse, error) {
	reportingLabelsJson, err := dutinteraction.RunCmd(context.Background(), "/usr/local/graphics/hardware_probe", []string{"--labels-reporting"}, dutClient)

	if err != nil {
		logger.Printf("gfx hardware_probe cmd FAILED: %s\n", err)
		reportingLabelsJson = ""
	}

	labelsMap, err := labelsInputToStringMap(reportingLabelsJson)
	if err != nil {
		logger.Printf("json conversion FAILED: %s\n", err)
	}

	resp := &api.GetGfxInfoResponse{GfxLabels: labelsMap}
	return resp, nil
}

// labelsInputToStringMap converts a json object string into map[string]string
func labelsInputToStringMap(labelsJson string) (map[string]string, error) {
	labelsBytes := []byte(labelsJson)

	var labelsObj any
	var err = json.Unmarshal(labelsBytes, &labelsObj)
	if err != nil {
		return nil, err
	}

	labelsMap, ok := labelsObj.(map[string]any)
	if !ok {
		err := errors.New("labels json is expected to be an object")
		return nil, err
	}

	labelsMapStr := make(map[string]string)
	for k, v := range labelsMap {
		labelsMapStr[k] = fmt.Sprintf("%v", v)
	}

	return labelsMapStr, nil
}
