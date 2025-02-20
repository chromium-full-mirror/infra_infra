// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"context"
	"log"
	"os"
	"time"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/api/gerrit"

	"go.chromium.org/infra/cros/support/internal/cli"
	sgerrit "go.chromium.org/infra/cros/support/internal/gerrit"
	"go.chromium.org/infra/cros/support/internal/git"
)

type Input struct {
	TempDir       string          `json:"temp_dir"`
	GerritChanges sgerrit.Changes `json:"gerrit_changes"`
}

type Output struct {
	Errors []string `json:"errors"`
}

func main() {
	cli.SetAuthScopes(auth.OAuthScopeEmail, gerrit.OAuthScope)
	cli.Init()

	httpClient, err := cli.AuthenticatedHTTPClient()
	if err != nil {
		log.Fatal(err)
	}

	var input Input
	cli.MustUnmarshalInput(&input)

	ctx, cancel := context.WithTimeout(cli.Context, 15*time.Minute)
	defer cancel()

	output := &Output{}
	if input.TempDir == "" {
		input.TempDir = os.TempDir()
	}
	if err := os.MkdirAll(input.TempDir, 0755); err != nil {
		// Will only happen if parentDir isn't already a dir and it failed to be created.
		log.Fatalf("%s is not a directory", input.TempDir)
	}
	errs := git.CheckCherryPick(ctx, httpClient, input.TempDir, input.GerritChanges)
	for i, err := range errs {
		log.Printf("Error %d/%d\n%s", i+1, len(errs), err.Error())
		output.Errors = append(output.Errors, err.Error())
	}
	cli.MustMarshalOutput(output)
}
