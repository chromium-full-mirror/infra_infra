// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package json

import (
	"encoding/json"
	"strings"
	"testing"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/testingtools"
)

func TestJson2strInteger(t *testing.T) {
	checkJsonDecodePrintInvariant("2", t)
}

func TestJson2strFloat(t *testing.T) {
	checkJsonDecodePrintInvariant("2.1", t)
}

func TestJson2strString(t *testing.T) {
	checkJsonDecodePrintInvariant("\"test\"", t)
}

func TestJson2strTrue(t *testing.T) {
	checkJsonDecodePrintInvariant("true", t)
}

func TestJson2strFalse(t *testing.T) {
	checkJsonDecodePrintInvariant("false", t)
}

func TestJson2strNull(t *testing.T) {
	checkJsonDecodePrintInvariant("null", t)
}

func TestJson2strObjectEmpty(t *testing.T) {
	checkJsonDecodePrintInvariant("{}", t)
}
func TestJson2strObjectSingleField(t *testing.T) {
	checkJsonDecodePrint(
		"{ \"f1\" : 1}",
		"{ \"f1\" : 1 }", t)
}

func TestJson2strObjectTwoFields(t *testing.T) {
	checkJsonDecodePrint(
		"{ \"f1\" : 1,\n  \"f2\" : 2}",
		"{ \"f1\" : 1 , \"f2\" : 2 }", t)
}

func TestJson2strArrayEmpty(t *testing.T) {
	checkJsonDecodePrintInvariant(
		"[]", t)
}

func TestJson2strArrayOne(t *testing.T) {
	checkJsonDecodePrint(
		"[ 1]",
		"[1]", t)
}

func TestJson2strArrayTwo(t *testing.T) {
	checkJsonDecodePrint(
		"[ 1,\n  2]",
		"[1,2]", t)
}

func TestJsonWithFloat64(t *testing.T) {
	str := "1"
	var value interface{}
	reader := strings.NewReader(str)
	decoder := json.NewDecoder(reader)

	err := decoder.Decode(&value)
	if err != nil {
		t.Error("Unable to parse string into json:", err)
	}

	jsonStr := DispValue(value)
	testingtools.Equivalent(t, str, jsonStr)
}

// checkJsonDecodePrint checks if parsing str as json and pretty printing it
// returns string equal to str. If it is not equal, the test will be marked as
// failed and debugging logging will be emitted
func checkJsonDecodePrintInvariant(str string, t *testing.T) {
	checkJsonDecodePrint(str, str, t)
}

// checkJsonDecodePrint parse str as a json string, pretty prints it and
// compares the output of the pretty print to expected. If the two are equal,
// there is no side effect. If the two are not equal, the test will be marked as
// failed and debugging logging will be emitted
func checkJsonDecodePrint(expected string, str string, t *testing.T) {

	var value interface{}
	reader := strings.NewReader(str)
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()

	err := decoder.Decode(&value)
	if err != nil {
		t.Error("Unable to parse string into json:", err)
	}

	jsonStr := DispValue(value)
	testingtools.Equivalent(t, expected, jsonStr)
}

func TestPrinterParserCompability(t *testing.T) {
	// Checks that the output returned by the pretty print can be reparsed by
	// the standard go json library
	jsonValue := []interface{}{
		json.Number("1"),
		json.Number("2"),
		map[string]interface{}{
			"field1": []interface{}{
				"a",
				"b",
			},
			"field2": []interface{}{
				"c",
				"d",
			},
		}}

	prettyPrint := DispValue(jsonValue)
	var value interface{}
	reader := strings.NewReader(prettyPrint)
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()

	err := decoder.Decode(&value)
	if err != nil {
		t.Error("Unable to parse string into json:", err)
	}

	testingtools.Equivalent(t, jsonValue, value)
}
