// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tpm provides function to work with TPM.
package tpm

import (
	"context"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

// tpmStatus is a data structure to represent the parse-version of the Status.
type tpmStatus struct {
	statusMap map[string]string
	success   bool
}

// NewTpmStatus retrieves the TPM status for the DUT and returns the
// status values as a map.
func NewTpmStatus(ctx context.Context, ha components.HostAccess, timeout time.Duration) *tpmStatus {
	statusRes, _ := ha.Run(ctx, timeout, "tpm_manager_client", "status", "--nonsensitive")
	log.Debugf(ctx, "New Tpm Status :%q", statusRes.GetStdout())
	statusItems := strings.Split(statusRes.GetStdout(), "\n")
	var ts = &tpmStatus{
		statusMap: make(map[string]string),
		// The uppercase on this string is deliberate.
		success: strings.Contains(strings.ToUpper(statusRes.GetStdout()), "STATUS_SUCCESS"),
	}
	// Following the logic in Labpack, if the TPM status string
	// contains 2 lines or fewer, we will return an empty map for the
	// TPM status values.
	if len(statusItems) > 2 {
		statusItems = statusItems[1 : len(statusItems)-1]
		for _, statusLine := range statusItems {
			item := strings.Split(statusLine, ":")[:]
			if item[0] == "" {
				continue
			}
			if len(item) == 1 {
				item = append(item, "")
			}
			for i, j := range item {
				item[i] = strings.TrimSpace(j)
			}
			ts.statusMap[item[0]] = item[1]
			// The labpack (Python) implementation checks whether the
			// string item[1] contains true of false in the string
			// form, and then explicitly converts that boolean
			// values. We do not attempt that here since the key and
			// value types for maps are strongly typed in Go-lang.
		}
	}
	return ts
}

// HasSuccess checks whether the TpmStatus includes success indicator
// or not.
func (s *tpmStatus) HasSuccess() bool {
	return s.success
}

// IsOwned checks whether TPM has been cleared or not.
func (s *tpmStatus) IsOwned() (bool, error) {
	if len(s.statusMap) == 0 {
		return false, errors.Reason("tpm status is owned: not initialized").Err()
	}
	return s.statusMap["is_owned"] == "true", nil
}
