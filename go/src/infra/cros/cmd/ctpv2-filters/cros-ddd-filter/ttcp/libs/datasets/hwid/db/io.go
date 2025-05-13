// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package db

import (
	"log"
	"runtime/debug"
)

func cast_to_string(data any) string {
	if data == nil {
		return ""
	} else {
		return data.(string)
	}
}

func castToMapIntString(data map[any]any) map[int]string {
	result := map[int]string{}
	for k, v := range data {
		result[k.(int)] = v.(string)
	}
	return result
}

func castToArrayInt(data []any) []int {
	result := []int{}
	for _, v := range data {
		result = append(result, v.(int))
	}
	return result
}

func castToStringArray(data any) []string {
	switch data := data.(type) {
	case nil:
		return []string{}
	case string:
		return []string{data}
	case []any:
		data_slice := data
		result := []string{}
		for _, v := range data_slice {
			result = append(result, v.(string))
		}
		return result
	default:
		debug.PrintStack()
		log.Fatal("Can't convert value to []string. Value received:", data)
	}
	return []string{}
}
