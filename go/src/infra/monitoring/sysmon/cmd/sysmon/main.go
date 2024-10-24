// Copyright (c) 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"os"
	"time"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/common/tsmon"

	"infra/cmdsupport/service"
	"infra/monitoring/sysmon/android"
	"infra/monitoring/sysmon/cipd"
	"infra/monitoring/sysmon/cros"
	"infra/monitoring/sysmon/docker"
	"infra/monitoring/sysmon/puppet"
	"infra/monitoring/sysmon/system"
)

func main() {
	fs := flag.NewFlagSet("", flag.ExitOnError)

	tsmonFlags := tsmon.NewFlags()
	tsmonFlags.Flush = "auto"
	tsmonFlags.Register(fs)

	loggingConfig := logging.Config{Level: logging.Info}
	loggingConfig.AddFlags(fs)

	// This flag is mainly useful in tests.
	exitAfter := fs.Duration("sysmon-exit-after", 0, "If set, makes sysmon run for the given duration only")

	_ = fs.Parse(os.Args[1:])

	ctx := context.Background()
	ctx = gologger.StdConfig.Use(ctx)
	ctx = loggingConfig.Set(ctx)

	os.Exit(run(ctx, &tsmonFlags, *exitAfter))
}

func run(ctx context.Context, flags *tsmon.Flags, exitAfter time.Duration) int {
	if err := tsmon.InitializeFromFlags(ctx, flags); err != nil {
		logging.Errorf(ctx, "Failed to initialize tsmon: %s", err)
		return 1
	}
	if flags.Flush != tsmon.FlushAuto {
		logging.Errorf(ctx, "Sysmon supports only -ts-mon-flush=auto")
		return 1
	}

	// Register metric callbacks.
	android.Register()
	cipd.Register()
	cros.Register()
	docker.Register()
	puppet.Register()
	system.Register()

	// Flush initial values of all metrics (in particular 0s for all counters).
	_ = tsmon.Flush(ctx)
	// Do the final flush before exiting.
	defer tsmon.Shutdown(ctx)

	if exitAfter > 0 {
		// This is useful in tests.
		time.Sleep(exitAfter)
		return 0
	}

	// Support running as a Windows Service, or as a regular background process.
	stop := make(chan struct{})
	s := &service.Service{
		Start: func() int {
			// tsmon's auto-flusher goroutine will call the metric callbacks and
			// flush the metrics every minute.
			<-stop
			return 0
		},
		Stop: func() {
			close(stop)
		},
	}
	return service.Run(s)
}
