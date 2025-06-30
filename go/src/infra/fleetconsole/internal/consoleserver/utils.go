// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package consoleserver

import "go.chromium.org/infra/fleetconsole/internal/utils"

func pageTokenToOffset(token string, filter string, orderBy string) (int, error) {
	return utils.PageTokenToOffset(token, []string{
		filter,
		orderBy,
	})
}

func offsetToPageToken(offset int, filter string, orderBy string) (string, error) {
	return utils.OffsetToPageToken(offset, []string{
		filter,
		orderBy,
	})
}
