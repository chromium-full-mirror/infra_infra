// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package provides a wrapper to specify local dev options.
package dev

import (
	"context"
)

// localDevKeyType is a unique type for a context key.
type localDevKeyType string

const (
	localDevOptionsKey localDevKeyType = "local_developing_options"
)

// WithDevOptions sets DevOptions to the context.
func WithDevOptions(ctx context.Context, devOptions any) context.Context {
	if devOptions == nil {
		panic("logger is not provided")
	}
	return context.WithValue(ctx, localDevOptionsKey, devOptions)
}

// ActiveLocalDevOption represent interface to tell if local development is active.
type ActiveLocalDevOption interface {
	// Specify if expected to print DUT proto at the start/end of execution.
	PrintDUTProtos() bool
	// Specify if client is active.
	IsActive() bool
}

// HasDevOptions tells if dev options specified.
func HasDevOptions(ctx context.Context) bool {
	return ctx.Value(localDevOptionsKey) != nil
}

// IsActive specifies if local dev option is active.
// Default: No.
func IsActive(ctx context.Context) bool {
	if o, ok := ctx.Value(localDevOptionsKey).(ActiveLocalDevOption); ok {
		return o.IsActive()
	}
	return false
}

// PrintDUTProtos specifies if printing DUT protos is expected.
// Default: Yes.
func PrintDUTProtos(ctx context.Context) bool {
	if o, ok := ctx.Value(localDevOptionsKey).(ActiveLocalDevOption); ok {
		return o.PrintDUTProtos()
	}
	return true
}
