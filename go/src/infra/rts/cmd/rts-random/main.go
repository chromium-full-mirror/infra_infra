// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"math/rand"
	"time"

	"go.chromium.org/infra/rts/presubmit/eval"
)

func main() {
	ctx := context.Background()
	rand.Seed(time.Now().Unix())
	eval.Main(ctx, func(ctx context.Context, in eval.Input, out *eval.Output) error {
		for i := range in.TestVariants {
			out.TestVariantAffectedness[i].Distance = rand.Float64()
		}
		return nil
	})
}
