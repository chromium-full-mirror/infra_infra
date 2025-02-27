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
// numberOfArgs is the number of args in each parenthesis (3 in the example above)
//
// If lenValues is not cleanly divisible by numberOfArgs the remaining values will be ignored:
// E.G: ValuesString(5, 2) = "($1, $2), ($3, $4)"
func ValuesString(lenValues int, numberOfArgs int) string {
	values := make([]string, lenValues/numberOfArgs)

	for i := 0; i < lenValues/numberOfArgs; i++ {
		inner := make([]string, numberOfArgs)
		for j := 0; j < numberOfArgs; j++ {
			inner[j] = fmt.Sprintf("$%d", j+i*numberOfArgs+1)
		}
		values[i] = "(" + strings.Join(inner, ", ") + ")"
	}
	return strings.Join(values, ", ")
}
