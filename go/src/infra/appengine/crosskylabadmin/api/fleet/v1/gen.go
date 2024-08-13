// Copyright 2018 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:generate cproto -proto-path ../../../../../../go.chromium.org/chromiumos/infra/proto/src -proto-path ../../../../../..

//go:generate mockgen -source tracker.pb.go -destination tracker.mock.pb.go -package fleet
//go:generate mockgen -source inventory.pb.go -destination inventory.mock.pb.go -package fleet

//go:generate svcdec -type TrackerServer
//go:generate svcdec -type InventoryServer

// Package fleet contains service definitions for fleet management in
// crosskylabadmin.
package fleet
