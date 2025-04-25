// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package protodoc

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	protoanalyzer "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/protos_doc_gen/protoanalyzer"
)

// DocumentProtoFile generate a documentation of the analyzed protobuffer file `file` in Markdown format at
// `documentationOutputPath`.
//
// If a type graph image is provided via `typeGraphPath`  it will be included in the generated
// documentation.
func DocumentProtoFile(file protoanalyzer.FileInfo, documentationOutputPath string, typeGraphPath string) {
	outputFile, err := os.OpenFile(documentationOutputPath, os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		log.Fatal("Error while opening file:", err)
	}

	writeDocumentHeader(file, typeGraphPath, outputFile)
	for _, m := range file.GetMessageSortedByFullName() {
		writeDocumentMessage(m, outputFile)
	}

	err = outputFile.Close()
	if err != nil {
		log.Fatal("Could not close output to file:", documentationOutputPath, " error:", err)
	}
	log.Printf("Documentation was written to:%s", documentationOutputPath)

}

// writeDocumentHeader writes the header of the documentation to `outputFile`:
//   - The name of the package
//   - If a typeGraphPath is not an empty string, it should point to an image of the  graph of the protomessages
//     in the package
func writeDocumentHeader(file protoanalyzer.FileInfo, typeGraphPath string, outputFile *os.File) {
	fmt.Fprintf(outputFile, "# Package %s\n\n", file.Name)

	for _, line := range file.Description {
		fmt.Fprint(outputFile, strings.TrimSpace(line), "\n")
	}
	fmt.Fprintf(outputFile, "\n\n")

	if typeGraphPath != "" {
		fmt.Fprintf(outputFile, "## Entities relation maps\n\n")
		fmt.Fprint(outputFile, "![Entity relation map]("+filepath.Base(typeGraphPath)+")", "\n\n")
	}

	fmt.Fprintf(outputFile, "## Contents\n\n")
	fmt.Fprint(outputFile, "[TOC]\n\n")

}

// writeDocumentMessage writes the documentation for a protobuffer message.
func writeDocumentMessage(message protoanalyzer.MessageInfo, outputFile *os.File) {
	// Write message name and description of the message
	fmt.Fprintf(outputFile, "### %s\n\n", message.FullName)
	for _, line := range message.Description {
		fmt.Fprint(outputFile, strings.TrimSpace(line), "\n")
	}
	fmt.Fprint(outputFile, "\n\n")

	// Write the documentation of each field.
	fmt.Fprint(outputFile, "|Field||type|Description|\n")
	fmt.Fprint(outputFile, "|-----|----|----|-----------|\n")
	for _, field := range message.GetFieldsSortedByName() {
		switch typedField := field.Type.(type) {
		case protoanalyzer.OneOfFieldType:
			// For OneOf fields on the first line is printed the details of the OneOf
			fmt.Fprintf(outputFile, "|%s||%s|%s|\n", field.Name, field.Type.ToString(), strings.Join(field.Description, " "))
			// On the next lines are printed all the possible fields of the OneOf with an identation.
			for _, subfield := range typedField.GetSubFieldsSortedByName() {
				fmt.Fprintf(outputFile, "||%s|%s|%s|\n", subfield.Name, subfield.Type.ToString(), strings.Join(subfield.Description, " "))
			}
		default:
			fmt.Fprintf(outputFile, "|%s||%s|%s|\n", field.Name, field.Type.ToString(), strings.Join(field.Description, " "))
		}
	}
	fmt.Fprint(outputFile, "\n\n")
}
