// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tracing contains helper for reporting OpenTelemetry tracing spans.
package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("go.chromium.org/infra/qscheduler")

// Start opens a tracing span.
//
// Finish it with End, usually like this:
//
//	ctx, span := tracing.Start(ctx, "..."")
//	defer func() { tracing.End(span, err) }()
func Start(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

// End records the error (if any) and closes the span.
func End(span trace.Span, err error, attrs ...attribute.KeyValue) {
	span.SetAttributes(attrs...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}
