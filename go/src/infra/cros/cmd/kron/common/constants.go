// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package common has utilities that are not context specific and can be used by
// all packages.
package common

import (
	"time"
)

const (
	Day       = 24 * time.Hour
	Week      = 7 * Day
	Fortnight = 14 * Day

	DefaultHoursAhead = -1 * time.Hour
	DefaultInt64      = int64(-1)
	DefaultString     = ""

	// This is the value mapping that legacy SuiteScheduler uses for days in
	// weekly and Fortnightly configs.

	Monday                     = 0
	Tuesday                    = 1
	Wednesday                  = 2
	Thursday                   = 3
	Friday                     = 4
	Saturday                   = 5
	Sunday                     = 6
	FortnightlySecondMonday    = 7
	FortnightlySecondTuesday   = 8
	FortnightlySecondWednesday = 9
	FortnightlySecondThursday  = 10
	FortnightlySecondFriday    = 11
	FortnightlySecondSaturday  = 12
	FortnightlySecondSunday    = 13

	SuiteSchedulerCfgURL = "https://chrome-internal.googlesource.com/chromeos/config-internal/+/refs/heads/main/test/suite_scheduler/generated/suite_scheduler.cfg?format=text"
	LabCfgURL            = "https://chrome-internal.googlesource.com/chromeos/config-internal/+/refs/heads/main/test/suite_scheduler/generated/lab_config.cfg?format=text"
	SuiteSchedulerIniURL = "https://chrome-internal.googlesource.com/chromeos/config-internal/+/refs/heads/main/test/suite_scheduler/generated/suite_scheduler.ini?format=text"
	LabIniURL            = "https://chrome-internal.googlesource.com/chromeos/config-internal/+/refs/heads/main/test/suite_scheduler/generated/lab_config.ini?format=text"

	TotFileURL = "https://chromium.googlesource.com/chromiumos/overlays/chromiumos-overlay/+/refs/heads/main/chromeos/config/chromeos_version.sh?format=text"

	StagingProjectID     = "google.com:suite-scheduler-staging"
	StagingProjectNumber = 118927920079
	ProdProjectID        = "google.com:suite-scheduler"
	ProdProjectNumber    = 542690066668

	BuildsSubscription          = "chromeos-builds-all"
	BuildsSubscriptionMultiDUT  = "chromeos-builds-all-multidut"
	BuildsSubscriptionTesting   = "chromeos-builds-all-testing"
	BuildsSubscription3d        = "chromeos-builds-all-3d"
	BuildsSubscription3dTesting = "chromeos-builds-all-3d-testing"

	BuildsPubSubTopic = "kron-builds"
	EventsPubSubTopic = "kron-events"
	RunsPubSubTopic   = "kron-runs"

	FirestoreDatabaseName         = "suite-scheduler-configs"
	FirestoreConfigCollectionName = "configs"

	// MultirequestSize is the maximum number of tests requests that we can
	// combine per CTP builder run.
	MultirequestSize = 25

	// Names are shared across environments but versions may have skew depending
	// on individual key recycling. If a different version needs to be targeted
	// then the version number will need to be updated here.

	KronWriterUsernameSecret               = "kron-writer-username"
	KronWriterUsernameSecretVersionStaging = 2
	KronWriterUsernameSecretVersionProd    = 1

	KronWriterPasswordSecret               = "kron-writer-password"
	KronWriterPasswordSecretVersionStaging = 1
	KronWriterPasswordSecretVersionProd    = 2

	KronReaderUsernameSecret               = "kron-reader-username"
	KronReaderUsernameSecretVersionStaging = 1
	KronReaderUsernameSecretVersionProd    = 1

	KronReaderPasswordSecret               = "kron-reader-password"
	KronReaderPasswordSecretVersionStaging = 1
	KronReaderPasswordSecretVersionProd    = 1

	KronBuildsDBNameSecret               = "kron-builds-dbname"
	KronBuildsDBNameSecretVersionStaging = 1
	KronBuildsDBNameSecretVersionProd    = 1

	KronBuildsConnectionNameSecret               = "kron-builds-connection-name"
	KronBuildsConnectionNameSecretVersionStaging = 1
	KronBuildsConnectionNameSecretVersionProd    = 1

	// StagingConfigsPrefix is the prefix for all configs that we will allow to
	// execute in staging. This will ensure that we are testing specific CUJs in
	// staging.
	StagingConfigsPrefix = "TSEStaging"
)
