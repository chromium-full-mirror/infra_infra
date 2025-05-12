// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This package contains constants for recoverylib, including task names for example.
// For more information, see b:208688399.
package buildbucket

import (
	"errors"
	"fmt"
	"strings"
)

// TaskName describes which flow/plans will be involved in the process.
type TaskName string

const (
	InvalidTaskName TaskName = ""
	// Audit is used to run audit task of RPM.
	AuditRPM TaskName = "audit_rpm"
	// Audit is used to run audit task of internal storage.
	AuditStorage TaskName = "audit_storage"
	// Audit is used to run audit task of USB drive.
	AuditUSB TaskName = "audit_usb"
	// Task used to run verify flow in the lab.
	// The verify flow is more lighter that recovery/repair flows.
	Verify TaskName = "verify"
	// Task used to run auto recovery/repair flow in the lab.
	Recovery TaskName = "recovery"
	// Task used to run deep repair flow in the lab.
	DeepRecovery TaskName = "deep_recovery"
	// Task used to run auto recovery/repair flow in MH.
	MHRecovery TaskName = "mh_recovery"
	// Task used to prepare device to be used in the lab.
	Deploy TaskName = "deploy"
	// Task used to prepare devices to be used in the MH.
	MHDeploy TaskName = "mh_deploy"
	// Task used to execute custom plans.
	// Configuration has to be provided by the user.
	Custom TaskName = "custom"
	// DryRun is a task that runs an empty plan with no actions.
	// Its intended use case is to verify that a recipe or luciexe executable
	// can transfer control to labpack (or another recoverylib runner) successfully.
	DryRun TaskName = "dry_run"
	// PostTest is a task that runs a lightweight verifier task during test
	// teardown. Also known as "ile-de-france".
	PostTest = "post_test"
)

// String returns the name of the task as an argument to the labpack command-line tool.
func (tn TaskName) String() string {
	return string(tn)
}

// BuilderName returns builder-name specified per TaskName.
func (tn TaskName) BuilderName() string {
	return TaskNameToBuilderNamePerVersion(tn, CIPDProd)
}

// NormalizeTaskName takes a task name from anywhere and normalizes it.
// This is a necessary first step towards consolidating our notion of task names.
//
// Names are taken from here and https://chromium.googlesource.com/infra/infra/+/refs/heads/main/go/src/infra/appengine/crosskylabadmin/internal/app/frontend/tracker.go .
func NormalizeTaskName(name string) (TaskName, error) {
	switch strings.ToLower(name) {
	case "verify-servo-usb-drive", "usb-drive", "audit-usb", "audit_usb":
		return AuditUSB, nil
	case "verify-dut-storage", "storage", "audit-storage", "audit_storage":
		return AuditStorage, nil
	case "verify-rpm-config", "rpm config", "audit-rpm", "audit_rpm":
		return AuditRPM, nil
	case "repair", "recovery":
		return Recovery, nil
	case "verify", "check-only":
		return Verify, nil
	case "mhrepair", "mh_recovery", "mh-recovery", "mh-repair", "mh_repair":
		return MHRecovery, nil
	case "deep-repair", "deep_repair":
		return DeepRecovery, nil
	case "deploy":
		return Deploy, nil
	case "mh-deploy", "mh_deploy", "mhdeploy":
		return MHDeploy, nil
	case "dry_run", "dry-run":
		return DryRun, nil
	case "custom":
		return Custom, nil
	}
	return InvalidTaskName, fmt.Errorf("normalize task name: unrecognized task name %q", name)
}

// ValidateTaskName checks whether a task name is valid
func ValidateTaskName(tn TaskName) error {
	if tn == "" {
		return errors.New("validate task name: task name cannot be empty")
	}
	switch tn {
	case AuditRPM:
	case AuditStorage:
	case AuditUSB:
	case Recovery:
	case MHRecovery:
	case DeepRecovery:
	case Deploy:
	case MHDeploy:
	case Custom:
	default:
		return fmt.Errorf("validate task name: %q is not a valid task name", tn)
	}
	return nil
}

// TaskNameToBuilderNamePerVersion returns builder-name specified per TaskName and CIPDVersion.
// By default any unknown task will be treated as custom tasks.
func TaskNameToBuilderNamePerVersion(tn TaskName, v CIPDVersion) string {
	switch tn {
	case AuditRPM:
		if v == CIPDLatest {
			return "audit-rpm-latest"
		}
		return "audit-rpm"
	case AuditStorage:
		if v == CIPDLatest {
			return "audit-storage-latest"
		}
		return "audit-storage"
	case AuditUSB:
		if v == CIPDLatest {
			return "audit-servo-usb-key-latest"
		}
		return "audit-servo-usb-key"
	case Recovery, MHRecovery, DeepRecovery:
		if v == CIPDLatest {
			return "repair-latest"
		}
		return "repair"
	case Deploy:
		if v == CIPDLatest {
			return "deploy-latest"
		}
		return "deploy"
	case MHDeploy:
		if v == CIPDLatest {
			return "mh_deploy-latest"
		}
		return "mh_deploy"
	default:
		if v == CIPDLatest {
			return "custom-latest"
		}
		return "custom"
	}
}
