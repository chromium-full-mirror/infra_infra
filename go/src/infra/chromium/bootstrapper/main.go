// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	stderrors "errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/gologger"
	logdogbootstrap "go.chromium.org/luci/logdog/client/butlerlib/bootstrap"
	"go.chromium.org/luci/logdog/client/butlerlib/streamclient"
	"go.chromium.org/luci/lucictx"
	"go.chromium.org/luci/luciexe"

	"go.chromium.org/infra/chromium/bootstrapper/bootstrap"
	"go.chromium.org/infra/chromium/bootstrapper/clients/gclient"
	"go.chromium.org/infra/chromium/bootstrapper/clients/gerrit"
	"go.chromium.org/infra/chromium/bootstrapper/clients/gitiles"
)

type getOptionsFn func() options

func parseFlags() options {
	outputPath := flag.String("output", "", "Path to write the final build.proto state to.")
	polymorphic := flag.Bool("polymorphic", false, "Whether the builder bootstraps properties for other builders instead of itself; polymorphic builders give precedence to build properties rather than the properties in the properties file")
	propertiesOptional := flag.Bool("properties-optional", false, "Whether missing $bootstrap/properties property should be allowed")
	flag.Parse()
	return options{
		outputPath:         *outputPath,
		packagesRoot:       "packages",
		polymorphic:        *polymorphic,
		propertiesOptional: *propertiesOptional,
	}
}

func getBuild(ctx context.Context, input io.Reader) (*buildbucketpb.Build, error) {
	logging.Infof(ctx, "reading build input")
	data, err := io.ReadAll(input)
	if err != nil {
		return nil, errors.Annotate(err, "failed to read build input").Err()
	}
	logging.Infof(ctx, "unmarshalling build input")
	build := &buildbucketpb.Build{}
	if err = proto.Unmarshal(data, build); err != nil {
		return nil, errors.Annotate(err, "failed to unmarshall build").Err()
	}
	return build, nil
}

type options struct {
	outputPath         string
	packagesRoot       string
	polymorphic        bool
	propertiesOptional bool
}

type bootstrapFn func(ctx context.Context, input io.Reader, opts options) ([]string, []byte, error)

func performBootstrap(ctx context.Context, input io.Reader, opts options) ([]string, []byte, error) {
	build, err := getBuild(ctx, input)
	if err != nil {
		return nil, nil, err
	}

	logging.Infof(ctx, "creating bootstrap input")
	inputOpts := bootstrap.InputOptions{
		Polymorphic:        opts.polymorphic,
		PropertiesOptional: opts.propertiesOptional,
	}
	bootstrapInput, err := inputOpts.NewInput(build)
	if err != nil {
		return nil, nil, err
	}

	var config *bootstrap.BootstrapConfig

	var exe *bootstrap.BootstrappedExe
	var cmd []string

	// Downloading the necessary packages and getting the appropriate properties both speak to
	// external services but don't necessarily depend on each other, so use an errgroup to do
	// them in parallel

	// Introduce a new block to shadow the ctx variable so that the outer
	// value can't be used accidentally
	{
		group, ctx := errgroup.WithContext(ctx)

		// If the builder's properties are in a dependent project, getting the properties
		// might require the gclient binary from the depot_tools package, so provide a
		// channel that can be used to synchronize where necessary
		depotToolsCh := make(chan string, 1)

		group.Go(func() error {
			logging.Infof(ctx, "downloading necessary packages")
			var err error
			exe, cmd, err = bootstrap.DownloadPackages(ctx, bootstrapInput, opts.packagesRoot, map[string]chan<- string{
				bootstrap.DepotToolsId: depotToolsCh,
			})
			return errors.WrapIf(err, "failed to download necessary packages")
		})

		group.Go(func() error {
			// gclientGetter will only be called if dependency_project is set in the
			// $bootstrap/exe property, depot_tools will always be downloaded in that
			// case
			gclientGetter := func(ctx context.Context) (*gclient.Client, error) {
				var depotToolsPackagePath string
				select {
				case depotToolsPackagePath = <-depotToolsCh:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				gclientPath := filepath.Join(depotToolsPackagePath, "depot_tools", "gclient")
				return gclient.NewClient(gclientPath), nil
			}
			bootstrapper := bootstrap.NewBuildBootstrapper(gitiles.NewClient(ctx), gerrit.NewClient(ctx), gclientGetter)

			logging.Infof(ctx, "getting bootstrapped config")
			var err error
			config, err = bootstrapper.GetBootstrapConfig(ctx, bootstrapInput)
			return err
		})

		if err := group.Wait(); err != nil {
			return nil, nil, err
		}
	}

	logging.Infof(ctx, "updating build")
	err = config.UpdateBuild(build, exe)
	if err != nil {
		return nil, nil, err
	}

	logging.Infof(ctx, "marshalling bootstrapped build input")
	recipeInput, err := proto.Marshal(build)
	if err != nil {
		return nil, nil, errors.Annotate(err, "failed to marshall bootstrapped build input: <%s>", build).Err()
	}

	if opts.outputPath != "" {
		cmd = append(cmd, "--output", opts.outputPath)
	}

	return cmd, recipeInput, nil
}

type executeCmdFn func(ctx context.Context, cmd []string, input []byte) error

func executeCmd(ctx context.Context, cmd []string, input []byte) error {
	cmdCtx := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	cmdCtx.Stdin = bytes.NewBuffer(input)
	cmdCtx.Stdout = os.Stdout
	cmdCtx.Stderr = os.Stderr
	return cmdCtx.Run()
}

type getStreamFn func(ctx context.Context) (streamclient.DatagramStream, error)

func getStream(ctx context.Context) (streamclient.DatagramStream, error) {
	logging.Infof(ctx, "bootstrapping logdog")
	logdog, err := logdogbootstrap.Get()
	if err != nil {
		return nil, errors.Annotate(err, "failed to bootstrap logdog").Err()
	}

	logging.Infof(ctx, "getting datagram stream")
	stream, err := logdog.Client.NewDatagramStream(
		ctx,
		luciexe.BuildProtoStreamSuffix,
		streamclient.WithContentType(luciexe.BuildProtoContentType),
	)
	if err != nil {
		return nil, errors.Annotate(err, "failed to get datagram stream").Err()
	}

	return stream, nil

}

func handleBootstrapError(ctx context.Context, bootstrapErr error, getStream getStreamFn) {
	stream, err := getStream(ctx)
	if err != nil {
		logging.Errorf(ctx, err.Error())
		return
	}
	defer func() {
		if err := stream.Close(); err != nil {
			logging.Errorf(ctx, errors.Annotate(err, "failed to close datagram stream").Err().Error())
		}
	}()

	build := &buildbucketpb.Build{}

	writeBuild := func() error {
		outputData, err := proto.Marshal(build)
		if err != nil {
			return err
		}
		return stream.WriteDatagram(outputData)
	}

	// Write out a build setting the STARTED status to establish the links to the build logs in
	// milo in case writing out the final build fails (e.g. summary markdown is too big)
	logging.Infof(ctx, "writing out initial build")
	build.Output = &buildbucketpb.Build_Output{
		Status: buildbucketpb.Status_STARTED,
	}
	if err := writeBuild(); err != nil {
		logging.Errorf(ctx, errors.Annotate(err, "failed to write out initial build").Err().Error())
		return
	}

	if bootstrap.PatchRejected.In(bootstrapErr) {
		build.SummaryMarkdown = "<pre>Patch failure: See build stderr log. Try rebasing?</pre>"
		build.Output = &buildbucketpb.Build_Output{
			Status: buildbucketpb.Status_FAILURE,
			Properties: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"failure_type": structpb.NewStringValue("PATCH_FAILURE"),
				},
			},
		}
	} else {
		build.SummaryMarkdown = fmt.Sprintf("<pre>%s</pre>", bootstrapErr)
		build.Output = &buildbucketpb.Build_Output{
			Status: buildbucketpb.Status_INFRA_FAILURE,
		}
	}

	logging.Infof(ctx, "updating build with failure details")
	if err := writeBuild(); err != nil {
		logging.Errorf(ctx, errors.Annotate(err, "failed to update build with failure details").Err().Error())
		return
	}
}

func bootstrapMain(ctx context.Context, getOpts getOptionsFn, performBootstrap bootstrapFn, executeCmd executeCmdFn, getStream getStreamFn) (time.Duration, error) {
	opts := getOpts()
	cmd, input, err := performBootstrap(ctx, os.Stdin, opts)
	if err == nil {
		logging.Infof(ctx, "executing %s", cmd)
		err = executeCmd(ctx, cmd, input)
		// An ExitError indicates that we were able to bootstrap the executable and that it
		// failed, as opposed to being unable to launch the bootstrapped executable. In that
		// case, the recipe will have run and we don't need to make any modifications to the
		// build.
		var exitErr *exec.ExitError
		if stderrors.As(err, &exitErr) {
			return 0, err
		}
	}

	if err != nil {
		logging.Errorf(ctx, err.Error())
		handleBootstrapError(ctx, err, getStream)

		return bootstrap.SleepBeforeExiting.ValueOrDefault(err), err
	}

	return 0, nil
}

func main() {
	ctx := context.Background()
	ctx = gologger.StdConfig.Use(ctx)

	// Tracking soft deadline and calling shutdown causes the bootstrapper
	// to participate in the termination protocol. No explicit action is
	// necessary to terminate the bootstrapped executable, the signal will
	// be propagated to the entire process/console group.
	ctx, shutdown := lucictx.TrackSoftDeadline(ctx, 500*time.Millisecond)
	defer shutdown()

	sleepDuration, err := bootstrapMain(ctx, parseFlags, performBootstrap, executeCmd, getStream)
	time.Sleep(sleepDuration)
	if err != nil {
		os.Exit(1)
	}
}
