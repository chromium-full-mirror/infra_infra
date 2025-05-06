// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"runtime/pprof"

	args "github.com/alexflint/go-arg"
	"google.golang.org/protobuf/encoding/protojson"

	solver_proto "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/protos/ttcp/solver"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets"
	buildmetadata "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/buildmetadata"
	dlmmetadata "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/dlmmetadata"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/hwid/db"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/errors"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/inventory"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/solver"
)

type CliArgs struct {
	// solve results output
	OutPath string `arg:"" help:"Output path for the computed variations and their respective representative devices, if ommited the results will be written to stdout."`

	// different ways to provide the variants category
	Variants     string `arg:"" help:"String of a TTCP category expression defining the variants to solve"`
	VariantsFile string `arg:"" help:"Path of a file containing a TTCP category expression defining the variants to solve"`

	// different ways to provide the opt in device class
	OptIn     string `arg:"" help:"String of a TTCP class expression defining the device to opt in from the inventory"`
	OptInFile string `arg:"" help:"Path of a file containing a TTCP class expression defining the device to opt in from the inventory"`

	// different ways to provide the opt out device class
	OptOut     string `arg:"" help:"String of a TTCP class expression defining the device to opt out from the inventory"`
	OptOutFile string `arg:"" help:"Path of a file containing a TTCP class expression defining the device to opt out from the inventory"`

	// different ways to provide an inventory
	InventorySwarming     bool   `arg:"" help:"Use the set of devices in the swarming fleet as the inventory of available devices"`
	InventorySwarmingPool string `arg:"" help:"restricts the inventory of swarming devices to a specific pool"`
	InventoryFile         string `arg:"" help:"Path of the file containing the inventory of available devices. At the moment the format is one HWID per line"`

	// Filter option over the results
	ClassFilter string `arg:"" help:"Optional filtering of the generated classes. The options are onlyTestable,onlyUntestable"`

	// data sets for predefined classes and categories
	ClassesAndCategoriesPath []string `arg:"required" help:"[Internal use only] Paths of the TTCP Classes and Categories collections."`

	// data sets for  chromeos build properties
	BuildMetadataPath string `arg:"required" help:"[Internal use only] Path of the build metadata dataset"`

	// data sets for chromeos dlm properties
	DlmMetadataPath string `arg:"required" help:"[Internal use only] Path of the dlm metadata dataset"`

	// data sets for device properties
	Index   string   `arg:"required" help:"[Internal use only] Hwid db Project index file path"`
	DbPaths []string `arg:"required" help:"[Internal use only] List of paths of the HWID db descriptors"`

	// Profiling options
	CpuProfileDataOutput string `arg:"" help:"If this option is provided with a path, CPU performance profiling data will be generated and written in the file at this path."`
}

func (CliArgs) Description() string {
	return `TTCP category expression solver. The solver computes all the variations of a category.`
}

func main() {
	fmt.Println()
	mainInt(os.Args[1:])
}

func mainInt(osargs []string) {
	var cliArgs CliArgs
	parser, err := args.NewParser(args.Config{Program: "", IgnoreEnv: true}, &cliArgs)
	if err != nil {
		fmt.Println("Error in building cli command parser:", err)
		os.Exit(1)
	}
	parser.MustParse(osargs)

	if cliArgs.CpuProfileDataOutput != "" {
		f, err := os.Create(cliArgs.CpuProfileDataOutput)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	results := computeVariants(cliArgs)

	marshalOptions := protojson.MarshalOptions{
		Indent: "    ",
	}
	json, err := marshalOptions.Marshal(&results)

	if err != nil {
		log.Println("Error:", err)
	} else {
		if cliArgs.OutPath == "" {
			log.Println(string(json))
		} else {
			err := os.WriteFile(cliArgs.OutPath, []byte(json), 0755)
			if err != nil {
				log.Fatal(errors.JoinError("Could not write results to "+cliArgs.OutPath, err))
			} else {
				log.Println("Results were written to " + cliArgs.OutPath)
			}
		}
	}
}

// computeVariants process the CliArgs in order to feed them to the TTCP solver.
func computeVariants(cliArgs CliArgs) solver_proto.SolvedCategory {
	// retrieve the set of devices in the inventory specified in the options.
	var (
		buf    bytes.Buffer
		logger = log.New(&buf, "logger: ", log.Lshortfile)
	)

	inventoryInfo := inventory.RetreiveInventoryProperties(cliArgs.InventorySwarming, cliArgs.InventoryFile,
		datasets.AllDatasetsResources{
			HwidDB: db.HwidDbResources{
				DescriptorsPaths: cliArgs.DbPaths,
				ProjectIndexPath: cliArgs.Index,
			},
			Buildmetadata: buildmetadata.BuildMetadataResources{
				Path: cliArgs.BuildMetadataPath,
			},
			Dlmmetadata: dlmmetadata.DlmResources{
				Path: cliArgs.DlmMetadataPath,
			},
		},
		cliArgs.InventorySwarmingPool,
		logger,
	)

	// Load and parse the TTCP request that specifies the dimension constraints on the desired variants
	variantsStr, err := parameterFromStringOrFile(cliArgs.Variants, cliArgs.VariantsFile, true)
	if err != nil {
		log.Fatal(errors.JoinError("Error in variant parameter.", err))
	}
	variants := solver.ParseTtcpCategoryExpression(variantsStr, logger)

	// Parse the optIn parameter
	optInStr, err := parameterFromStringOrFile(cliArgs.OptIn, cliArgs.OptInFile, false)
	if err != nil {
		log.Fatal(errors.JoinError("Error in optIn parameter.", err))
	}
	optIn := solver.ParseTtcpClassExpression(optInStr, logger)

	// Parse the optOut parameter
	optOutStr, err := parameterFromStringOrFile(cliArgs.OptOut, cliArgs.OptOutFile, false)
	if err != nil {
		log.Fatal(errors.JoinError("Error in optOut parameter.", err))
	}
	optOut := solver.ParseTtcpClassExpression(optOutStr, logger)

	// Parse the class/variant filter
	classFilter := solver.ClassFilterNone
	if cliArgs.ClassFilter != "" {
		var err error
		classFilter, err = solver.ParseClassFilter(cliArgs.ClassFilter)
		if err != nil {
			log.Fatal(errors.JoinError("Error in decoding ClassFilter option", err))
		}
	}

	categoriesAndClassesCollection, err := solver.ParseTtcpCategoryAndClassCollection(cliArgs.ClassesAndCategoriesPath, logger)
	if err != nil {
		log.Fatal(err)
	}

	// Compute the solution (Set of requests variants)
	solution, err := solver.EvalExpression(
		variants,
		classFilter,
		inventoryInfo,
		optIn,
		optOut,
		categoriesAndClassesCollection,
		logger,
		nil,
		cliArgs.InventorySwarming,
		cliArgs.InventorySwarmingPool)
	if err != nil {
		log.Fatal(err)
	}

	return solution
}

func parameterFromStringOrFile(str string, path string, required bool) (string, error) {
	variantsStr := ""
	if str != "" {
		variantsStr = str
	} else if path != "" {
		fileContent, err := ioutil.ReadFile(path)
		if err != nil {
			return "", errors.JoinError(
				fmt.Sprintf("Error while reading the file %s", path),
				err)
		}
		variantsStr = string(fileContent)
	} else {
		if required {
			return "", errors.NewError("No value was passed by string of path.")
		} else {
			return "", nil
		}
	}
	return variantsStr, nil
}
