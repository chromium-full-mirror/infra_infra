// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package storage

import (
	"context"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"

	"infra/cros/recovery/tlw"
)

// Test cases for TestStorageSMARTFieldValue
var storageSMARTFieldValueTests = []struct {
	testName      string
	rawOutput     string
	expectedType  tlw.Storage_Type
	expectedState StorageState
}{
	{
		"StorageTypeUnspecified, StorageStateUndefined, no error",
		`
		xxxxxx
		xxxxxx
		`,
		tlw.Storage_TYPE_UNSPECIFIED,
		StorageStateUndefined,
	},
	{
		"SSD Type, StorageStateNormal, no error",
		`
		xxxxxx
		SATA Version is: SATA 3.1, 6.0 Gb/s (current: 6.0 Gb/s)
		xxxxxx
		`,
		tlw.Storage_SSD,
		StorageStateNormal,
	},
	{
		"SSD Type, StorageStateCritical, no error",
		`
		xxxxxx
		SATA Version is: SATA 3.1, 6.0 Gb/s (current: 6.0 Gb/s)
		184 End-to-End_Error   PO--CK   001   001   097    NOW  135
		xxxxxx
		`,
		tlw.Storage_SSD,
		StorageStateCritical,
	},
	{
		"SSD Type, StorageStateWarning, no error",
		`
		xxxxxx
		SATA Version is: SATA 3.1, 6.0 Gb/s (current: 6.0 Gb/s)
		7 Reallocated_Sector_Ct   PO--CK   101   001   097
		xxxxxx
		`,
		tlw.Storage_SSD,
		StorageStateWarning,
	},
	{
		"MMC Type, StorageStateCritical, no error",
		`
		xxxxxx
		Extended CSD rev 1.7 (MMC 5.0)
		PRE_EOL_INFO: 0x03
		DEVICE_LIFE_TIME_EST_TYP_A: 0x01
		xxxxxx
		`,
		tlw.Storage_MMC,
		StorageStateCritical,
	},
	{
		"MMC Type, StorageStateWarning, no error",
		`
		xxxxxx
		Extended CSD rev 1.7 (MMC 5.0)
		PRE_EOL_INFO: 0x02
		DEVICE_LIFE_TIME_EST_TYP_A: 0x01
		xxxxxx
		`,
		tlw.Storage_MMC,
		StorageStateWarning,
	},
	{
		"MMC Type, StorageStateNormal, no error",
		`
		xxxxxx
		Extended CSD rev 1.7 (MMC 5.0)
		PRE_EOL_INFO: 0x01
		DEVICE_LIFE_TIME_EST_TYP_A: 0x01
		xxxxxx
		`,
		tlw.Storage_MMC,
		StorageStateNormal,
	},
	{
		"NVME Type, StorageStateWarning, no error",
		`
		xxxxxx
		SMART/Health Information (NVMe Log 0x02, NSID 0xffffffff)
		Percentage Used:         100%
		xxxxxx
		`,
		tlw.Storage_NVME,
		StorageStateWarning,
	},
	{
		"NVME Type, StorageStateNormal, no error",
		`
		xxxxxx
		SMART/Health Information (NVMe Log 0x02, NSID 0xffffffff)
		Percentage Used:         90%
		xxxxxx
		`,
		tlw.Storage_NVME,
		StorageStateNormal,
	},
	{
		"UFS Type, StorageStateCritical, no error",
		`
		xxxxxx
		$ ufs-utils desc -a -p /dev/bsg/ufs-bsg0
		Device Health Descriptor: [Byte offset 0x2]: bPreEOLInfo = 0x3
		Device Health Descriptor: [Byte offset 0x3]: bDeviceLifeTimeEstA = 0x1
		xxxxxx
		`,
		tlw.Storage_UFS,
		StorageStateCritical,
	},
	{
		"UFS Type, StorageStateWarning, no error",
		`
		xxxxxx
		$ ufs-utils desc -a -p /dev/bsg/ufs-bsg0
		Device Health Descriptor: [Byte offset 0x2]: bPreEOLInfo = 0x2
		Device Health Descriptor: [Byte offset 0x3]: bDeviceLifeTimeEstA = 0x1
		xxxxxx
		`,
		tlw.Storage_UFS,
		StorageStateWarning,
	},
	{
		"UFS Type, StorageStateNormal, no error",
		`
		xxxxxx
		$ ufs-utils desc -a -p /dev/bsg/ufs-bsg0
		Device Health Descriptor: [Byte offset 0x2]: bPreEOLInfo = 0x1
		Device Health Descriptor: [Byte offset 0x3]: bDeviceLifeTimeEstA = 0x1
		xxxxxx
		`,
		tlw.Storage_UFS,
		StorageStateNormal,
	},
}

func TestStorageSMARTFieldValue(t *testing.T) {
	t.Parallel()
	for _, tt := range storageSMARTFieldValueTests {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			actualType, actualState, err := storageSMARTFieldValue(ctx, tt.rawOutput)
			if err != nil {
				t.Errorf("Expected no error")
			}
			if tt.expectedType != actualType {
				t.Errorf("Expected storage type: %q, got: %q", tt.expectedType, actualType)
			}
			if tt.expectedState != actualState {
				t.Errorf("Expected storage state: %q, got: %q", tt.expectedState, actualState)
			}
		})
	}
}

func TestExtractStorageType(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ftt.Run("SSD Type, no error", t, func(t *ftt.Test) {
		storageInfoSlice := []string{
			"xxxxxx",
			"SATA Version is: SATA 3.1, 6.0 Gb/s (current: 6.0 Gb/s)",
			"xxxxxx",
		}
		typeOfStorage, err := extractStorageType(ctx, storageInfoSlice)
		if err != nil {
			t.Errorf("Expected no error")
		}
		if typeOfStorage != tlw.Storage_SSD {
			t.Errorf("Expected storage type: %q, got: %q", tlw.Storage_SSD, typeOfStorage)
		}
	})
	ftt.Run("MMC Type, no error", t, func(t *ftt.Test) {
		storageInfoSlice := []string{
			"xxxxxx",
			"Extended CSD rev 1.7 (MMC 5.0)",
			"xxxxxx",
		}
		typeOfStorage, err := extractStorageType(ctx, storageInfoSlice)
		if err != nil {
			t.Errorf("Expected no error")
		}
		if typeOfStorage != tlw.Storage_MMC {
			t.Errorf("Expected storage type: %q, got: %q", tlw.Storage_MMC, typeOfStorage)
		}
	})
	ftt.Run("NVME Type, no error", t, func(t *ftt.Test) {
		storageInfoSlice := []string{
			"xxxxxx",
			"SMART/Health Information (NVMe Log 0x02, NSID 0xffffffff)",
			"xxxxxx",
		}
		typeOfStorage, err := extractStorageType(ctx, storageInfoSlice)
		if err != nil {
			t.Errorf("Expected no error")
		}
		if typeOfStorage != tlw.Storage_NVME {
			t.Errorf("Expected storage type: %q, got: %q", tlw.Storage_NVME, typeOfStorage)
		}
	})
	ftt.Run("Undefined Type, no error", t, func(t *ftt.Test) {
		storageInfoSlice := []string{
			"?????",
		}
		typeOfStorage, err := extractStorageType(ctx, storageInfoSlice)
		if err != nil {
			t.Errorf("Expected no error")
		}
		if typeOfStorage != tlw.Storage_TYPE_UNSPECIFIED {
			t.Errorf("Expected storage type: %q, got: %q", tlw.Storage_TYPE_UNSPECIFIED, typeOfStorage)
		}
	})
}

func TestDetectSSDState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ftt.Run("storageStateCritical, no error", t, func(t *ftt.Test) {
		storageInfoSlice := []string{
			"xxxxxx",
			"184 End-to-End_Error   PO--CK   001   001   097    NOW  135",
			"xxxxxx",
		}
		stateOfStorage, err := detectSSDState(ctx, storageInfoSlice)
		if err != nil {
			t.Errorf("Expected no error")
		}
		if stateOfStorage != StorageStateCritical {
			t.Errorf("Expected storage state: %q, got: %q", StorageStateCritical, stateOfStorage)
		}
	})
	ftt.Run("storageStateWarning, no error", t, func(t *ftt.Test) {
		storageInfoSlice := []string{
			"7 Reallocated_Sector_Ct   PO--CK   101   001   097",
			"xxxxxx",
		}
		stateOfStorage, err := detectSSDState(ctx, storageInfoSlice)
		if err != nil {
			t.Errorf("Expected no error")
		}
		if stateOfStorage != StorageStateWarning {
			t.Errorf("Expected storage state: %q, got: %q", StorageStateWarning, stateOfStorage)
		}
	})
	ftt.Run("storageStateNormal, no error", t, func(t *ftt.Test) {
		storageInfoSlice := []string{
			"yyyyyy",
			"xxxxxx",
		}
		stateOfStorage, err := detectSSDState(ctx, storageInfoSlice)
		if err != nil {
			t.Errorf("Expected no error")
		}
		if stateOfStorage != StorageStateNormal {
			t.Errorf("Expected storage state: %q, got: %q", StorageStateNormal, stateOfStorage)
		}
	})
}

func TestDetectMMCState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ftt.Run("StorageStateCritical, no error", t, func(t *ftt.Test) {
		storageInfoSlice := []string{
			"xxxxxx",
			"PRE_EOL_INFO: 0x03",
			"DEVICE_LIFE_TIME_EST_TYP_A: 0x01",
			"xxxxxx",
		}
		stateOfStorage, err := detectMMCState(ctx, storageInfoSlice)
		if err != nil {
			t.Errorf("Expected no error")
		}
		if stateOfStorage != StorageStateCritical {
			t.Errorf("Expected storage state: %q, got: %q", StorageStateCritical, stateOfStorage)
		}
	})
	ftt.Run("StorageStateWarning, no error", t, func(t *ftt.Test) {
		storageInfoSlice := []string{
			"xxxxxx",
			"PRE_EOL_INFO: 0x02",
			"DEVICE_LIFE_TIME_EST_TYP_A: 0x01",
			"xxxxxx",
		}
		stateOfStorage, err := detectMMCState(ctx, storageInfoSlice)
		if err != nil {
			t.Errorf("Expected no error")
		}
		if stateOfStorage != StorageStateWarning {
			t.Errorf("Expected storage state: %q, got: %q", StorageStateWarning, stateOfStorage)
		}
	})
	ftt.Run("StorageStateNormal, no error", t, func(t *ftt.Test) {
		storageInfoSlice := []string{
			"xxxxxx",
			"PRE_EOL_INFO: 0x01",
			"DEVICE_LIFE_TIME_EST_TYP_A: 0x01",
			"xxxxxx",
		}
		stateOfStorage, err := detectMMCState(ctx, storageInfoSlice)
		if err != nil {
			t.Errorf("Expected no error")
		}
		if stateOfStorage != StorageStateNormal {
			t.Errorf("Expected storage state: %q, got: %q", StorageStateNormal, stateOfStorage)
		}
	})
	ftt.Run("StorageStateNormal, no error", t, func(t *ftt.Test) {
		storageInfoSlice := []string{
			"xxxxxx",
			"PRE_EOL_INFO: 0x00",
			"DEVICE_LIFE_TIME_EST_TYP_A: 0x02",
			"xxxxxx",
		}
		stateOfStorage, err := detectMMCState(ctx, storageInfoSlice)
		if err != nil {
			t.Errorf("Expected no error")
		}
		if stateOfStorage != StorageStateNormal {
			t.Errorf("Expected storage state: %q, got: %q", StorageStateNormal, stateOfStorage)
		}
	})
	ftt.Run("StorageStateNormal, no error", t, func(t *ftt.Test) {
		storageInfoSlice := []string{
			"xxxxxx",
			"DEVICE_LIFE_TIME_EST_TYP_A: 0x02",
			"xxxxxx",
		}
		stateOfStorage, err := detectMMCState(ctx, storageInfoSlice)
		if err != nil {
			t.Errorf("Expected no error")
		}
		if stateOfStorage != StorageStateNormal {
			t.Errorf("Expected storage state: %q, got: %q", StorageStateNormal, stateOfStorage)
		}
	})
	ftt.Run("StorageStateWarning, no error", t, func(t *ftt.Test) {
		storageInfoSlice := []string{
			"xxxxxx",
			"DEVICE_LIFE_TIME_EST_TYP_A: 0x09",
			"xxxxxx",
		}
		stateOfStorage, err := detectMMCState(ctx, storageInfoSlice)
		if err != nil {
			t.Errorf("Expected no error")
		}
		if stateOfStorage != StorageStateWarning {
			t.Errorf("Expected storage state: %q, got: %q", StorageStateWarning, stateOfStorage)
		}
	})
	ftt.Run("StorageStateCritical, no error", t, func(t *ftt.Test) {
		storageInfoSlice := []string{
			"xxxxxx",
			"DEVICE_LIFE_TIME_EST_TYP_A: 0x0a",
			"xxxxxx",
		}
		stateOfStorage, err := detectMMCState(ctx, storageInfoSlice)
		if err != nil {
			t.Errorf("Expected no error")
		}
		if stateOfStorage != StorageStateWarning {
			t.Errorf("Expected storage state: %q, got: %q", StorageStateWarning, stateOfStorage)
		}
	})
	ftt.Run("StorageStateCritical, no error", t, func(t *ftt.Test) {
		storageInfoSlice := []string{
			"xxxxxx",
			"DEVICE_LIFE_TIME_EST_TYP_A: 0x0b",
			"xxxxxx",
		}
		stateOfStorage, err := detectMMCState(ctx, storageInfoSlice)
		if err != nil {
			t.Errorf("Expected no error")
		}
		if stateOfStorage != StorageStateWarning {
			t.Errorf("Expected storage state: %q, got: %q", StorageStateWarning, stateOfStorage)
		}
	})
}

func TestDetectNVMEState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ftt.Run("StorageStateWarning, no error", t, func(t *ftt.Test) {
		storageInfoSlice := []string{
			"xxxxxx",
			"Percentage Used:         100%",
			"xxxxxx",
		}
		stateOfStorage, err := detectNVMEState(ctx, storageInfoSlice)
		if err != nil {
			t.Errorf("Expected no error")
		}
		if stateOfStorage != StorageStateWarning {
			t.Errorf("Expected storage state: %q, got: %q", StorageStateWarning, stateOfStorage)
		}
	})
	ftt.Run("StorageStateNormal, no error", t, func(t *ftt.Test) {
		storageInfoSlice := []string{
			"xxxxxx",
			"Percentage Used:         90%",
			"xxxxxx",
		}
		stateOfStorage, err := detectNVMEState(ctx, storageInfoSlice)
		if err != nil {
			t.Errorf("Expected no error")
		}
		if stateOfStorage != StorageStateNormal {
			t.Errorf("Expected storage state: %q, got: %q", StorageStateNormal, stateOfStorage)
		}
	})
	ftt.Run("StorageStateNormal, no error", t, func(t *ftt.Test) {
		storageInfoSlice := []string{
			"xxxxxx",
			"Percentage Used:         0%",
			"xxxxxx",
		}
		stateOfStorage, err := detectNVMEState(ctx, storageInfoSlice)
		if err != nil {
			t.Errorf("Expected no error")
		}
		if stateOfStorage != StorageStateNormal {
			t.Errorf("Expected storage state: %q, got: %q", StorageStateNormal, stateOfStorage)
		}
	})
	ftt.Run("StorageStateNormal, no error", t, func(t *ftt.Test) {
		storageInfoSlice := []string{
			"xxxxxx",
			"xxxxxx",
		}
		stateOfStorage, err := detectNVMEState(ctx, storageInfoSlice)
		if err != nil {
			t.Errorf("Expected no error")
		}
		if stateOfStorage != StorageStateNormal {
			t.Errorf("Expected storage state: %q, got: %q", StorageStateNormal, stateOfStorage)
		}
	})
}
