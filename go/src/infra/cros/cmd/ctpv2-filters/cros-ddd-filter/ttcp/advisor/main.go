// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/csv"
	jsonstr "encoding/json"
	"log"
	"os"
	"sort"
	"strings"

	args "github.com/alexflint/go-arg"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets"
	buildmetadata "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/buildmetadata"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/hwid/db"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/hwid/decoder"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/errors"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/inventory"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/inventory/croslab"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/pprinter/json"
)

const hwid_db_path = "../../chromeos-internal/src/platform/chromeos-hwid/v3"

type InventoryPropertiesCmd struct {
	InventorySwarming     bool   `arg:"" help:"Use the set of devices in the swarming fleet as the inventory of available devices"`
	InventoryFile         string `arg:"" help:"Path of the file containing the inventory of available devices. At the moment the format is one HWID per line"`
	InventorySwarmingPool string `arg:"" help:"restricts the inventory of swarming devices to a specific pool"`

	OutPathCSV string `arg:"required" help:"path of the CSV where the properties of the inventory will be stored"`

	// datasets
	BuildMetadataPath string   `arg:"required" help:"[Internal use only] Path of the build metadata dataset"`
	Index             string   `arg:"required" help:"[Internal use only] Hwid db Project index file path"`
	DbPaths           []string `arg:"required" help:"[Internal use only] List of paths of the HWID db descriptors"`
}

type PropertiesCmd struct {
	Hwid    string `arg:"required" help:"HWID to retreive properties for"`
	OutPath string `arg:"" help:"Optional file path for the retrieved properties output"`

	// datasets
	BuildMetadataPath string   `arg:"required" help:"[Internal use only] Path of the build metadata dataset"`
	Index             string   `arg:"required" help:"[Internal use only] Hwid db Project index file path"`
	DbPaths           []string `arg:"required" help:"[Internal use only] List of paths of the HWID db descriptors"`
}

type HwidDescriptorCmd struct {
	Model   string `arg:"required" help:"Model to retreive HWID descriptor for"`
	OutPath string `arg:"" help:"Optional file path for the retrieved HWID descriptor output"`

	//datasets
	BuildMetadataPath string   `arg:"required" help:"[Internal use only] Path of the build metadata dataset"`
	Index             string   `arg:"required" help:"Hwid db Project index file path"`
	DbPaths           []string `arg:"required" help:"List of paths of the HWID db descriptors"`
}

type ListLabInventoryCmd struct {
	InventorySwarmingPool string `arg:"" help:"restricts the inventory of swarming devices to a specific pool"`

	OutPath string `arg:"" help:"Optional file path for the retrieved Lab Inventory output"`

	// datasets
	BuildMetadataPath string   `arg:"required" help:"[Internal use only] Path of the build metadata dataset"`
	Index             string   `arg:"required" help:"[Internal use only] Hwid db Project index file path"`
	DbPaths           []string `arg:"required" help:"[Internal use only] List of paths of the HWID db descriptors"`
}

type CliArgs struct {
	InventoryProperties *InventoryPropertiesCmd `arg:"subcommand:inventory_properties"`
	Properties          *PropertiesCmd          `arg:"subcommand:properties"`
	HwidDescriptor      *HwidDescriptorCmd      `arg:"subcommand:descriptor"`
	ListLabInventory    *ListLabInventoryCmd    `arg:"subcommand:list_lab_inventory"`
}

func main() {
	mainInt(os.Args[1:])
}

func mainInt(osargs []string) {
	var cliArgs CliArgs
	parser, err := args.NewParser(args.Config{Program: "", IgnoreEnv: true}, &cliArgs)
	if err != nil {
		log.Println("Error in building cli command parser:", err)
		os.Exit(1)
	}
	parser.MustParse(osargs)

	if cliArgs.Properties != nil {
		propertiesSubCommand(cliArgs.Properties)
	}
	if cliArgs.HwidDescriptor != nil {
		descriptorSubCommand(cliArgs.HwidDescriptor)
	}
	if cliArgs.ListLabInventory != nil {
		ListLabInventorySubCommand(cliArgs.ListLabInventory)
	}
	if cliArgs.InventoryProperties != nil {
		InventoryPropertiesCommand(cliArgs.InventoryProperties)
	}
	log.Println("Error:No subcommand was specified.")
	os.Exit(1)
}

func InventoryPropertiesCommand(cliArgs *InventoryPropertiesCmd) {
	log.Println("build:", cliArgs.BuildMetadataPath)

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
		},
		cliArgs.InventorySwarmingPool,
		"",
		logger,
	)

	// Extract all the unique properties present across all devices in the provided inventory
	propertyTypesAvailable := map[string]interface{}{}
	for _, d := range inventoryInfo {
		for k := range d.Properties.PropertiesDetails {
			propertyTypesAvailable[k] = nil
		}
	}
	//
	orderedPropertyTypes := []string{}
	for k := range propertyTypesAvailable {
		orderedPropertyTypes = append(orderedPropertyTypes, k)
	}
	sort.Slice(orderedPropertyTypes, func(i, j int) bool { return orderedPropertyTypes[i] < orderedPropertyTypes[j] })

	out, err := os.Create(cliArgs.OutPathCSV)
	if err != nil {
		log.Fatal("Could not create CSV output file. Err:", err)
	}
	defer out.Close()
	csvOut := csv.NewWriter(out)
	err = csvOut.Write(append([]string{"Device HWID"}, orderedPropertyTypes...))
	if err != nil {
		log.Fatal("Could not write header to CSV file:", err)
	}
	for _, d := range inventoryInfo {
		deviceCsvFields := []string{string(d.Id())}
		for _, k := range orderedPropertyTypes {
			values, ok := d.Properties.PropertiesDetails[k]
			if ok {
				var valuesStr []byte
				if len(values.Values) == 1 {
					valuesStr, err = jsonstr.Marshal(values.Values[0])
				} else {
					valuesStr, err = jsonstr.Marshal(values.Values)
				}
				if err != nil {
					valuesStr = []byte("Error encoding value")
				}
				deviceCsvFields = append(deviceCsvFields, string(valuesStr))
			} else {
				deviceCsvFields = append(deviceCsvFields, "")
			}
		}
		err := csvOut.Write(deviceCsvFields)
		if err != nil {
			log.Fatal("Could not write record to CSV file:", err)
		}
	}
	csvOut.Flush()
	log.Print("Inventory device details written to :", cliArgs.OutPathCSV)
	os.Exit(0)
}

func propertiesSubCommand(cliArgs *PropertiesCmd) {
	if len(cliArgs.DbPaths) == 0 {
		log.Fatalln("Not a single path of a HWID db file has been provided.")
	}

	log.Println("Load Hwid DB")
	hwidDb := db.InitializeHwidDb(db.HwidDbResources{
		DescriptorsPaths: cliArgs.DbPaths,
		ProjectIndexPath: cliArgs.Index,
	})
	log.Println("hwid lookup")
	hwidArg := strings.ToUpper(cliArgs.Hwid)
	hwid, err := decoder.DecodeHwid(hwidArg, hwidDb)

	if err != nil {
		log.Fatal("Could not decode hwid:", err)
		os.Exit(1)
	}
	//TODO check that the buildmetada properties are present.
	bomJson, err := json.FormatStructAsJson(hwid)
	if err != nil {
		log.Println("Error encoding results in json:", err)
		os.Exit(1)
	}
	if cliArgs.OutPath != "" {
		writeOutFile(bomJson, cliArgs.OutPath)
	} else {
		log.Println("TTCP properties for HWID ", cliArgs.Hwid)
		log.Println(bomJson)
	}
	os.Exit(0)
}

func descriptorSubCommand(cliArgs *HwidDescriptorCmd) {
	if len(cliArgs.DbPaths) == 0 {
		log.Fatalln("Not a single path of a HWID db file has been provided.")
	}

	log.Println("Load Hwid DB")
	hwidDb := db.InitializeHwidDb(db.HwidDbResources{
		DescriptorsPaths: cliArgs.DbPaths,
		ProjectIndexPath: cliArgs.Index,
	})
	log.Println("=====================")
	model := strings.ToUpper(cliArgs.Model)
	hwidDescriptor, error := hwidDb.GetDescriptor(model)
	if error != nil {
		log.Println("Error in retreiving descriptor:", error)
		os.Exit(1)
	}
	log.Println("Descriptor")
	str, err := json.FormatStructAsJson(hwidDescriptor)
	if err != nil {
		log.Println("Error in formating responce in Json:", err)
		os.Exit(1)
	}
	if cliArgs.OutPath != "" {
		writeOutFile(str, cliArgs.OutPath)
	} else {
		log.Println(str)
	}
	os.Exit(0)
}

func writeOutFile(outStr string, outPath string) {
	writeErr := os.WriteFile(outPath, []byte(outStr), 0755)
	if writeErr != nil {
		log.Fatal(errors.JoinError("Could not write results to "+outPath, writeErr))
	} else {
		log.Println("Results were written to " + outPath)
	}
}

func ListLabInventorySubCommand(cliArgs *ListLabInventoryCmd) {
	var labDeviceIds []string
	var (
		buf    bytes.Buffer
		logger = log.New(&buf, "logger: ", log.Lshortfile)
	)
	labDevices := croslab.GenerateAvailableDevicesInfo(
		datasets.AllDatasetsResources{
			HwidDB: db.HwidDbResources{
				DescriptorsPaths: cliArgs.DbPaths,
				ProjectIndexPath: cliArgs.Index,
			},
			Buildmetadata: buildmetadata.BuildMetadataResources{
				Path: cliArgs.BuildMetadataPath,
			},
		},
		cliArgs.InventorySwarmingPool,
		"",
		logger)

	if cliArgs.OutPath != "" {
		for _, device := range labDevices {
			labDeviceIds = append(labDeviceIds, string(device.Id()))
		}
		str, err := json.FormatStructAsJson(labDeviceIds)
		if err != nil {
			log.Println("Error in formating responce in Json:", err)
			os.Exit(1)
		} else {
			writeOutFile(str, cliArgs.OutPath)
		}
	} else {
		for _, device := range labDevices {
			log.Println(device.Id())
		}
	}
	os.Exit(0)
}
