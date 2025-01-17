// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policies

import "fmt"

// Convert the milestone + num into a useful regex for query.
func mileStoneRegex(numMileStones int, mileStone int) string {
	baseStr := fmt.Sprintf("%d", mileStone)
	for i := 1; i <= numMileStones; i++ {
		baseStr = fmt.Sprintf("%s|%s", baseStr, fmt.Sprintf("%d", mileStone-i))
	}
	return baseStr
}
