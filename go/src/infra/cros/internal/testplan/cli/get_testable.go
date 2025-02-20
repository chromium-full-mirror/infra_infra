// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cli defines different commands for the test_plan tool.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/maruel/subcommands"

	buildpb "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/payload"
	testpb "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/infra/proto/go/chromiumos"
	"go.chromium.org/luci/auth"
	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/flag"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/cros/internal/testplan"
	"go.chromium.org/infra/cros/internal/testplan/compatibility"
	"go.chromium.org/infra/cros/internal/testplan/protoio"
)

type getTestableRun struct {
	baseTestPlanRun

	planPaths              []string
	builds                 []*bbpb.Build
	buildsPath             string
	buildMetadataListPath  string
	dutAttributeListPath   string
	configBundleListPath   string
	builderConfigsPath     string
	templateParametersFlag TemplateParametersFlag
}

func CmdGetTestable(authOpts auth.Options) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: `get-testable -plan plan1.star [-plan plan2.star] -builds PATH -dutattributes PATH -buildmetadata PATH -configbundlelist PATH -builderconfigs PATH`,
		ShortDesc: "get a list of builds that could possibly be tested by plans",
		LongDesc: `Get a list of builds that could possibly be tested by plans.

First compute a set of CoverageRules from plans, then compute which builds could
possibly be tested based off the CoverageRules.

This doesn't take the status, output test artifacts, etc. of builds into
account, just whether their build target, variant, and profile could be included
in one of the CoverageRules.

The list of testable builders is printed to stdout, delimited by spaces.

Note that the main use case for this is programatically deciding which builders
to collect for testing, so it is marked as advanced, and doesn't offer
conveniences such as -crossrcroot that generate does.
	`,
		Advanced: true,
		CommandRun: func() subcommands.CommandRun {
			r := &getTestableRun{}
			r.addSharedFlags(authOpts)

			r.Flags.Var(
				flag.StringSlice(&r.planPaths),
				"plan",
				"Starlark file to use. Must be specified at least once.",
			)
			r.Flags.StringVar(
				&r.buildsPath,
				"builds",
				"",
				"Path to a file containing Buildbucket build protos to analyze, with"+
					"one JSON proto per-line. Each proto must include the "+
					"`builder.builder` field and the `build_target.name` input "+
					"property, all other fields will be ignored")
			r.Flags.StringVar(
				&r.dutAttributeListPath,
				"dutattributes",
				"",
				"Path to a proto file containing a DutAttributeList. Can be JSON "+
					"or binary proto.",
			)
			r.Flags.StringVar(
				&r.buildMetadataListPath,
				"buildmetadata",
				"",
				"Path to a proto file containing a SystemImage.BuildMetadataList. "+
					"Can be JSON or binary proto.",
			)
			r.Flags.StringVar(
				&r.configBundleListPath,
				"configbundlelist",
				"",
				"Path to a proto file containing a ConfigBundleList. Can be JSON or "+
					"binary proto.",
			)
			r.Flags.StringVar(
				&r.builderConfigsPath,
				"builderconfigs",
				"",
				"Path to a proto file containing a BuilderConfigs. Can be JSON"+
					"or binary proto. Should be set iff ctpv1 is set.",
			)

			r.templateParametersFlag.Register(&r.Flags)

			return r
		},
	}
}
func (r *getTestableRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, r, env)
	return errToCode(a, r.run(ctx))
}

// validateFlags checks valid flags are passed to get-testable, e.g. all
// required flags are set.
func (r *getTestableRun) validateFlags() error {
	if len(r.planPaths) == 0 {
		return errors.New("at least one -plan is required")
	}

	if len(r.buildsPath) == 0 {
		return errors.New("builds must be set")
	}

	// Parse the builds into r.builds.
	parsedBuilds, err := protoio.ReadJsonl(r.buildsPath, func() *bbpb.Build { return &bbpb.Build{} })
	if err != nil {
		return err
	}

	r.builds = parsedBuilds

	if len(r.builds) == 0 {
		return errors.New("at least one build is required")
	}

	for _, build := range r.builds {
		if build.GetBuilder().GetBuilder() == "" {
			return fmt.Errorf("builds must set builder.builder, got %q", build)
		}

		inputProps := build.GetInput().GetProperties()
		btProp, ok := inputProps.GetFields()["build_target"]
		if !ok {
			return fmt.Errorf("builds must set the build_target.name input prop, got %q", build)
		}

		if _, ok := btProp.GetStructValue().GetFields()["name"]; !ok {
			return fmt.Errorf("builds must set the build_target.name input prop, got %q", build)
		}
	}

	if r.dutAttributeListPath == "" {
		return errors.New("-dutattributes is required")
	}

	if r.buildMetadataListPath == "" {
		return errors.New("-buildmetadata is required")
	}

	if r.configBundleListPath == "" {
		return errors.New("-configbundlelist is required")
	}

	if r.builderConfigsPath == "" {
		return errors.New("-builderconfigs is required")
	}

	return nil
}

func (r *getTestableRun) run(ctx context.Context) error {
	if err := r.validateFlags(); err != nil {
		return err
	}

	pathToTemplateParametersList, err := r.templateParametersFlag.Parse(ctx)
	if err != nil {
		return err
	}

	buildMetadataList := &buildpb.SystemImage_BuildMetadataList{}
	if err := protoio.ReadBinaryOrJSONPb(ctx, r.buildMetadataListPath, buildMetadataList); err != nil {
		return err
	}

	logging.Infof(ctx, "Read %d SystemImage.Metadata from %s", len(buildMetadataList.Values), r.buildMetadataListPath)

	for _, buildMetadata := range buildMetadataList.Values {
		logging.Infof(ctx, "Read BuildMetadata: %s", buildMetadata)
	}

	dutAttributeList := &testpb.DutAttributeList{}
	if err := protoio.ReadBinaryOrJSONPb(ctx, r.dutAttributeListPath, dutAttributeList); err != nil {
		return err
	}

	logging.Infof(ctx, "Read %d DutAttributes from %s", len(dutAttributeList.DutAttributes), r.dutAttributeListPath)

	for _, dutAttribute := range dutAttributeList.DutAttributes {
		logging.Infof(ctx, "Read DutAttribute: %s", dutAttribute)
	}

	logging.Infof(ctx, "Starting read of ConfigBundleList from %s", r.configBundleListPath)

	configBundleList := &payload.ConfigBundleList{}
	if err := protoio.ReadBinaryOrJSONPb(ctx, r.configBundleListPath, configBundleList); err != nil {
		return err
	}

	logging.Infof(ctx, "Read %d ConfigBundles from %s", len(configBundleList.Values), r.configBundleListPath)

	hwTestPlans, vmTestPlans, err := testplan.Generate(
		ctx, r.planPaths, buildMetadataList, dutAttributeList, configBundleList, pathToTemplateParametersList,
	)
	if err != nil {
		return err
	}

	builderConfigs := &chromiumos.BuilderConfigs{}
	if err := protoio.ReadBinaryOrJSONPb(ctx, r.builderConfigsPath, builderConfigs); err != nil {
		return err
	}

	logging.Infof(ctx,
		"Read %d BuilderConfigs from %s",
		len(builderConfigs.GetBuilderConfigs()),
		r.builderConfigsPath,
	)

	testableBuilds, err := compatibility.TestableBuilds(
		ctx,
		hwTestPlans,
		vmTestPlans,
		r.builds,
		builderConfigs,
		dutAttributeList,
	)
	if err != nil {
		return err
	}

	builderNames := make([]string, 0, len(testableBuilds))
	for _, build := range testableBuilds {
		builderNames = append(builderNames, build.GetBuilder().GetBuilder())
	}

	_, err = fmt.Fprint(os.Stdout, strings.Join(builderNames, " ")+"\n")
	return err
}
