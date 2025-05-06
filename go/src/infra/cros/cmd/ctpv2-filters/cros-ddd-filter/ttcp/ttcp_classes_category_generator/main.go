// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ttcpclassescategorygenerator

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"runtime/debug"
	"sort"

	args "github.com/alexflint/go-arg"
	"google.golang.org/protobuf/encoding/protojson"

	ttcpSyntax "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/protos/ttcp/syntax"
	buildmetadata "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/buildmetadata"
	dlmmetadata "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/dlmmetadata"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/hwid/db"
	swarmingdata "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/swarmingdata"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/errors"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/inventory/croslab"
)

type CliArgs struct {
	OutPath       string   `arg:"positional,required"  help:"Output path prefix for the computed TTCP categories and classes set data"`
	DocOutPath    string   `arg:"positional,required"  help:"Output path for the computed TTCP categories and classes set documentation"`
	Index         string   `arg:"positional,required"  help:"Hwid db Project index file path"`
	BuildMetaData string   `arg:"positional,required" help:"file path of the json file with the build metadata"`
	DlmMetaData   string   `arg:"positional,required"  help:"file path of the json file with the dlm metadata"`
	DbPaths       []string `arg:"positional,required"  help:"List of paths of the HWID db descriptors"`
}

func (CliArgs) Description() string {
	return ``
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
	log.Println("Args:", osargs)
	parser.MustParse(osargs)
	CreateTtcpClasessAndCategories(db.HwidDbResources{
		DescriptorsPaths: cliArgs.DbPaths,
		ProjectIndexPath: cliArgs.Index,
	}, buildmetadata.BuildMetadataResources{
		Path: cliArgs.BuildMetaData},
		dlmmetadata.DlmResources{
			Path: cliArgs.DlmMetaData}, cliArgs.OutPath, cliArgs.DocOutPath)
}

const maxSectionSize = 500

func CreateTtcpClasessAndCategories(hwidResc db.HwidDbResources, buildResc buildmetadata.BuildMetadataResources, dlmResc dlmmetadata.DlmResources, outputPath string, docOutPath string) {
	categoriesAndClasses := ttcpSyntax.Collection{
		Name: "HWID based auto generated Classes and Categories",
		Description: `TTCP named classes and categories.
					   This file is a json serialize protobuffer message of type NamedCollection.
					   The type NamedCollection is defined in src/config-internal/ttcp/protos/syntax.proto`,
		Categories: map[string]*ttcpSyntax.Category{},
		Classes:    map[string]*ttcpSyntax.Class{},
	}

	extractHwidClassesAndCategories(hwidResc, &categoriesAndClasses)
	extractBuildClassesAndCategories(buildResc, &categoriesAndClasses)
	extractDlmClassesAndCategories(dlmResc, &categoriesAndClasses)

	swarmResc, err := croslab.GetInventory("", nil)
	if err != nil {
		log.Fatal("Could not retrieve swarming inventory:", err)
	}
	extractSwarmClassesAndCategories(swarmResc, &categoriesAndClasses)
	//TODO add user generated classes and categories

	chunks := chunkClassesAndCategories(categoriesAndClasses, maxSectionSize)
	writeClassesAndCategoriesArchive(chunks, outputPath)
	writeDocs(chunks, docOutPath)
}

func extractBuildClassesAndCategories(resc buildmetadata.BuildMetadataResources, categoriesAndClasses *ttcpSyntax.Collection) {
	buildData, err := buildmetadata.ParseAsList(resc)
	if err != nil {
		log.Fatal("Fatal error.", err)
	}
	err = buildData.ExportCategories(categoriesAndClasses)
	if err != nil {
		log.Fatal("Could not export build metadata categories.", err)
	}
}

func extractHwidClassesAndCategories(resc db.HwidDbResources, categoriesAndClasses *ttcpSyntax.Collection) {
	hwidDb := db.InitializeHwidDb(resc)
	hwidDb.ExportCategories(categoriesAndClasses)
}

func extractDlmClassesAndCategories(resc dlmmetadata.DlmResources, categoriesAndClasses *ttcpSyntax.Collection) {
	dlmData, err := dlmmetadata.ParseAsList(resc)
	if err != nil {
		log.Fatal("Fatal error.", err)
	}
	dlmData.ExportCategories(categoriesAndClasses)
}

func extractSwarmClassesAndCategories(resc swarmingdata.SwarmDataResources, categoriesAndClasses *ttcpSyntax.Collection) {
	swarmData, err := resc.ParseAsList()
	if err != nil {
		log.Fatal("Fatal error.", err)
	}
	err = swarmData.ExportCategories(categoriesAndClasses)
	if err != nil {
		log.Fatal("Could not export swarming data categories.", err)
	}
}

func writeClassesAndCategoriesArchive(chunks []ttcpSyntax.Collection, outputPath string) {
	classesAndCategoriesArchive, err := os.Create(outputPath)
	if err != nil {
		log.Fatal(errors.JoinError("Unable to creating the archive for the dataset", err))
	}
	classesAndCategoriesArchiveWriter := zip.NewWriter(classesAndCategoriesArchive)

	for i, chunk := range chunks {
		dataSectionOutput, err := classesAndCategoriesArchiveWriter.Create(fmt.Sprintf("section_%d.json", i))
		if err != nil {
			log.Fatal(errors.JoinError("Unable to create archive section of the dataset", err))
		}
		writeClassesAndCategories(dataSectionOutput, chunk)
	}

	err = classesAndCategoriesArchiveWriter.Close()
	if err != nil {
		errors.JoinError("Unable to close classes and categories archive.", err)
	}
	err = classesAndCategoriesArchive.Close()
	if err != nil {
		errors.JoinError("Unable to close docs archive.", err)
	}
}

func writeDocs(chunks []ttcpSyntax.Collection, docOutPath string) {
	docs, err := os.Create(docOutPath)
	if err != nil {
		log.Fatal(errors.JoinError("Unable to create the archive for the documentation", err))
	}
	docsWriter := zip.NewWriter(docs)

	for i, chunk := range chunks {
		docSectionOutput, err := docsWriter.Create(fmt.Sprintf("section_%d.md", i))
		if err != nil {
			log.Fatal(errors.JoinError("Unable to create archive section of the documentation ", err))
		}
		writeChunkDocumentation(docSectionOutput, chunk)
	}
	indexOutput, err := docsWriter.Create("index.md")
	writeIndexDocumentation(indexOutput, chunks)

	err = docsWriter.Close()
	if err != nil {
		errors.JoinError("Unable to close doc archive.", err)
	}
	err = docs.Close()
	if err != nil {
		errors.JoinError("Unable to close docs archive.", err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

func chunkClassesAndCategories(classesAndCategories ttcpSyntax.Collection, sliceLength int) []ttcpSyntax.Collection {
	chunks := []ttcpSyntax.Collection{}
	if len(classesAndCategories.Categories) > 0 {
		categoryKeys := []string{}
		for name := range classesAndCategories.Categories {
			categoryKeys = append(categoryKeys, name)
		}
		sort.SliceStable(categoryKeys, func(i, j int) bool { return categoryKeys[i] < categoryKeys[j] })
		for nextKey := 0; nextKey < len(categoryKeys); {
			chunkLastKey := min(nextKey+sliceLength-1, len(categoryKeys)-1)
			newChunk := ttcpSyntax.Collection{
				Name:        classesAndCategories.Name,
				Description: "Categories from " + categoryKeys[nextKey] + " to " + categoryKeys[chunkLastKey],
				Categories:  map[string]*ttcpSyntax.Category{},
				Classes:     map[string]*ttcpSyntax.Class{},
			}
			for ; nextKey <= chunkLastKey; nextKey++ {
				key := categoryKeys[nextKey]
				newChunk.Categories[key] = classesAndCategories.Categories[key]
			}
			chunks = append(chunks, newChunk)
		}
	}
	if len(classesAndCategories.Classes) > 0 {
		classKeys := []string{}
		for name := range classesAndCategories.Classes {
			classKeys = append(classKeys, name)
		}
		sort.SliceStable(classKeys, func(i, j int) bool { return classKeys[i] < classKeys[j] })
		for nextKey := 0; nextKey < len(classKeys); {
			chunkLastKey := min(nextKey+sliceLength-1, len(classKeys)-1)
			newChunk := ttcpSyntax.Collection{
				Name:        classesAndCategories.Name,
				Description: "Classes from " + classKeys[nextKey] + " to " + classKeys[chunkLastKey],
				Categories:  map[string]*ttcpSyntax.Category{},
				Classes:     map[string]*ttcpSyntax.Class{},
			}
			for ; nextKey <= chunkLastKey; nextKey++ {
				key := classKeys[nextKey]
				newChunk.Classes[key] = classesAndCategories.Classes[key]
			}
			chunks = append(chunks, newChunk)
		}
	}
	return chunks
}

func writeClassesAndCategories(outputStream io.Writer, categoriesAndClasses ttcpSyntax.Collection) {
	// serializing the classes and categories to json encoded protobuf
	protosMarshaler := protojson.MarshalOptions{
		Multiline:       true,
		Indent:          "   ",
		UseProtoNames:   false,
		UseEnumNumbers:  false,
		EmitUnpopulated: false,
	}
	categoriesAndClassesJson, err := protosMarshaler.Marshal(&categoriesAndClasses)
	checkOrFatal(err, "Unable to marshal ttcp categories and classes to json.")
	re := regexp.MustCompile(`:\s+`) // Matches a colon followed by a space
	categoriesAndClassesJson = []byte(re.ReplaceAllLiteralString(string(categoriesAndClassesJson), ": "))

	_, err = outputStream.Write(categoriesAndClassesJson)
	checkOrFatal(err, "writing classes and categories ")
}

func writeIndexDocumentation(outputStream io.Writer, chunks []ttcpSyntax.Collection) {
	docs := "#  3D Classes and Categories index\n\n"
	docs += "## Index\n\n"

	docs += "| Section |\n"
	docs += "|---------|\n"

	for i, section := range chunks {
		docs += "|[" + section.Description + "](" + fmt.Sprintf("section_%d.md", i) + ")|\n"
	}

	_, err := outputStream.Write([]byte(docs))
	checkOrFatal(err, "writing documentation ")
}

func writeChunkDocumentation(outputStream io.Writer, categoriesAndClasses ttcpSyntax.Collection) {
	docs := "# " + categoriesAndClasses.Name + "\n\n"

	docs += "## Categories\n\n"

	docs += "| name | description | classes |\n"
	docs += "|------|-------------|---------|\n"

	categoryKeys := []string{}
	for name := range categoriesAndClasses.Categories {
		categoryKeys = append(categoryKeys, name)
	}
	sort.SliceStable(categoryKeys, func(i, j int) bool { return categoryKeys[i] < categoryKeys[j] })
	for _, name := range categoryKeys {
		category := categoriesAndClasses.Categories[name]
		description := category.Description
		CategoryClasses := bytes.Buffer{}
		first := true
		for _, cl := range category.Category.(*ttcpSyntax.Category_Enumerated).Enumerated.Classes {
			if first {
				first = false
			} else {
				CategoryClasses.WriteString("<br />")
			}
			switch typedClass := cl.Body.(type) {
			case *ttcpSyntax.ClassExpression_Name:
				CategoryClasses.WriteString(typedClass.Name)
			case *ttcpSyntax.ClassExpression_Value:
				CategoryClasses.WriteString(typedClass.Value.Name)
			}
		}
		docs += "|" + name + "|" + description + "|" + CategoryClasses.String() + "|\n"
	}

	if len(categoriesAndClasses.Classes) > 0 {

		docs += "\n\n## Classes\n\n"

		docs += "| name | description|\n"
		docs += "|------|------------|\n"

		classKeys := []string{}
		for name := range categoriesAndClasses.Classes {
			classKeys = append(classKeys, name)
		}
		sort.SliceStable(classKeys, func(i, j int) bool { return categoryKeys[i] < categoryKeys[j] })
		for _, name := range classKeys {
			class := categoriesAndClasses.Classes[name]
			fmt.Println("class:", class)
			docs += "|" + name + "|" + class.Description + "|\n"
		}
	}

	_, err := outputStream.Write([]byte(docs))
	checkOrFatal(err, "writing documentation ")
}

func checkOrFatal(err error, context string) {
	if err != nil {
		log.Println(string(debug.Stack()))
		log.Fatal("Unexpected error while ", context, ":", err)
	}
}
