// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package queryutils

import (
	"fmt"
	"strings"
)

// Returns a string in the shape of "($1, $2, $3), ($4, $5, $6)"
// lenValues is the total number of values (6 in the example above)
// numberOfArgs is the number of args in each parenthesis (3 in the example above).
// If numberOfArgs is 0 then all the values will be in a single parenthesis
// E.G: ValuesString(5, 0) = "($1, $2, $3, $4, $5)"
//
// If lenValues is not cleanly divisible by numberOfArgs the remaining values will be ignored:
// E.G: ValuesString(5, 2) = "($1, $2), ($3, $4)"
func ValuesString(lenValues int, numberOfArgs int) string {
	return ValuesStringWithOffset(lenValues, numberOfArgs, 0)
}

// ValuesStringWithOffset works just like [ValuesString] but you can also provide an offset
// E.G: ValuesStringWithOffset(4, 2, 100) = "($101, $102), ($103, $104)"
func ValuesStringWithOffset(lenValues int, numberOfArgs int, offset int) string {
	if lenValues == 0 {
		return "()"
	}

	if numberOfArgs == 0 {
		numberOfArgs = lenValues
	}

	values := make([]string, lenValues/numberOfArgs)

	for i := range lenValues / numberOfArgs {
		inner := make([]string, numberOfArgs)
		for j := range numberOfArgs {
			inner[j] = fmt.Sprintf("$%d", j+i*numberOfArgs+1+offset)
		}
		values[i] = "(" + strings.Join(inner, ", ") + ")"
	}
	return strings.Join(values, ", ")
}
