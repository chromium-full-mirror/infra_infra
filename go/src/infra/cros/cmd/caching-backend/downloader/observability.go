// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.chromium.org/luci/common/tsmon"
	"go.chromium.org/luci/common/tsmon/field"
	"go.chromium.org/luci/common/tsmon/metric"
	"go.chromium.org/luci/common/tsmon/target"
	"go.chromium.org/luci/common/tsmon/types"
)

// metricsInit sets up the metrics.
func metricsInit(ctx context.Context, tsmonEndpoint, tsmonCredentialPath string) error {
	log.Printf("Setting up cache-downloader tsmon...")
	fl := tsmon.NewFlags()
	fl.Endpoint = tsmonEndpoint
	fl.Credentials = tsmonCredentialPath
	fl.Flush = tsmon.FlushAuto
	fl.Target.SetDefaultsFromHostname()
	fl.Target.TargetType = target.TaskType
	fl.Target.TaskServiceName = "cache-downloader"
	fl.Target.TaskJobName = "cache-downloader"

	if err := tsmon.InitializeFromFlags(ctx, &fl); err != nil {
		return fmt.Errorf("metrics: error setup tsmon: %w", err)
	}

	return nil
}

// updateMetrics add data points to the metrics.
func updateMetrics(ctx context.Context, rpc, method string, m *metricData, startTime time.Time) {
	dataDownloadBytes.Add(ctx, m.size, method, rpc, m.status)
	dataDownloadTime.Set(ctx, float64(time.Since(startTime).Seconds()), method, rpc, m.status)
	dataDownloadRate.Set(ctx, float64(float64(m.size)/time.Since(startTime).Seconds()), method, rpc, m.status)
}

// metricsShutdown stops the metrics.
func metricsShutdown(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	log.Printf("Shutting down metrics...")
	tsmon.Shutdown(ctx)
}

var (
	dataDownloadTime = metric.NewFloat("chromeos/fleet/caching-backend/downloader/time_download",
		"The total number of download time",
		&types.MetricMetadata{Units: types.Seconds},
		field.String("http_method"),
		field.String("rpc"),
		field.Int("status"))
	dataDownloadBytes = metric.NewCounter("chromeos/fleet/caching-backend/downloader/data_download",
		"The total number of download bytes",
		&types.MetricMetadata{Units: types.Bytes},
		field.String("http_method"),
		field.String("rpc"),
		field.Int("status"))
	dataDownloadRate = metric.NewFloat("chromeos/fleet/caching-backend/downloader/rate_download",
		"The download rate byte per second",
		&types.MetricMetadata{},
		field.String("http_method"),
		field.String("rpc"),
		field.Int("status"))
)

// traceInit sets up the request tracer.
func traceInit(ctx context.Context, endpoint string) {
	if endpoint != "" {
		tp, err := newTracerProvider(ctx, endpoint)
		if err != nil {
			log.Printf("tracer provider error: %s", err)
		} else {
			defer func() {
				if err := tp.Shutdown(ctx); err != nil {
					log.Printf("failed to shutdown tracer provider: %v", err)
				}
			}()
			log.Printf("Will post traces to %s", endpoint)
			otel.SetTracerProvider(tp)
			p := propagation.NewCompositeTextMapPropagator(
				propagation.TraceContext{}, propagation.Baggage{})
			otel.SetTextMapPropagator(p)
		}
	}
}

// newTraceProvider returns trace provider with given endpoint.
// endpoint can be a file or grpc type.
func newTracerProvider(ctx context.Context, endpoint string) (*sdktrace.TracerProvider, error) {
	r, err := newResource(ctx)
	if err != nil {
		return nil, fmt.Errorf("resource error: %w", err)
	}
	var exp sdktrace.SpanExporter
	exp, err = newGRPCExporter(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(r),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.5))),
	), nil
}

// newResource creates a new cache-downloader OTel resource.
func newResource(ctx context.Context) (*resource.Resource, error) {
	// This should never error.
	// Even if it does, try to keep running normally.
	return resource.New(
		ctx,
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String("cache-downloader"),
		),
	)
}

// newGRPCExporter returns a gRPC exporter.
func newGRPCExporter(ctx context.Context, target string) (sdktrace.SpanExporter, error) {
	conn, err := grpc.DialContext(ctx, target,
		// Connection is not secured.
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	return otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
}
