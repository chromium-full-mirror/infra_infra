// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devicesdb

type DeviceMetricsDAO struct {
	Total            int32
	Leased           int32
	Available        int32
	Ready            int32
	NeedManualRepair int32
	NeedRepair       int32
	RepairFailed     int32
}
