// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gitiles

import (
	"fmt"
	"net/http"

	"golang.org/x/time/rate"

	"go.chromium.org/luci/common/api/gitiles"
)

// NewThrottlingClient creates REST Gitiles client and consumes limiter quota
// on each API call to Gitiles. If there is no quota left, it blocks until
// there is.
func NewThrottlingClient(host string, limiter *rate.Limiter) (Client, error) {
	c, err := gitiles.NewRESTClient(&http.Client{}, host, false)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", "couldn't initialize Gitiles REST client", err)
	}
	return c, err
}
