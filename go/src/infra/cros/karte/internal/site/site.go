// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package site

import (
	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/api/gitiles"
	"go.chromium.org/luci/hardcoded/chromeinfra"
)

const progName = "karte"

// EventsDataset is the name of the events dataset in bigquery for Karte.
const EventsDatasetName = "events"

// EventsTable is the name of the events table in the events dataset.
const EventsTableName = "events_table"

// DefaultCLIKarteServer is the default server that the karte command line tool talks to.
// The Karte commands are *not* exclusively readonly, therefore we should default to talking
// to the dev instance rather than the prod instance.
//
// TODO(gregorynisbet): Add non readonly commands to karte CLI.
const DefaultCLIKarteServer = DevKarteServer

// DevKarteServer is the dev cloud project for Karte.
const DevKarteServer = "chrome-fleet-karte-dev.appspot.com"

// ProdKarteServer is the prod cloud project for Karte.
const ProdKarteServer = "chrome-fleet-karte.appspot.com"

// DefaultAuthOptions is an auth.Options struct prefilled with chrome-infra
// defaults.
var DefaultAuthOptions auth.Options

func init() {
	DefaultAuthOptions = chromeinfra.DefaultAuthOptions()
	DefaultAuthOptions.Scopes = []string{auth.OAuthScopeEmail, gitiles.OAuthScope}
}
