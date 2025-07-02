// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"io"
	"log"
	"os"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/tsmon"
	"go.chromium.org/luci/common/tsmon/field"
	"go.chromium.org/luci/common/tsmon/metric"

	"go.chromium.org/infra/cmd/shivas/clilib"
	"go.chromium.org/infra/cmd/shivas/site"
	"go.chromium.org/infra/libs/cipd"
)

// shivasExitStatus is a metric to track exit status of shivas.
var shivasExitStatus = metric.NewInt(
	"chromeos/shivas/exit_code",
	"exit code for the shivas run",
	nil,
	field.String("version"),
)

func main() {
	log.SetOutput(io.Discard)
	exit := subcommands.Run(clilib.Application(), nil)
	ctx := context.Background()
	version := "unknown"

	p, err := cipd.FindPackage("shivas", site.CipdInstalledPath)
	if err == nil {
		version = p.Pin.InstanceID
	}

	// Attempt to initialize tsmon, update the metric if we succeed.
	if err = tsmon.InitializeFromFlags(ctx, &site.TSMonFlags); err == nil {
		shivasExitStatus.Set(ctx, int64(exit), version)
		tsmon.Flush(ctx)
	}

	os.Exit(exit)
}
