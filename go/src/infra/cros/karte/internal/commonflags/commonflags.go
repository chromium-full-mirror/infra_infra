// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commonflags

import (
	"flag"
	"fmt"

	"go.chromium.org/luci/auth"

	"go.chromium.org/infra/cros/karte/client"
)

// Flags are the flags shared by most Karte commands.
type Flags struct {
	// Environment selection
	Env string
}

// Register registers the common flags.
func (flags *Flags) Register(f *flag.FlagSet) {
	if f == nil {
		panic("common flags cannot be nil")
	}
	if flags == nil {
		panic("flagset cannot be nil")
	}
	f.StringVar(&flags.Env, "env", "local", `choose the Karte service, valid values are local|dev|prod`)
}

// MustSelectKarteConfig selects the a Karte config corresponding to the environment flag.
func (flags *Flags) MustSelectKarteConfig(o auth.Options) *client.Config {
	switch flags.Env {
	case "", "local":
		return client.LocalConfig(o)
	case "dev":
		return client.DevConfig(o)
	case "prod":
		return client.ProdConfig(o)
	default:
		panic(fmt.Sprintf("unrecognized environment %q", flags.Env))
	}
}
