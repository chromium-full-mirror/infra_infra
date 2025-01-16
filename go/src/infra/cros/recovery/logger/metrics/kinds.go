// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package metrics

const (
	// Cr50FwReflashKind is the name/kind in the karte metrics
	// used for query or update the cr 50 fw reflash information.
	Cr50FwReflashKind = "cr50_flash"
	// PerResourceTaskKindGlob is the template for the name/kind for
	// query the complete record for each task for each resource.
	PerResourceTaskKindGlob = "run_task_%s"
	// RunLibraryKind is the actionKind for query the
	// record for overall PARIS recovery result each run.
	RunLibraryKind = "run_recovery"
	// ServoEachDeviceFwUpdateKind is the actionkind for query the record for each
	// of the servo device's fw update information.
	ServoEachDeviceFwUpdateKind = "servo_firmware_update_%s"
	// USBDriveDetectionKind is the actionKind for query the
	// record for the DUT's servo's USB-drive.
	USBDriveDetectionKind = "servo_usbdrive_detection"
	// USBDriveReplacedKind is the actionKind for query the
	// record for DUT's old/replaced USB-drive.
	USBDriveReplacedKind = "servo_usbdrive_replaced_detection"
	// BadBlocksROExecutionKind is an actionKind that indicates the RO
	// badblocks has been executed.
	BadBlocksROExecutionKind = "backblocks_ro_execution"
	// BadBlocksRWExecutionKind is an actionKind that indicates the RW
	// badblocks has been executed.
	BadBlocksRWExecutionKind = "backblocks_rw_execution"
	// AuditCameraKind is an actionKind for tracking audit camera tasks.
	AuditCameraKind = "audit_camera"
)
