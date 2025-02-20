// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/maruel/subcommands"
	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/infra/libs/vmlab"
	"go.chromium.org/infra/libs/vmlab/api"
)

var ImageCmd = &subcommands.Command{
	UsageLine: "image",
	ShortDesc: "import a VM image from GCS to GCE",
	CommandRun: func() subcommands.CommandRun {
		c := &imageRun{}
		c.imageFlags.register(&c.Flags)
		return c
	},
}

type imageFlags struct {
	buildPath string
	wait      bool
	json      bool
}

func (c *imageFlags) register(f *flag.FlagSet) {
	f.StringVar(&c.buildPath, "build-path", "", "Build path of the image. For example \"betty-arc-r-release/R108-15178.0.0\".")
	f.BoolVar(&c.wait, "wait", false, "Wait for image import to complete. Default is not to wait.")
	f.BoolVar(&c.json, "json", false, "Output json result.")
}

type imageRun struct {
	subcommands.CommandRunBase
	imageFlags
}

func (c *imageRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if c.imageFlags.buildPath == "" {
		fmt.Fprintln(os.Stderr, "Build path must be set.")
		return 1
	}

	imageApi, err := vmlab.NewImageApi(api.ProviderId_CLOUDSDK)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot get image api provider: %v", err)
	}

	gceImage, err := imageApi.GetImage(c.imageFlags.buildPath, c.imageFlags.wait)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to import image: %v\n", err)
		return 1
	}

	if c.imageFlags.json {
		if instanceJson, err := protojson.Marshal(gceImage); err != nil {
			fmt.Fprintf(os.Stderr, "BUG! Cannot convert output to json: %v\n", err)
		} else {
			fmt.Println(string(instanceJson))
		}
	} else {
		fmt.Printf("Image named %s created at project %s, status %s\n", gceImage.Name, gceImage.Project, gceImage.Status.String())
	}
	return 0
}
