// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ufsclient is the client lib for UFS.
package ufsclient

import (
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
)

const (
	// UfsDevURL is the URL of the dev ufs instance.
	UfsDevURL = "staging.ufs.api.cr.dev"
	// UfsProdURL is the URL of the prod ufs instance.
	UfsProdURL = "ufs.api.cr.dev"
	// UfsPort is the port for ufs.
	UfsPort = 443
)

// Client is a client for UFS.
type Client = ufsAPI.FleetClient
