// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package json

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"sort"
	"strings"
)

// DispValue takes a unstructured marshaled json structure and returns a string
// with the pretty print of the structure.	The pretty print format is optimized
// for readability and compactness.
//
// Note that the Go's standard json library parses any number into a float64,
// This can result in some undesired changes. To avoid this use
// func (*Decoder) UseNumber()
func DispValue(v any) string {
	var dest strings.Builder
	dispValueInt(&dest, v, "", "", "")
	return dest.String()
}

func dispValueInt(
	dest *strings.Builder,
	toPrint any,
	firstLineIndent string,
	nextLineIndent string,
	postfix string,
) {
	switch toPrint := toPrint.(type) {
	case map[string]any:
		dispValueObject(
			dest,
			toPrint,
			firstLineIndent,
			nextLineIndent,
			postfix)
	case []any:
		dispValueArray(
			dest,
			toPrint,
			firstLineIndent,
			nextLineIndent,
			postfix)
	case string:
		dest.WriteString(
			fmt.Sprintf(
				"%s\"%s\"%s",
				firstLineIndent,
				toPrint,
				postfix))
	case json.Number:
		number := toPrint
		dest.WriteString(
			fmt.Sprintf(
				"%s%s%s",
				firstLineIndent,
				number.String(),
				postfix))
	case int:
		dest.WriteString(
			fmt.Sprintf(
				"%s%d%s",
				firstLineIndent,
				toPrint,
				postfix))
	case float64:
		dest.WriteString(
			fmt.Sprintf(
				"%s%g%s",
				firstLineIndent,
				toPrint,
				postfix))
	case bool:
		if toPrint {
			dest.WriteString(
				fmt.Sprintf(
					"%strue%s",
					firstLineIndent,
					postfix))
		} else {
			dest.WriteString(
				fmt.Sprintf(
					"%sfalse%s",

					firstLineIndent,
					postfix))
		}
	case nil:
		dest.WriteString(fmt.Sprintf("%snull%s", firstLineIndent, postfix))
	default:
		log.Fatal(fmt.Sprint("unhandled type:", reflect.TypeOf(toPrint)))
	}
}

func dispValueObject(
	dest *strings.Builder,
	objToPrint map[string]any,
	firstLineIndent string,
	nextLineIndent string,
	postfix string,
) {
	maxKeyLen := 0
	nbKeys := len(objToPrint)
	keys := make([]string, nbKeys)
	i := 0
	for k := range objToPrint {
		key := string(k)
		keys[i] = key
		kLen := len(key)
		if kLen > maxKeyLen {
			maxKeyLen = kLen
		}
		i++
	}
	keyIndex := 0
	// We sort the entries of a map to increase readability
	sort.SliceStable(keys, func(k1 int, k2 int) bool {
		return keys[k1] < keys[k2]
	})
	for _, key := range keys {
		value := objToPrint[key]
		var fieldFirstLineIndent string
		if keyIndex == 0 {
			fieldFirstLineIndent = fmt.Sprintf("%s{ \"%s\"%s : ",
				firstLineIndent,
				key,
				strings.Repeat(" ", maxKeyLen-len(key)))
		} else {
			dest.WriteString("\n")
			fieldFirstLineIndent = fmt.Sprintf("%s  \"%s\"%s : ",
				nextLineIndent,
				key,
				strings.Repeat(" ", maxKeyLen-len(key)))
		}

		fieldNextLineIndent := fmt.Sprintf("%s%s",
			nextLineIndent,
			strings.Repeat(" ", len(fieldFirstLineIndent)-len(nextLineIndent)))

		keyIndex = keyIndex + 1
		if keyIndex == nbKeys {
			dispValueInt(
				dest,
				value,
				fieldFirstLineIndent,
				fieldNextLineIndent,
				"")
		} else {
			dispValueInt(
				dest,
				value,
				fieldFirstLineIndent,
				fieldNextLineIndent,
				",")
		}
	}
	if keyIndex == 0 {
		dest.WriteString(fmt.Sprintf("%s{}", firstLineIndent))
	} else {
		dest.WriteString(fmt.Sprintf("}%s", postfix))
	}
}

func dispValueArray(
	dest *strings.Builder,
	array []any,
	firstLineIndent string,
	nextLineIndent string,
	postfix string,
) {
	nextIndent := nextLineIndent + "  "
	firstLine := true
	if len(array) == 0 {
		dest.WriteString(fmt.Sprintf("%s%s", firstLineIndent, "[]"))
	} else {
		for index, item := range array {
			var postfix string
			if index == len(array)-1 {
				postfix = ""
			} else {
				postfix = ","
			}
			if firstLine {
				dispValueInt(dest, item, firstLineIndent+"[ ", nextIndent, postfix)
			} else {
				dest.WriteString("\n")
				dispValueInt(dest, item, nextIndent, nextIndent, postfix)
			}
			firstLine = false
		}
		dest.WriteString(fmt.Sprintf("]%s", postfix))
	}
}

func FormatStructAsJson(st any) (string, error) {
	b, err := json.MarshalIndent(st, "", "    ")
	if err != nil {
		return "", fmt.Errorf("Error in marshaling results to json:%s", err)
	}
	return string(b), nil
}
