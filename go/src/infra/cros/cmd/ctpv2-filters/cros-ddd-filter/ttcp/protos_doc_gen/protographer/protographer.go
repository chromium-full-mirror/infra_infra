// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package protographer

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"

	protoanalyzer "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/protos_doc_gen/protoanalyzer"
)

// GraphProtoPackage generates a graphviz dot file from the typemap
// typeMap: is a map of message names to field infos.
// outputPath: is the path where the dot file will be written.
func GraphProtoPackage(dotCmdPath string, protoFile protoanalyzer.FileInfo, outputPath string) {
	dotFilePath := CreateDotFile(protoFile)
	dotToPng(dotCmdPath, dotFilePath, outputPath)
}

// `dotToPng` calls [Graphviz](https://graphviz.org/) dot command to convert
// a dot file to a Png file.
func dotToPng(dotCmdPath string, dotFilePath string, pngOutputPath string) {
	cmd := exec.Command(dotCmdPath, "-Tpng", "-o"+pngOutputPath, dotFilePath)
	var cmdout bytes.Buffer
	var cmderr bytes.Buffer
	cmd.Stdout = &cmdout
	cmd.Stderr = &cmderr
	err := cmd.Run()
	if err != nil {
		log.Println("Error in protos graph dependencies generation while converting dot file to png:", err)
		log.Println("   out:", cmdout.String())
		log.Println("   err:", cmderr.String())
		dotFileContent, err := os.ReadFile(dotFilePath)
		if err != nil {
			log.Println("could not read dot file at:", dotFilePath)
		} else {
			log.Println("   dot:", string(dotFileContent))
		}
		log.Fatal("Could not generate graph image.")
	}
}

// `CreateDotFile` use the results `protoFile` of the analyzis of a probuffer package
// to create the graph of the relation ships between the messages of the package in
// [Graphviz](https://graphviz.org/) dot language format. `CreateDotFile` returns the
// path of the generated dot file.
func CreateDotFile(protoFile protoanalyzer.FileInfo) string {
	dotFilePath := "local/protographer.dot"
	dotFile, err := os.OpenFile(dotFilePath, os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Fprintln(dotFile, "digraph G{")
	writeHeader(dotFile)
	writeNodes(protoFile, dotFile)
	writeLinks(protoFile, dotFile)
	fmt.Fprintln(dotFile, "}")

	err = dotFile.Close()
	if err != nil {
		log.Fatal("Could not close output to file:", dotFilePath, " error:", err)
	}
	return dotFilePath
}

// `writeHeader` writes the header of the dot file.
func writeHeader(dotFile io.Writer) {

	// Dot file preamble with a couple of options.
	fmt.Fprintln(dotFile, `node [shape=plain];
		layout="dot";
		start=1;
		overlap=False;
		graph [
		   rankdir = "LR"
		   ordering="out"
		];
		`)
}

// `writeNodes` writes the nodes of the graph. Each node is a message of the probuf package
// being documented, and inside the node are all the fields of the message.
// Notes:
//   - OneOf have all theior field grouped under on their right side.
//   - Repeated fields:
//   - if they are scalars, the type of the field has `[]` append to it.
//   - if they are reference to other message, the name of the field is appended with `[]`
//   - Map fields:
//   - if values are scalars, the type of the field is of the form `map[<key_type>]<value_type>`.
//   - if values are reference to other message, the type is of the form `map[<key_type>]`
//   - Any field type referencing a message gets a port so the vertices can use the field as a start point.
func writeNodes(protoFile protoanalyzer.FileInfo, dotFile io.Writer) {
	for _, message := range protoFile.GetMessageSortedByName() {
		fmt.Fprint(dotFile, message.Name, `   [label=<
		  <table border="0" cellborder="1" cellspacing="0">`, "\n")
		fmt.Fprint(dotFile, "              <tr><td port=\"header\" colspan=\"3\"><b>", message.FullName, "</b></td></tr>\n")
		for _, field := range message.GetFieldsSortedByName() {
			switch typedFieldType := field.Type.(type) {
			case protoanalyzer.ScalarFieldType:
				fmt.Fprint(dotFile, "              <tr><td colspan=\"2\" >", field.Name, "</td><td>", typedFieldType.ToString(), "</td></tr>\n")
			case protoanalyzer.ReferenceFieldType:
				fmt.Fprint(dotFile, "              <tr><td  colspan=\"3\" port=\"", message.Name+"_"+field.Name, "\">", field.Name, "[]</td></tr>\n")
			case protoanalyzer.OneOfFieldType:
				nbSubFields := len(typedFieldType.SubFields)
				first := true
				for _, subfield := range typedFieldType.GetSubFieldsSortedByName() {
					prefix := "              <tr>"
					if first {
						prefix = prefix + "<td rowspan=\"" + strconv.Itoa(nbSubFields) + "\" >" + field.Name + "</td>"
					}
					switch typedSubField := subfield.Type.(type) {
					case protoanalyzer.ScalarFieldType:
						fmt.Fprint(dotFile, prefix, "<td>", subfield.Name, "</td><td>", typedSubField.ToString(), "</td></tr>\n")
					case protoanalyzer.ReferenceFieldType:
						fmt.Fprint(dotFile, prefix, "<td colspan=\"2\" port=\"", message.Name+"_"+field.Name+"_"+subfield.Name, "\">", subfield.Name, "</td></tr>\n")
					}
					first = false
				}
			case protoanalyzer.ArrayField:
				switch typedArrayType := typedFieldType.SubType.(type) {
				case protoanalyzer.ScalarFieldType:
					fmt.Fprint(dotFile, "              <tr><td colspan=\"2\" >", field.Name, "</td><td>", typedFieldType.ToString(), "</td></tr>\n")
				case protoanalyzer.ReferenceFieldType:
					fmt.Fprint(dotFile, "              <tr><td  colspan=\"3\" port=\"", message.Name+"_"+field.Name, "\">", field.Name, "[]</td></tr>\n")
				default:
					log.Print("Warning graphing of array type is not implemented:", typedArrayType)
				}
			case protoanalyzer.MapField:
				switch typedValueType := typedFieldType.Value.(type) {
				case protoanalyzer.ScalarFieldType:
					fmt.Fprint(dotFile, "              <tr><td  colspan=\"2\" >", field.Name, "</td><td> map["+typedFieldType.Key.ToString()+"]"+typedValueType.ToString(), "[]</td></tr>\n")
				case protoanalyzer.ReferenceFieldType:
					fmt.Fprint(dotFile, "              <tr><td  colspan=\"2\">", field.Name+"</td><td port=\"", message.Name+"_"+field.Name, "\">  map["+typedFieldType.Key.ToString()+"]", "</td></tr>\n")
				default:
					log.Print("Warning graphing of array type is not implemented:", typedValueType)
				}
			default:
				log.Println("Warning, graphing for type not implemented: ", typedFieldType)
			}
		}
		fmt.Fprint(dotFile, "   </table>>];\n\n")
	}
}

// `writeLinks` generates graph vertices from the fields of messages that reference a message to that message node.
func writeLinks(protoFile protoanalyzer.FileInfo, dotFile io.Writer) {
	for _, message := range protoFile.GetMessageSortedByName() {
		for _, field := range message.GetFieldsSortedByName() {
			switch typedFieldType := field.Type.(type) {
			case protoanalyzer.ScalarFieldType:
				continue
			case protoanalyzer.ReferenceFieldType:
				fmt.Fprint(dotFile, "   ", message.Name, ":", message.Name+"_"+field.Name, " -> ", typedFieldType.Path, ":header;\n")
			case protoanalyzer.OneOfFieldType:
				for _, subfield := range typedFieldType.GetSubFieldsSortedByName() {
					switch typedSubField := subfield.Type.(type) {
					case protoanalyzer.ScalarFieldType:
						continue
					case protoanalyzer.ReferenceFieldType:

						fmt.Fprint(dotFile, "   ", message.Name, ":", message.Name+"_"+field.Name+"_"+subfield.Name, " -> ", typedSubField.Path, ":header;\n")
					default:
						log.Println("Warning, graphing for of OneOf field is not implemented for the subtype:", typedSubField)
					}
				}
			case protoanalyzer.ArrayField:
				switch typedArrayType := typedFieldType.SubType.(type) {
				case protoanalyzer.ScalarFieldType:
					continue
				case protoanalyzer.ReferenceFieldType:
					fmt.Fprint(dotFile, "   ", message.Name, ":", message.Name+"_"+field.Name, " -> ", typedArrayType.Path, ":header;\n")
				default:
					log.Print("Warning graphing of array type is not implemented:", typedArrayType)
				}
			case protoanalyzer.MapField:
				switch typedValueType := typedFieldType.Value.(type) {
				case protoanalyzer.ScalarFieldType:
					continue
				case protoanalyzer.ReferenceFieldType:
					fmt.Fprint(dotFile, "   ", message.Name, ":", message.Name+"_"+field.Name, " -> ", typedValueType.Path, ":header;\n")
				default:
					log.Print("Warning graphing of array type is not implemented:", typedValueType)
				}
			default:
				log.Println("Warning, graphing for type not implemented: ", typedFieldType)
			}
		}
	}
}
