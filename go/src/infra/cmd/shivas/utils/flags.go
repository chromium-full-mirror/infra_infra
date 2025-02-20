// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"flag"
	"strings"

	"go.chromium.org/infra/cmdsupport/cmdlib"
)

// CSVStringFlag is a flag.Getter implementation representing a []string.
type CSVStringFlag []string

// String returns a comma-separated string representation of the flag values.
func (f CSVStringFlag) String() string {
	return strings.Join(f, ", ")
}

// Set records seeing a flag value.
func (f *CSVStringFlag) Set(val string) error {
	// Split the values if they contain a comma
	if strings.Contains(val, ",") {
		*f = append(*f, strings.Split(val, ",")...)
	} else {
		*f = append(*f, val)
	}
	return nil
}

// Get retrieves the flag value.
func (f CSVStringFlag) Get() interface{} {
	return []string(f)
}

// CSVString returns a flag.Getter which reads flags into the given []string pointer.
func CSVString(s *[]string) flag.Getter {
	return (*CSVStringFlag)(s)
}

// CSVStringListFlag is a flag.Getter implementation representing a [][]string.
type CSVStringListFlag [][]string

// String returns a comma-separated string representation of the flag values separated by semicolon.
func (f CSVStringListFlag) String() string {
	var innerStrings []string
	for _, strList := range f {
		innerStrings = append(innerStrings, strings.Join(strList, ","))
	}
	return strings.Join(innerStrings, "; ")
}

// Set records seeing a flag value.
func (f *CSVStringListFlag) Set(val string) error {
	// Split the values if they contain a comma
	if strings.Contains(val, ",") {
		*f = append(*f, strings.Split(val, ","))
	} else {
		*f = append(*f, []string{val})
	}
	return nil
}

// Get retrieves the flag value.
func (f CSVStringListFlag) Get() interface{} {
	return [][]string(f)
}

// CSVStringList returns a flag.Getter which reads flags into the given []string pointer.
func CSVStringList(s *[][]string) flag.Getter {
	return (*CSVStringListFlag)(s)
}

// ValidateNameAndPositionalArg validates that name and positional args cannot exist/miss together
func ValidateNameAndPositionalArg(flags flag.FlagSet, name string) error {
	if flags.NArg() > 0 && name != "" {
		return cmdlib.NewUsageError(flags, "flag `-name` or positional arguments cannot be used simultaneously")
	}
	return nil
}
