// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"os"

	args "github.com/alexflint/go-arg"

	protoanalyzer "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/protos_doc_gen/protoanalyzer"
	protodoc "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/protos_doc_gen/protodoc"
	protographer "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/protos_doc_gen/protographer"
)

type CliArgs struct {
	DocumentationPath        string   `arg:"positional,required" help:"out: destination path of the markup documentation"`
	TypesGraphPath           string   `arg:"positional,required" help:"out: destination path of the png documentation"`
	DotPath                  string   `arg:"positional,required" help:"in: path of the dot command from graphviz"`
	ProtoFilePathToDocument  string   `arg:"positional,required" help:"in: path of the protobuf to document" `
	ProtobuferDescriptorPath []string `arg:"positional,required" help:"in: set of paths of all protobuf descriptors"`
}

func (CliArgs) Description() string {
	return `ProtoDocGen automatically generates a documentation of a protobuffer descriptor generated with protoc.
	It produces a Markdown file that fully documents the protobuffer types and png of the graph relations between the
	 types`
}
func main() {
	mainInt(os.Args[1:])
}

// This is part of the main function but isolated in its own function
// in order to test it.
func mainInt(osargs []string) {
	var cliArgs CliArgs
	parser, err := args.NewParser(args.Config{Program: "", IgnoreEnv: true}, &cliArgs)
	if err != nil {
		log.Println("Error in building cli command parser:", err)
		os.Exit(1)
	}
	err = parser.Parse(osargs)
	if err != nil {
		log.Println("Error in parsing cli arguments:", err)
		os.Exit(1)
	}

	fmt.Println("cli args:", cliArgs)
	// load all the descriptors
	protoFiles, err := protoanalyzer.LoadDescriptors(cliArgs.ProtobuferDescriptorPath)
	if err != nil {
		log.Fatal("Error encountered while parsing proto descriptors:", err)
	}

	// Get the descriptor file to document
	protoFileToDocument, err := protoFiles.FindFileByPath(cliArgs.ProtoFilePathToDocument)
	if err != nil {
		log.Fatalf("Failed to find the protobuf file to document: %s err: %s", cliArgs.ProtoFilePathToDocument, err)
	}

	// Analyse the descriptor
	fileInfo := protoanalyzer.AnalyzeFile(protoFileToDocument)

	// Generate the diagram of relation between messages in the descriptor
	protographer.GraphProtoPackage(cliArgs.DotPath, fileInfo, cliArgs.TypesGraphPath)
	// Generate the markdown file describing all the messages in the descriptor
	protodoc.DocumentProtoFile(fileInfo,
		cliArgs.DocumentationPath,
		cliArgs.TypesGraphPath)
}
