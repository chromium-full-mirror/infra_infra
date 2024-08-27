// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"flag"
	"os"

	"github.com/google/uuid"

	"infra/appengine/chromium_build_stats/ninjalog"
)

var ninjaLog = flag.String("ninjalog", "", "")
var projectID = flag.String("project-id", "", "")

func main() {
	flag.Parse()

	buf, err := os.ReadFile(*ninjaLog)
	if err != nil {
		panic(err)
	}

	logs, err := ninjalog.Parse(*ninjaLog, bytes.NewBuffer(buf))
	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	err = ninjalog.UploadTraceOnCriticalPath(ctx, *projectID, "build", logs, uuid.New())
	if err != nil {
		panic(err)
	}
}
