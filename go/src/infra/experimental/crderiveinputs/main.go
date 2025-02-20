// Copyright (c) 2023 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"os"

	"golang.org/x/sync/errgroup"
	"golang.org/x/term"
	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/hardcoded/chromeinfra"

	"go.chromium.org/infra/experimental/crderiveinputs/inputpb"
)

func MainImpl(ctx context.Context, args *Args, authenticator *auth.Authenticator) (*inputpb.Manifest, error) {
	PIN("using embedded cipd to resolve cipd manifests.")
	PIN("using embedded vpython to resolve vpython specs.")
	PIN("using system vpython3 to execute python scripts.")
	PIN("using system git to resolve git dependencies.")

	embedTools, err := ExtractEmbed(args)

	manifest := &inputpb.Manifest{}
	manifest.GclientInputs = args.GClientVars.ToGclientInputs()

	oracle, err := NewOracle(ctx, manifest, args, authenticator)
	if err != nil {
		return nil, err
	}

	if err := ResolveDepotTools(oracle); err != nil {
		return nil, err
	}

	if err = oracle.PinGit("src", "https://chromium.googlesource.com/chromium/src", args.CrCommit); err != nil {
		return nil, err
	}

	hookImpls := []HookImpl{
		ignoreHookNamed("landmines"),
		ignoreHookNamed("vpython3_common"),
		ignoreHookNamed("remove_stale_pyc_files"),
		ignoreHookNamed("configure_siso"),
		ignoreHookNamed("configure_reclient_cfgs"),

		// Assuming these are not needed for the build, just for tests.
		ignoreHookNamed("perfetto_testdata"),
		ignoreHookNamed("mediapipe_integration_testdata"),
		ignoreHookNamed("Generate location tags for tests"),

		DownloadFromGCS{},
		DisableDepotToolsSelfupdate{},
		Lastchange{},
		NaclToolchain{},
		ClangUpdate{},
		RustUpdate{},
	}

	if err := embedTools.ParseDEPS(ctx, oracle, "", "src", args.GClientVars, true, hookImpls); err != nil {
		return nil, err
	}

	LEAKY("Assuming no scripts with embedded vpython specs are necessary for building.")

	eg, ctx := errgroup.WithContext(ctx)
	for _, source := range oracle.AllGitSources() {
		source := source

		eg.Go(func() error {
			vpythons, err := oracle.WalkDirectory(source.Path, "*.vpython3", "*.vpython")
			if err != nil {
				return err
			}

			for _, vpySpec := range vpythons {
				vpySpec := vpySpec

				eg.Go(func() error {
					return oracle.PinVpythonSpec(vpySpec)
				})
			}
			return nil
		})
	}

	if args.HostOS == "linux" && args.TargetOS == "linux" {
		linDeps, err := embedTools.ExtractInstallBuildDeps(ctx, oracle, "src")
		if err != nil {
			return nil, err
		}

		LEAKY("Assuming base system image Ubuntu 22.04 LTS")
		PIN("Base system image Ubuntu 22.04 LTS is unpinned.")
		IMPROVE("BaseImage specifier needs to be formalized.")
		linDeps.BaseImage = "Ubuntu 22.04 LTS"

		manifest.System = &inputpb.Manifest_LinuxDeps{LinuxDeps: linDeps}
	} else {
		TODO("Do not know how to calculate system deps for HostOS/TargetOS: %q/%q", args.HostOS, args.TargetOS)
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return manifest, nil
}

func main() {
	logCfg := gologger.LoggerConfig{
		Out:    os.Stderr,
		Format: "[%{level:.1s}%{time:15:04:05.000000Z-07:00} %{shortfile}]",
	}
	if term.IsTerminal(int(os.Stderr.Fd())) {
		logCfg.Format = "%{color}" + logCfg.Format + "%{color:reset}"
	}
	logCfg.Format += " %{message}"

	ctx := logCfg.Use(context.Background())
	Logger = logging.Get(ctx)

	cwd, err := os.Getwd()
	if err != nil {
		logging.Errorf(ctx, "Unable to get current working directory: %s", err)
		os.Exit(1)
	}

	args, err := parseArgs(os.Args[1:], cwd)
	if err != nil {
		errors.Log(ctx, err)
		os.Exit(1)
	}

	// TODO(Use real authcli)
	authOpts := chromeinfra.DefaultAuthOptions()
	authOpts.Scopes = append(
		authOpts.Scopes,
		auth.OAuthScopeEmail,
		"https://www.googleapis.com/auth/devstorage.read_only",
	)
	authenticator := auth.NewAuthenticator(ctx, auth.InteractiveLogin, authOpts)

	manifest, err := MainImpl(ctx, args, authenticator)
	if err != nil {
		errors.Log(ctx, err)
		os.Exit(1)
	}

	// NOTE: This output is not deterministic, but it would be possible to
	// implement a stable hash over this output by sorting all the maps/etc.
	//
	// This is left as an exercise for the reader.
	os.Stdout.WriteString(protojson.Format(manifest))

	if todoCount > 0 {
		Logger.Infof("More than one TODO emitted, exiting 1.")
		os.Exit(1)
	}
}
