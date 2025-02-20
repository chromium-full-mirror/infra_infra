// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package externalclients

import (
	"context"

	"go.chromium.org/infra/libs/bqwrapper"
)

type key string

var BQClientKey key = "karte bigquery client"

// UseBQ installs the bigquery client on the context
func UseBQ(ctx context.Context, bqClient bqwrapper.BQIf) context.Context {
	return context.WithValue(ctx, BQClientKey, bqClient)
}

// GetBQ returns the BQ client from the context
func GetBQ(ctx context.Context) bqwrapper.BQIf {
	var zero bqwrapper.BQIf
	out, ok := ctx.Value(BQClientKey).(bqwrapper.BQIf)
	if ok {
		return out
	}
	return zero
}
