// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package flagx contains custom CLI flag types for crosfleet.
package flagx

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
)

// int64SliceVar implements the flag.Value interface for an int64 slice flag.
type int64SliceVar struct {
	handle *[]int64
}

// Int64Slice takes an initial int64 slice and produces a flag variable that can
// be set from command line arguments.
func Int64Slice(m *[]int64) flag.Value {
	return int64SliceVar{handle: m}
}

// String returns the default value for an int64 slice represented as a string.
// The default value is nil, which stringifies to an empty string.
func (int64SliceVar) String() string {
	return ""
}

// Set populates the int64 slice from comma-separated string values.
func (is int64SliceVar) Set(values string) error {
	if values == "" {
		return nil
	}
	for _, v := range strings.Split(values, ",") {
		i, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return err
		}
		*is.handle = append(*is.handle, i)
	}
	fmt.Println(values)
	return nil
}
