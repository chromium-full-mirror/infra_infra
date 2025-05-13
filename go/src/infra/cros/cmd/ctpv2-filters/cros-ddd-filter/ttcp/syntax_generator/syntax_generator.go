// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"strings"

	jsonpp "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/pprinter/json"
)

func main() {
	//This is temporary main body for development purposes.
	log.Println("Started TTCP syntax generator.")
	// TODO(b:244754491) replace this with a command line tool
	args := os.Args
	if len(args) != 3 {
		argsStr := strings.Join(args[:], " ")
		log.Fatal("TTCP generator needs two arguments: \n   ttcp_generator <boxster_configs.jsonproto> <syntax_output> \n arguments given were:", argsStr)
	}
	boxsterFile := args[1]
	syntaxOutput := args[2]
	log.Println("Boxster data file:", boxsterFile)
	log.Println("TTCP syntax file:", syntaxOutput)
	boxsterEntries := parseBoxster(boxsterFile)
	processBoxsterEntries(boxsterEntries, syntaxOutput)
}

// parseBoxster parses a boxster database file in returns an slice of all the
// individual values as unmarshaled json value with numbers parsed into
// json.Number.
func parseBoxster(boxster_dataset_path string) []any {
	log.Println("Parsing Boxster dataset")
	jsonFile, err := os.Open(boxster_dataset_path)
	if err != nil {
		log.Fatal("Error while trying to open boxster dataset file:", err)
	}
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	var value any
	reader := strings.NewReader(string(byteValue))
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()

	err = decoder.Decode(&value)
	if err != nil {
		log.Fatal("Unable to parse string into json:", err)
	}

	switch t := value.(type) {
	case map[string]any:
		boxsterValues, ok := value.(map[string]any)["values"]
		if !ok {
			log.Fatal("Expected boxster value to be container in an outer object.")
		}
		switch boxsterValues := boxsterValues.(type) {
		case []any:
			return boxsterValues
		default:
			log.Fatal("Boxster values are expected to be an array.")
		}
	default:
		log.Fatal("Unexpected type for boxster dataset", t)
	}
	return nil
}

// This is currently a place holder for just outputing the boxster data for development purposes.
func processBoxsterEntries(entries []any, outputPath string) {
	outputFile, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		log.Fatal("Error while opening file:", err)
	}

	for _, value := range entries {
		strValue := jsonpp.DispValue(value)
		_, err = outputFile.WriteString(strValue)
		if err != nil {
			log.Fatal("Could not write output to file:", outputPath, " error:", err)
		}
		_, err = outputFile.WriteString("\n\n----------------------------------------------------------------------------------------------\n\n")
		if err != nil {
			log.Fatal("Could not write output to file:", outputPath, " error:", err)
		}
	}

	err = outputFile.Close()
	if err != nil {
		log.Fatal("Could not close output to file:", outputPath, " error:", err)
	}
}
