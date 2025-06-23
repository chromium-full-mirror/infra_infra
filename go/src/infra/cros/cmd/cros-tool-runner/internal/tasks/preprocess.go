// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tasks

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/golang/protobuf/jsonpb"
	"github.com/maruel/subcommands"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/cros-tool-runner/internal/common"
	"go.chromium.org/infra/cros/cmd/cros-tool-runner/internal/preprocess"
)

type preProcessCmd struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags

	inputPath     string
	outputPath    string
	imagesPath    string
	dockerKeyFile string
}

// PreProcess execute pre-process to find tests.
func PreProcess(authOpts auth.Options) *subcommands.Command {

	c := &preProcessCmd{}
	return &subcommands.Command{
		UsageLine: "pre-process -images md_container.jsonpb -input input.json -output output.json",
		ShortDesc: "pre-process runs the given pre-process commands.",
		CommandRun: func() subcommands.CommandRun {
			c.authFlags.Register(&c.Flags, authOpts)
			// Used to provide input by files.
			c.Flags.StringVar(&c.inputPath, "input", "", "The input file contains a jsonproto representation of pre-process requests (CrosToolRunnerPreTestRequest)")
			c.Flags.StringVar(&c.outputPath, "output", "", "The output file contains a jsonproto representation of pre-process responses (CrosToolRunnerPreTestResponse)")
			c.Flags.StringVar(&c.imagesPath, "images", "", "The input file contains a jsonproto representation of containers metadata (ContainerMetadata)")
			c.Flags.StringVar(&c.dockerKeyFile, "docker_key_file", "", "The input file contains the docker auth key")
			return c
		},
	}
}

// Run executes the tool.
func (c *preProcessCmd) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)

	out, err := c.innerRun(ctx, a, args, env)
	// Unexpected error will counted as incorrect request data.
	// all expected cases has to generate responses.
	if err != nil {
		log.Printf("Failed in running pre-process: %s", err)
		return 1
	}
	if err := savePreProcessOutput(out, c.outputPath); err != nil {
		log.Printf("Failed to save pre-process output: %s", err)
	}
	return 0
}

func (c *preProcessCmd) innerRun(ctx context.Context, a subcommands.Application, args []string, env subcommands.Env) (*api.CrosToolRunnerPreTestResponse, error) {
	ctx, err := useSystemAuth(ctx, &c.authFlags)
	if err != nil {
		return nil, errors.Annotate(err, "inner run: read system auth").Err()
	}
	req, err := readPreProcessRequest(c.inputPath)
	if err != nil {
		return nil, errors.Annotate(err, "inner run: failed to read pre-process request").Err()
	}

	cm, err := readContainersMetadata(c.imagesPath)
	if err != nil {
		return nil, errors.Annotate(err, "inner run: failed to read containter metadata").Err()
	}
	lookupKey := req.ContainerMetadataKey

	preProcessContainer, err := findContainer(cm, lookupKey, preprocess.PreProcessName)
	if err != nil {
		return nil, errors.Annotate(err, "inner run: failed to find container").Err()
	}
	result, err := preprocess.Run(ctx, req, preProcessContainer, c.dockerKeyFile)
	return result, errors.WrapIf(err, "inner run: failed to find tests")
}

// readPreProcessRequest reads the jsonproto at path input request data.
func readPreProcessRequest(p string) (*api.CrosToolRunnerPreTestRequest, error) {
	in := &api.CrosToolRunnerPreTestRequest{}
	r, err := os.Open(p)
	if err != nil {
		return nil, errors.Annotate(err, "inner run: read pre-process request %q", p).Err()
	}

	umrsh := common.JsonPbUnmarshaler()
	err = umrsh.Unmarshal(r, in)
	return in, errors.WrapIf(err, "inner run: read pre-process request %q", p)
}

// savePreProcessOutput saves output data to the file.
func savePreProcessOutput(out *api.CrosToolRunnerPreTestResponse, outputPath string) error {
	if outputPath != "" && out != nil {
		dir := filepath.Dir(outputPath)
		// Create the directory if it doesn't exist.
		if err := os.MkdirAll(dir, 0777); err != nil {
			return errors.Annotate(err, "save pre-process output: failed to create directory while saving output").Err()
		}
		f, err := os.Create(outputPath)
		if err != nil {
			return errors.Annotate(err, "save pre-process output: failed to create file while saving output").Err()
		}
		defer f.Close()
		marshaler := jsonpb.Marshaler{}
		if err := marshaler.Marshal(f, out); err != nil {
			return errors.Annotate(err, "save pre-process output: failed to marshal result while saving output").Err()
		}
		if err := f.Close(); err != nil {
			return errors.Annotate(err, "save pre-process output: failed to close file while saving output").Err()
		}
	}
	return nil
}
