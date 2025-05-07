// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tlw

// DutStateReason describes different reason for states.
type DutStateReason string

const (
	DutStateReasonEmpty                                    = ""
	DutStateReasonInternalStorageFailureFromSMARTInfo      = "INTERNAL_STORAGE_FAILURE_FROM_SMART_INFO"
	DutStateReasonInternalStorageFailureFromBadblocksCheck = "INTERNAL_STORAGE_FAILURE_FROM_BADBLOCKS_CHECK"
	DutStateReasonInternalStoragePartitionHashesMismatch   = "INTERNAL_STORAGE_FAILURE_FROM_PARTITION_HASHES_MISMATCH"
	DutStateReasonInternalStorageCannotDetected            = "INTERNAL_STORAGE_CANNOT_DETECTED"
	DutStateReasonInternalStorageNoSpaceLeft               = "INTERNAL_STORAGE_NO_SPACE_LEFT"
	DutStateReasonInternalStorageIOError                   = "INTERNAL_STORAGE_IO_ERROR_DETECTED"
	DutStateReasonInternalStorageUncategorizedError        = "INTERNAL_STORAGE_UNCATEGORIZED_ERROR"
	DutStateReasonBatteryCapacityTooLow                    = "BATTERY_CHARGING_CAPACITY_TOO_LOW"
)

// NotEmpty checks that the reason is not empty.
func (r DutStateReason) NotEmpty() bool {
	return string(r) != DutStateReasonEmpty
}
