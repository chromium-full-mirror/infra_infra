// Copyright 2024 The LUCI Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Command logstreamer is a thin wrapper around gs.NewObjectStream to tail a GCS
// object which is being repeatedly appended via the composition API and dumps
// it to stdout.
//
// Run it like:
//
//	logstreamer gs://bucket/path/to/object
//
// Uses current user's LUCI credentials (i.e. `luci-auth login`) and will prompt
// for interactive login if there isn't already cached credentials for the
// correct scope.
//
// Send SIGINT (Ctrl-C) to quit.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/gcloud/gs"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/common/system/signals"
	"go.chromium.org/luci/hardcoded/chromeinfra"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	ctx = gologger.StdConfig.Use(ctx)

	stopper := signals.HandleInterrupt(cancel)
	defer stopper()

	opts := chromeinfra.DefaultAuthOptions()
	opts.Scopes = []string{
		auth.OAuthScopeEmail,
		"https://www.googleapis.com/auth/devstorage.read_only",
	}
	authn := auth.NewAuthenticator(ctx, auth.InteractiveLogin, opts)
	transp, err := authn.Transport()
	if err != nil {
		logging.Errorf(ctx, "getting auth transport: %s", err)
		os.Exit(1)
	}
	lines := make(chan string)
	errs := make(chan error)

	gcs, err := gs.NewProdClient(ctx, transp)
	if err != nil {
		logging.Errorf(ctx, "making GCS client: %s", err)
		os.Exit(1)
	}

	stream := gs.NewObjectStream(ctx, &gs.ObjectStreamOptions{
		Client:        gcs,
		Path:          gs.Path(os.Args[1]),
		PollFrequency: 2 * time.Second,
		LinesC:        lines,
		ErrorsC:       errs,
	})
	defer stream.Close()

	for {
		select {
		case l := <-lines:
			fmt.Fprintln(os.Stdout, l)
		case err := <-errs:
			log.Print(err)
		case <-ctx.Done():
			os.Exit(0)
		}
	}
}
