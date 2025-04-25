// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/testingtools"
)

func TestBaseCoverage(t *testing.T) {
	bazelQueryOutput := `//src/baseline_coverage_gen:baseline_coverage_gen.go
//src/baseline_coverage_gen:baseline_coverage_gen_test.go
//src/protos_doc_gen:protos_doc_gen.go
//src/protos_doc_gen:protos_doc_gen_test.go
//src/protos_doc_gen/protoanalyzer:proto_descriptor_loader.go
//src/protos_doc_gen/protoanalyzer:protoanalyzer.go
//src/protos_doc_gen/protoanalyzer:types.go
//src/protos_doc_gen/protodoc:protodoc.go
//src/protos_doc_gen/protographer:protographer.go
//src/ttcp/advisor:advisor_descriptor_test.go
//src/ttcp/advisor:advisor_list_lab_inventory_test.go
//src/ttcp/advisor:advisor_properties_test.go
//src/ttcp/advisor:main.go
//src/ttcp/expression_analyzer:expression_analyzer.go
//src/ttcp/expression_analyzer:expression_analyzer_test.go
//src/ttcp/json_pprint:json_pprint.go
//src/ttcp/json_pprint:json_pprint_test.go
//src/ttcp/libs/bitarray:bitarray.go
//src/ttcp/libs/bitarray:bitarray_test.go
//src/ttcp/libs/datasets:resources.go
//src/ttcp/libs/datasets/buildmetadata:reader.go
//src/ttcp/libs/datasets/hwid/db:db.go
//src/ttcp/libs/datasets/hwid/db:db_test.go
//src/ttcp/libs/datasets/hwid/db:descriptor.go
//src/ttcp/libs/datasets/hwid/db:descriptor_parser.go
//src/ttcp/libs/datasets/hwid/db:descriptor_properties.go
//src/ttcp/libs/datasets/hwid/db:deviceproperties.go
//src/ttcp/libs/datasets/hwid/db:io.go
//src/ttcp/libs/datasets/hwid/db:projectIndex.go
//src/ttcp/libs/datasets/hwid/decoder:decoder.go
//src/ttcp/libs/datasets/hwid/decoder:decoder_test.go
//src/ttcp/libs/datatools/validation:validation.go
//src/ttcp/libs/errors:errors.go
//src/ttcp/libs/inventory:inventory.go
//src/ttcp/libs/inventory/croslab:fleetInventory.go
//src/ttcp/libs/inventory/deviceinfo:deviceinfo.go
//src/ttcp/libs/pprinter/json:json.go
//src/ttcp/libs/pprinter/json:json_test.go
//src/ttcp/libs/solver:categoryFlattener.go
//src/ttcp/libs/solver:solver.go
//src/ttcp/libs/solver:solver_test.go
//src/ttcp/libs/solver:validation.go
//src/ttcp/libs/testingtools:assert.go
//src/ttcp/libs/testingtools:assert_test.go
//src/ttcp/libs/testingtools:bazel_test_tools.go
//src/ttcp/libs/testingtools:bazel_test_tools_test.go
//src/ttcp/libs/testingtools:negative_testing.go
//src/ttcp/libs/testingtools:negative_testing_test.go
//src/ttcp/solver_service:logger.go
//src/ttcp/solver_service:main.go
//src/ttcp/syntax_generator:syntax_generator.go
//src/ttcp/testingtools:assert.go
//src/ttcp/testingtools:assert_test.go
//src/ttcp/testingtools:negative_testing.go
//src/ttcp/testingtools:negative_testing_test.go
//src/ttcp/ttcp_classes_category_generator:main.go
//src/ttcp/ttcp_classes_category_generator:main_test.go`

	// creating the input file for extractFileSet
	tmpDir := t.TempDir()
	bazelQueryOutputFilePath := filepath.Join(tmpDir, "TestBaseCoverage")
	err := ioutil.WriteFile(bazelQueryOutputFilePath, []byte(bazelQueryOutput), 0644)
	testingtools.IsNilOrInvalid(t, err)

	files := extractFileSet(bazelQueryOutputFilePath)

	// Now we check the results of extractFileSet are the expected ones.
	expectedFiles := strings.Split(`src/baseline_coverage_gen/baseline_coverage_gen.go
src/baseline_coverage_gen/baseline_coverage_gen_test.go
src/protos_doc_gen/protos_doc_gen.go
src/protos_doc_gen/protos_doc_gen_test.go
src/protos_doc_gen/protoanalyzer/proto_descriptor_loader.go
src/protos_doc_gen/protoanalyzer/protoanalyzer.go
src/protos_doc_gen/protoanalyzer/types.go
src/protos_doc_gen/protodoc/protodoc.go
src/protos_doc_gen/protographer/protographer.go
src/ttcp/advisor/advisor_descriptor_test.go
src/ttcp/advisor/advisor_list_lab_inventory_test.go
src/ttcp/advisor/advisor_properties_test.go
src/ttcp/advisor/main.go
src/ttcp/expression_analyzer/expression_analyzer.go
src/ttcp/expression_analyzer/expression_analyzer_test.go
src/ttcp/json_pprint/json_pprint.go
src/ttcp/json_pprint/json_pprint_test.go
src/ttcp/libs/bitarray/bitarray.go
src/ttcp/libs/bitarray/bitarray_test.go
src/ttcp/libs/datasets/resources.go
src/ttcp/libs/datasets/buildmetadata/reader.go
src/ttcp/libs/datasets/hwid/db/db.go
src/ttcp/libs/datasets/hwid/db/db_test.go
src/ttcp/libs/datasets/hwid/db/descriptor.go
src/ttcp/libs/datasets/hwid/db/descriptor_parser.go
src/ttcp/libs/datasets/hwid/db/descriptor_properties.go
src/ttcp/libs/datasets/hwid/db/deviceproperties.go
src/ttcp/libs/datasets/hwid/db/io.go
src/ttcp/libs/datasets/hwid/db/projectIndex.go
src/ttcp/libs/datasets/hwid/decoder/decoder.go
src/ttcp/libs/datasets/hwid/decoder/decoder_test.go
src/ttcp/libs/datatools/validation/validation.go
src/ttcp/libs/errors/errors.go
src/ttcp/libs/inventory/inventory.go
src/ttcp/libs/inventory/croslab/fleetInventory.go
src/ttcp/libs/inventory/deviceinfo/deviceinfo.go
src/ttcp/libs/pprinter/json/json.go
src/ttcp/libs/pprinter/json/json_test.go
src/ttcp/libs/solver/categoryFlattener.go
src/ttcp/libs/solver/solver.go
src/ttcp/libs/solver/solver_test.go
src/ttcp/libs/solver/validation.go
src/ttcp/libs/testingtools/assert.go
src/ttcp/libs/testingtools/assert_test.go
src/ttcp/libs/testingtools/bazel_test_tools.go
src/ttcp/libs/testingtools/bazel_test_tools_test.go
src/ttcp/libs/testingtools/negative_testing.go
src/ttcp/libs/testingtools/negative_testing_test.go
src/ttcp/solver_service/logger.go
src/ttcp/solver_service/main.go
src/ttcp/syntax_generator/syntax_generator.go
src/ttcp/testingtools/assert.go
src/ttcp/testingtools/assert_test.go
src/ttcp/testingtools/negative_testing.go
src/ttcp/testingtools/negative_testing_test.go
src/ttcp/ttcp_classes_category_generator/main.go
src/ttcp/ttcp_classes_category_generator/main_test.go`, "\n")

	testingtools.Equal(t,
		len(files),
		len(expectedFiles),
	)
	for i, f := range files {
		testingtools.Equal(t, f, expectedFiles[i])
	}
}

func TestCliDescription(t *testing.T) {
	dsc := CliArgs{}.Description()
	if dsc == "" {
		t.Fatal("The description should not be empty")
	}
}

const testGoSource = `
	package a

	import(
		"fmt"
	)

	const c1 = "test"

	func typedSwitch(x interface{}){
		switch y:=1; typedX:=x.(type){
		case string:
			fmt.Println("it is a string:",typedX)
		default:
			fmt.Println("it is not a string")
		}

		_,ok:=x.(int)
		if ok{
			fmt.Println("x is an integer")
		}
	}

	func isFirstLetterA(str string) bool{

		switch c:=str[(0+1)];c{
		case 'A':
			return true
		default:
			fmt.Println("c was not 'A' instead it was:",c)
			return false
		}
		return false
	}

	func printall(a ...string){
		fmt.Println("all:",a ...)
		fmt.Println("first:",a[0])
		fmt.Println("head:",a[:2:2])
		fmt.Println("tail:",a[2:])
		addresse := &a
		fmt.Println("address:",addresse)
		fmt.Println("value:",*addresse)

	}

	func test(){
		fmt.Println("test")
	start:
		a:="test"
		if x:= a[2:];x == ""{
			fmt.Println("x is empty")
		}else{
			fmt.Println("x is not empty")
		}
		for _,c := range test {
			fmt.Println("code point:",c)
		}
		defer func(){
			fmt.Println("defered code")
		}()
	}

	func forTest(){
		for i:=1;i<5;i++ {
			if i%2==0{
				continue
			}
			fmt.Println("i:",i)
		}
	}

	func mapTest(){
		m:= map[string]int{
			"one":1,
			"two":2,
			"three":3,
		}
		for k,v:=range m{
			fmt.Println("m[",k,"]=",v,"==",m[k])
		}
	}

	func arrayTest(){
		a := [5]int{1,2,3,4,5}
		if len(a)!= 5{
			fmt.Println("unexpected state")
		}
		var x int = 4
		fmt.Println("5th item:",a[x])
	}

	type struct1 struct{field1 string}

	func structTest(){
		st :=struct{field1 string} {field1:"test"}
		fmt.Println(st.field1)
	}

	func testChannel(){
		done := make(chan struct{})
		go func(){
			defer func(){
				done <- struct{}{}
			}()
			fmt.Println("from channel")
		}()
	<-done
	}
	`

func TestExtractBaseLine(t *testing.T) {
	// creating the input file for extractBaseLine
	tmpDir := t.TempDir()
	testGoLangSourceFile := "testGoLangSource.go"
	testGoLangSourcePath := filepath.Join(tmpDir, testGoLangSourceFile)
	err := ioutil.WriteFile(testGoLangSourcePath, []byte(testGoSource), 0644)
	testingtools.IsNilOrInvalid(t, err)

	fileSetFile := "fileset.txt"
	fileSetPath := filepath.Join(tmpDir, fileSetFile)
	fmt.Println("fileSetPath", fileSetPath)
	err = ioutil.WriteFile(fileSetPath, []byte("//:"+testGoLangSourceFile+"\n//:test_test.go"), 0644)
	testingtools.IsNilOrInvalid(t, err)

	outfile := "results.lcov"
	outfilePath := filepath.Join(tmpDir, outfile)

	cliArgs := []string{
		tmpDir,
		fileSetPath,
		outfilePath,
	}

	fmt.Println("out file:", outfilePath)

	mainInt(cliArgs)

	_, err = ioutil.ReadFile(outfilePath)
	if err != nil {
		t.Fatalf("Could not read produced coverage file: %s", err)
	}
}
