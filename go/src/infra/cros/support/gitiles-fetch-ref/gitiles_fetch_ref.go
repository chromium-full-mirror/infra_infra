// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"log"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/api/gerrit"

	"go.chromium.org/infra/cros/support/internal/cli"
	sgerrit "go.chromium.org/infra/cros/support/internal/gerrit"
)

type Input struct {
	Branch sgerrit.Branch `json:"branch"`
}

type Output struct {
	Branch sgerrit.Branch `json:"branch"`
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
	branch := input.Branch

	branch = sgerrit.MustFetchBranch(cli.Context, httpClient, branch)
	cli.MustMarshalOutput(Output{Branch: branch})
}
