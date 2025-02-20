// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package middleware implements shared LUCI middleware and gRPC interceptors.
package middleware

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"go.chromium.org/luci/server/cron"
)

// Name used for OpenTelemetry tracers.
const tname = "go.chromium.org/infra/appengine/drone-queen/internal/middleware"

// A CronWrapper is a function that wraps a cron handler to provide
// middleware functionality.
type CronWrapper func(cron.Handler) cron.Handler

// CronTrace is a wrapper to add tracing to cron handlers.
// name is the name of the trace span.
func CronTrace(name string) CronWrapper {
	return func(h cron.Handler) cron.Handler {
		return func(ctx context.Context) error {
			var span trace.Span
			ctx, span = otel.Tracer(tname).Start(ctx, name, trace.WithSpanKind(trace.SpanKindServer))
			defer span.End()
			err := h(ctx)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
			return err
		}
	}
}
