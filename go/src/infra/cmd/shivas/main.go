// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"io/ioutil"
	"log"
	"os"

	"github.com/maruel/subcommands"

	"infra/cmd/shivas/clilib"
)

func main() {
	log.SetOutput(ioutil.Discard)
	os.Exit(subcommands.Run(clilib.Application(), nil))
}
