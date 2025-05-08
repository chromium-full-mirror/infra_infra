// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package otil contains OpenTelemetry utilities.
//
// These utilities are for convenience and do not have to be used.
// The OpenTelemetry API is canonical, and this package interoperates
// with it.
//
// To add instrumentation to a function:
//
//	func MyFunc(ctx context.Context) (err error) {
//		ctx, span := otil.FuncSpan(ctx)
//		defer func() { otil.EndSpan(span, err) }()
//		// rest of function
//	}
package otil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"runtime"

	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"
)

// Name used for OpenTelemetry tracers.
const tname = "go.chromium.org/infra/libs/otil"

// ValuesKey represents a slice of string values to attach to a span or event.
// This can be the arguments to a function or values relevant to an event.
var ValuesKey = attribute.Key("org.chromium.code.values")

// AddValues adds a "values" attribute to a span.
func AddValues(span trace.Span, v ...any) {
	if !span.SpanContext().IsSampled() {
		return
	}
	args := make([]string, len(v))
	for i, v := range v {
		switch v := v.(type) {
		case string:
			args[i] = v
		default:
			args[i] = fmt.Sprintf("%v", v)
		}
	}
	span.SetAttributes(ValuesKey.StringSlice(args))
}

// FuncSpan creates a span for the calling function.
// This function adds metadata (function name, filename, etc) to the
// span by inspecting the call stack.
func FuncSpan(ctx context.Context, o ...trace.SpanStartOption) (context.Context, trace.Span) {
	ctx, span := otel.Tracer(tname).Start(ctx, "unknownFuncSpan", o...)
	if !span.SpanContext().IsSampled() {
		return ctx, span
	}
	_, span2 := otel.Tracer(tname).Start(ctx, "runtime.Caller")
	pc, file, line, ok := runtime.Caller(1)
	if !ok {
		span2.AddEvent("runtime.Caller error")
		span2.SetStatus(codes.Error, "runtime.Caller error")
		span2.End()
		return ctx, span
	}
	span2.End()
	span.SetName(runtime.FuncForPC(pc).Name())
	span.SetAttributes(
		semconv.CodeFunctionKey.String(runtime.FuncForPC(pc).Name()),
		semconv.CodeFilepathKey.String(file),
		semconv.CodeLineNumberKey.Int(line),
	)
	return ctx, span
}

// EndSpan ends a span and provides some convenience for handling errors.
func EndSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

// AddHTTP adds tracing and context propagation integration to an HTTP client.
// The context provides the parent span for any child request spans.
func AddHTTP(c *http.Client) {
	c.Transport = otelhttp.NewTransport(c.Transport, otelhttp.WithClientTrace(func(ctx context.Context) *httptrace.ClientTrace {
		return otelhttptrace.NewClientTrace(ctx)
	}))
}
