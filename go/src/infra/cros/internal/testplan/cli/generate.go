// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cli

import (
	"context"
	"errors"
	"math/rand"
	"os"
	"path/filepath"

	"github.com/maruel/subcommands"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	buildpb "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/payload"
	testpb "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/infra/proto/go/chromiumos"
	"go.chromium.org/chromiumos/infra/proto/go/testplans"
	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/flag"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/cros/internal/testplan"
	"go.chromium.org/infra/cros/internal/testplan/compatibility"
	"go.chromium.org/infra/cros/internal/testplan/protoio"
)

func CmdGenerate(authOpts auth.Options) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "generate -plan plan1.star [-plan plan2.star] -dutattributes PATH -buildmetadata -out OUTPUT",
		ShortDesc: "generate CoverageRule protos",
		LongDesc: `Generate CoverageRule protos.

Evaluates Starlark files to generate CoverageRules as newline-delimited json protos.
`,
		CommandRun: func() subcommands.CommandRun {
			r := &generateRun{}
			r.addSharedFlags(authOpts)

			r.Flags.Var(
				flag.StringSlice(&r.planPaths),
				"plan",
				"Starlark file to use. Must be specified at least once.",
			)
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
				&r.chromiumosSourceRootPath,
				"crossrcroot",
				"",
				"Path to the root of a Chromium OS source checkout. Default "+
					"versions of dutattributes, buildmetadata, configbundlelist, "+
					"and boardprioritylist in this source checkout will be used, as "+
					"a convenience to avoid specifying all these full paths. "+
					"crossrcroot is mutually exclusive with the above flags.",
			)
			r.Flags.BoolVar(
				&r.ctpV1,
				"ctpv1",
				false,
				"Output GenerateTestPlanResponse protos instead of CoverageRules, "+
					"for backwards compatibility with CTP1. Output is still "+
					"to <out>. generatetestplanreq must be set if this flag is "+
					"true",
			)
			r.Flags.StringVar(
				&r.generateTestPlanReqPath,
				"generatetestplanreq",
				"",
				"Path to a proto file containing a GenerateTestPlanRequest. Can be"+
					"JSON or binary proto. Should be set iff ctpv1 is set.",
			)
			r.Flags.StringVar(
				&r.boardPriorityListPath,
				"boardprioritylist",
				"",
				"Path to a proto file containing a BoardPriorityList. Can be JSON"+
					"or binary proto. Should be set iff ctpv1 is set.",
			)
			r.Flags.StringVar(
				&r.builderConfigsPath,
				"builderconfigs",
				"",
				"Path to a proto file containing a BuilderConfigs. Can be JSON"+
					"or binary proto. Should be set iff ctpv1 is set.",
			)
			r.Flags.StringVar(
				&r.out,
				"out",
				"",
				"Path to the output CoverageRules (or GenerateTestPlanResponse if -ctpv1 is set).",
			)

			r.templateParametersFlag.Register(&r.Flags)

			return r
		},
	}
}

type generateRun struct {
	baseTestPlanRun

	planPaths                []string
	buildMetadataListPath    string
	dutAttributeListPath     string
	configBundleListPath     string
	chromiumosSourceRootPath string
	ctpV1                    bool
	generateTestPlanReqPath  string
	boardPriorityListPath    string
	builderConfigsPath       string
	templateParametersFlag   TemplateParametersFlag
	out                      string
}

func (r *generateRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, r, env)
	return errToCode(a, r.run(ctx))
}

// validateFlags checks valid flags are passed to generate, e.g. all required
// flags are set.
//
// If r.chromiumosSourceRootPath is set, other flags (e.g.
// r.dutAttributeListPath) are updated to default values relative to the source
// root.
func (r *generateRun) validateFlags(ctx context.Context) error {
	if len(r.planPaths) == 0 {
		return errors.New("at least one -plan is required")
	}

	if r.chromiumosSourceRootPath == "" {
		if r.dutAttributeListPath == "" {
			return errors.New("-dutattributes is required if -crossrcroot is not set")
		}

		if r.buildMetadataListPath == "" {
			return errors.New("-buildmetadata is required if -crossrcroot is not set")
		}

		if r.configBundleListPath == "" {
			return errors.New("-configbundlelist is required if -crossrcroot is not set")
		}

		if r.ctpV1 && r.boardPriorityListPath == "" {
			return errors.New("-boardprioritylist or -crossrcroot must be set if -ctpv1 is set")
		}

		if r.ctpV1 && r.builderConfigsPath == "" {
			return errors.New("-builderconfigs or -crossrcroot must be set if -ctpv1 is set")
		}
	} else {
		if r.dutAttributeListPath != "" || r.buildMetadataListPath != "" || r.configBundleListPath != "" || r.boardPriorityListPath != "" || r.builderConfigsPath != "" {
			return errors.New("-dutattributes, -buildmetadata, -configbundlelist, and -boardprioritylist cannot be set if -crossrcroot is set")
		}

		logging.Infof(ctx, "crossrcroot set to %q, updating dutattributes, buildmetadata, and configbundlelist", r.chromiumosSourceRootPath)
		r.dutAttributeListPath = filepath.Join(r.chromiumosSourceRootPath, "src", "config", "generated", "dut_attributes.jsonproto")
		r.buildMetadataListPath = filepath.Join(r.chromiumosSourceRootPath, "src", "config-internal", "build", "generated", "build_metadata.jsonproto")
		r.configBundleListPath = filepath.Join(r.chromiumosSourceRootPath, "src", "config-internal", "hw_design", "generated", "configs.jsonproto")

		if r.ctpV1 {
			logging.Infof(ctx, "crossrcroot set to %q, updating boardprioritylist and builderconfigs", r.chromiumosSourceRootPath)
			r.boardPriorityListPath = filepath.Join(r.chromiumosSourceRootPath, "src", "config-internal", "board_config", "generated", "board_priority.binaryproto")
			r.builderConfigsPath = filepath.Join(r.chromiumosSourceRootPath, "infra", "config", "generated", "builder_configs.binaryproto")
		}
	}

	if r.out == "" {
		return errors.New("-out is required")
	}

	if r.ctpV1 != (r.generateTestPlanReqPath != "") {
		return errors.New("-generatetestplanreq must be set iff -ctpv1 is set")
	}

	if !r.ctpV1 && r.boardPriorityListPath != "" {
		return errors.New("-boardprioritylist cannot be set if -ctpv1 is not set")
	}

	return nil
}

// run is the actual implementation of the generate command.
func (r *generateRun) run(ctx context.Context) (err error) {
	if err := r.validateFlags(ctx); err != nil {
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

	if r.ctpV1 {
		logging.Infof(ctx,
			"Outputting GenerateTestPlanRequest to %s instead of CoverageRules, for backwards compatibility with CTPV1",
			r.out,
		)

		generateTestPlanReq := &testplans.GenerateTestPlanRequest{}
		if err := protoio.ReadBinaryOrJSONPb(ctx, r.generateTestPlanReqPath, generateTestPlanReq); err != nil {
			return err
		}

		boardPriorityList := &testplans.BoardPriorityList{}
		if err := protoio.ReadBinaryOrJSONPb(ctx, r.boardPriorityListPath, boardPriorityList); err != nil {
			return err
		}

		builderConfigs := &chromiumos.BuilderConfigs{}
		if err := protoio.ReadBinaryOrJSONPb(ctx, r.builderConfigsPath, builderConfigs); err != nil {
			return err
		}

		resp, err := compatibility.ToCTP1(
			ctx,
			// Disable randomness when selecting boards for now, since this can
			// lead to cases where a different board is selected on the first
			// and second CQ runs, causing test history to not be reused.
			// TODO(b/278624587): Pass a list of previously-passed tests, so
			// this can be used to ensure test reuse.
			rand.New(rand.NewSource(0)),
			hwTestPlans, vmTestPlans, generateTestPlanReq, dutAttributeList, boardPriorityList, builderConfigs,
		)
		if err != nil {
			return err
		}

		outFile, err := os.Create(r.out)
		if err != nil {
			return err
		}
		defer func() {
			err = outFile.Close()
		}()

		respBytes, err := proto.Marshal(resp)
		if err != nil {
			return err
		}

		if _, err := outFile.Write(respBytes); err != nil {
			return err
		}

		jsonprotoOut := protoio.FilepathAsJsonpb(r.out)
		if jsonprotoOut == r.out {
			logging.Warningf(ctx, "Output path set to jsonpb (%q), but output will be written as binaryproto", r.out)
		} else {
			logging.Infof(ctx, "Writing jsonproto version of output to %s", jsonprotoOut)

			jsonprotoOutFile, err := os.Create(jsonprotoOut)
			if err != nil {
				return err
			}
			defer func() {
				err = jsonprotoOutFile.Close()
			}()

			jsonprotoRespBytes, err := protojson.Marshal(resp)
			if err != nil {
				return err
			}

			if _, err := jsonprotoOutFile.Write(jsonprotoRespBytes); err != nil {
				return err
			}
		}

		return nil
	}

	var allRules []*testpb.CoverageRule
	for _, m := range hwTestPlans {
		allRules = append(allRules, m.GetCoverageRules()...)
	}

	for _, m := range vmTestPlans {
		allRules = append(allRules, m.GetCoverageRules()...)
	}

	logging.Infof(ctx, "Generated %d CoverageRules, writing to %s", len(allRules), r.out)

	if err := protoio.WriteJsonl(allRules, r.out); err != nil {
		return err
	}

	return nil
}
