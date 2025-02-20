// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/flag/stringmapflag"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/system/environ"

	"go.chromium.org/infra/cmd/gaedeploy/gcloud"
	"go.chromium.org/infra/cmd/gaedeploy/module"
)

// Placeholders for some CLI flags that indicate they weren't set.
const (
	moduleNamePlaceholder    = "<name>"
	moduleVersionPlaceholder = "<version>"
	moduleYAMLPlaceholder    = "<path>"
)

var cmdModule = &subcommands.Command{
	UsageLine: "module [...]",
	ShortDesc: "deploys a single GAE module (aka service)",
	LongDesc: `Deploys a single GAE module (aka service).

Fetches and unpacks the tarball, reads and potentially modifies the module
YAML there (by injecting site-specific configuration provided via -var), and
then calls gcloud to actually deploy it:
	gcloud app deploy --project <app-id> --version <version> <yaml path>

Does nothing at all if such version (based on -version-name) already exists,
unless -force flag is passed.

Process the YAML before deployment by removing some unused deprecated fields
and by interpreting non-standard "luci_gae_vars" section which can be used to
parametrize the YAML. The section may look like this:

  luci_gae_vars:
    example-app-id-dev:
      AUTH_SERVICE_HOST: auth-service-dev.appspot.com
    example-app-id-prod:
      AUTH_SERVICE_HOST: auth-service-prod.appspot.com

Such variables can appear in the YAML (inside various values, but not keys)
as e.g. ${AUTH_SERVICE_HOST} and they'll be substituted with values provided via
e.g. "-var AUTH_SERVICE_HOST=..." CLI flag or, if there's no such flag, ones
specified in the "luci_gae_vars" section in the YAML.

It is recommended to put some sample values in the YAML (to act as a
documentation) and store real production configuration elsewhere, and provide it
to gaedeploy dynamically via -var flags.
`,

	CommandRun: func() subcommands.CommandRun {
		c := &cmdModuleRun{}
		c.init()
		return c
	},
}

type cmdModuleRun struct {
	commandBase

	moduleName    string              // -module-name flag, required
	moduleYAML    string              // -module-yaml flag, require
	moduleVersion string              // -module-version flag, required
	flexImageURL  string              // -flex-image-url flag, optional
	vars          stringmapflag.Value // -var flags
	force         bool                // -force flag
}

func (c *cmdModuleRun) init() {
	c.commandBase.init(c.exec, extraFlags{
		appID:    true,
		tarball:  true,
		cacheDir: true,
		dryRun:   true,
	})
	c.Flags.StringVar(&c.moduleName, "module-name", moduleNamePlaceholder,
		"Name of the module to deploy (must match what's in the YAML).")
	c.Flags.StringVar(&c.moduleYAML, "module-yaml", moduleYAMLPlaceholder,
		"Path within the tarball to a module YAML to deploy.")
	c.Flags.StringVar(&c.moduleVersion, "module-version", moduleVersionPlaceholder,
		"Version name for the deployed code. Does nothing if such version already exists, unless -force is also given.")
	c.Flags.StringVar(&c.flexImageURL, "flex-image-url", "", "(Flex only) Docker image URL to pass to gcloud app deploy.")
	c.Flags.Var(&c.vars, "var", "A KEY=VALUE pair that defines a variable used when rendering module's YAML. May be repeated.")
	c.Flags.BoolVar(&c.force, "force", false,
		"Deploy the module even if such version already exists")
}

func (c *cmdModuleRun) exec(ctx context.Context) error {
	switch {
	case c.moduleName == moduleNamePlaceholder:
		return errBadFlag("-module-name", "a value is required")
	case c.moduleYAML == moduleYAMLPlaceholder:
		return errBadFlag("-module-yaml", "a value is required")
	case c.moduleVersion == moduleVersionPlaceholder:
		return errBadFlag("-module-version", "a value is required")
	}

	logging.Infof(ctx, "App ID:  %s", c.appID)
	logging.Infof(ctx, "Tarball: %s", c.tarballSource)
	if c.flexImageURL != "" {
		logging.Infof(ctx, "Image:   %s", c.flexImageURL)
	}
	logging.Infof(ctx, "Cache:   %s", c.cache.Root)
	logging.Infof(ctx, "Module:  %s", c.moduleName)
	logging.Infof(ctx, "YAML:    %s", c.moduleYAML)
	logging.Infof(ctx, "Version: %s", c.moduleVersion)

	if !c.force {
		logging.Infof(ctx, "Checking if such version already exists...")
		mods, err := gcloud.List(ctx, c.appID, c.moduleName)
		if err != nil {
			return errors.Annotate(err, "failed to check whether such version already exists").Err()
		}
		if _, yes := mods[c.moduleName][c.moduleVersion]; yes {
			logging.Infof(ctx, "Version %q of %q already exists, skipping the deployment!", c.moduleVersion, c.moduleName)
			return nil
		}
		logging.Infof(ctx, "No such version, will deploy it.")
	}

	return c.cache.WithTarball(ctx, c.source, func(root string) error {
		// Read the original YAML to inject site-specific configuration into it.
		logging.Infof(ctx, "Loading %s...", filepath.Join(root, c.moduleYAML))
		mod, err := module.ReadYAML(filepath.Join(root, c.moduleYAML))
		if err != nil {
			return errors.Annotate(err, "failed to read module YAML").Err()
		}
		if mod.Name != c.moduleName {
			return errors.Reason("module name in the yaml %q doesn't match -module-name flag %q", mod.Name, c.moduleName).Err()
		}

		// Convert it to something that gcloud actually understands.
		consumedVars, err := mod.Process(c.appID, map[string]string(c.vars))
		if err != nil {
			return errors.Annotate(err, "failed to process module's config").Err()
		}

		// Pretty print the final YAML to the console.
		blob, err := mod.DumpYAML()
		if err != nil {
			return errors.Annotate(err, "failed to serialize processed module config").Err()
		}
		logging.Infof(ctx, "Processed module YAML:\n\n%s\n", blob)

		// Loudly warn about supplied but unused variables.
		sortedVars := make([]string, 0, len(c.vars))
		for key := range c.vars {
			sortedVars = append(sortedVars, key)
		}
		sort.Strings(sortedVars)
		for _, key := range sortedVars {
			if !consumedVars.Has(key) {
				logging.Warningf(ctx, "Variable %q was passed via -var flag but not referenced in the YAML", key)
			}
		}

		// E.g. "frontend/" and "app.yaml".
		modDir, yamlBaseName := filepath.Split(c.moduleYAML)
		yamlBaseName = strings.TrimSuffix(yamlBaseName, ".yaml")

		// Need to save the YAML on disk in the same directory as the original one,
		// so that gcloud resolves all paths in it correctly. Need to put in a temp
		// file with randomized name so that multiple concurrent deploys from the
		// same unpacked tarball directory don't accidentally overwrite each others
		// files.
		yamlTmp, err := os.CreateTemp(filepath.Join(root, modDir), ".gaedeploy_"+yamlBaseName+".*.yaml")
		if err != nil {
			return errors.Annotate(err, "failed to create a temp file").Err()
		}
		defer func() {
			_ = yamlTmp.Close()
			if err := os.Remove(yamlTmp.Name()); err != nil {
				logging.Errorf(ctx, "Failed to remove the temp file: %s", err)
			}
		}()
		if _, err := yamlTmp.Write(blob); err != nil {
			return errors.Annotate(err, "failed to write the processed module config").Err()
		}
		if err := yamlTmp.Close(); err != nil {
			return errors.Annotate(err, "failed to flush the processed module config").Err()
		}

		// If this is a tarball with Go code, need to setup Go environment and
		// deploy from within it to make sure "gcloud app deploy" picks up correct
		// files.
		var env environ.Env
		if strings.HasPrefix(mod.Runtime, "go") {
			if modDir, env, err = prepareForGoDeploy(ctx, root, modDir); err != nil {
				return errors.Annotate(err, "failed to prepare for Go deployment").Err()
			}
		}

		// Use "beta" variant of the command if the module has "app_engine_apis",
		// otherwise this setting has no effect.
		//
		// See https://cloud.google.com/appengine/docs/standard/go/services/access.
		var command []string
		if mod.UsesAppEngineAPIs() {
			command = append(command, "beta")
		}
		command = append(command,
			"app", "deploy",
			"--project", c.appID,
			"--quiet", // disable interactive prompts
			"--no-promote",
			"--no-stop-previous-version",
			"--version", c.moduleVersion,
		)
		if c.flexImageURL != "" {
			command = append(command, "--image-url", c.flexImageURL)
		}
		command = append(command, filepath.Base(yamlTmp.Name()))

		// Perform the actual deployment.
		return gcloud.Run(ctx, command, filepath.Join(root, modDir), env, c.dryRun)
	})
}

// prepareForGoDeploy prepares Go environment variables and finds the GAE module
// root in the checked out tarball.
//
// `root` is a path to where the tarball is checked out.
// `modDir` is a path within the tarball to the directory with module's YAML.
//
// Uses the presence of "<root>/_gopath" or "<root>/_gomod" as indicator that
// the tarball was built by cloudbuildhelper (using "go_gae_bundle" build step).
// If they are absent, assumes the tarball was built just by zipping all files
// in a repo and they form a correct Go module already.
//
// Returns:
//
//	`newModDir`: a path within the tarball to use as new "directory with
//	    module's YAML" (may be the same as `modDir` if no changes are needed).
//	`env`: a environ to pass to "gcloud app deploy".
//	`err`: if something is not right.
func prepareForGoDeploy(ctx context.Context, root, modDir string) (newModDir string, env environ.Env, err error) {
	// Scrub the existing Go environ. This scrubs a bit more, but gcloud should
	// not depend on env vars that start with GO or CGO anyway.
	env = environ.System()
	env.RemoveMatch(func(k, v string) bool {
		return strings.HasPrefix(k, "GO") || strings.HasPrefix(k, "CGO")
	})

	checkExists := func(rel string) (bool, string, error) {
		abs, err := filepath.Abs(filepath.Join(root, rel))
		if err != nil {
			return false, "", err
		}
		switch _, err = os.Stat(abs); {
		case err == nil:
			return true, abs, nil
		case os.IsNotExist(err):
			return false, abs, nil
		default:
			return false, abs, err
		}
	}

	// Pick env vars depending on the presence of "_gopath" or "_gomod".
	haveGoPath, goPathAbs, err := checkExists("_gopath")
	if err != nil {
		return "", environ.Env{}, errors.Annotate(err, "checking _gopath").Err()
	}
	haveGoMod, _, err := checkExists("_gomod")
	if err != nil {
		return "", environ.Env{}, errors.Annotate(err, "checking _gomod").Err()
	}
	switch {
	case haveGoPath:
		logging.Infof(ctx, "Found _gopath, enabling GOPATH mode")
		env.Set("GOPATH", goPathAbs)
		env.Set("GO111MODULE", "off")
	case haveGoMod:
		logging.Infof(ctx, "Found _gomod, enabling modules mode")
		env.Set("GO111MODULE", "on")
	default:
		// Not a tarball built by `cloudbuildhelper`, use defaults.
		return modDir, env, nil
	}

	// If `modDir` is a symlink to something inside the tarball, follow it. This
	// is how tarballs built by cloudbuildhelper look like. The symlink leads to
	// the correct location of the package in the Go source tree. This allows
	// "gcloud" to discover all Go files correctly. Note that EvalSymlinks returns
	// the input path as is if it is not a symlink.
	dest, err := filepath.EvalSymlinks(filepath.Join(root, modDir))
	if err != nil {
		return "", environ.Env{}, errors.Annotate(err, "failed to evaluate %q as a symlink", modDir).Err()
	}
	rel, err := filepath.Rel(root, dest)
	if err != nil {
		return "", environ.Env{}, errors.Annotate(err, "failed to calculate rel(%q, %q)", root, dest).Err()
	}
	if !filepath.IsLocal(rel) {
		return "", environ.Env{}, errors.Annotate(err, "refusing to step outside of the tarball: %q", rel).Err()
	}
	if rel != modDir {
		logging.Infof(ctx, "Following symlink %q to its destination in %q", modDir, rel)
	}
	return rel, env, nil
}
