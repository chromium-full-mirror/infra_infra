// Copyright 2018 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cmdlib

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/errors"
)

// NewAuthenticator creates a new authenticator based on flags.
func NewAuthenticator(ctx context.Context, f *authcli.Flags) (*auth.Authenticator, error) {
	o, err := f.Options()
	if err != nil {
		return nil, errors.Annotate(err, "create authenticator").Err()
	}
	return auth.NewAuthenticator(ctx, auth.SilentLogin, o), nil
}

// NewHTTPClient returns an HTTP client with authentication set up.
func NewHTTPClient(ctx context.Context, f *authcli.Flags) (*http.Client, error) {
	a, err := NewAuthenticator(ctx, f)
	if err != nil {
		return nil, errors.Annotate(err, "failed to create HTTP client").Err()
	}
	c, err := a.Client()
	if err != nil {
		return nil, errors.Annotate(err, "failed to create HTTP client").Err()
	}
	return c, nil
}

// NewAuthenticatedTransport creates a new authenticated transport
func NewAuthenticatedTransport(ctx context.Context, f *authcli.Flags) (http.RoundTripper, error) {
	at, err := NewAuthenticator(ctx, f)
	if err != nil {
		return nil, errors.Annotate(err, "create authenticated transport").Err()
	}
	return at.Transport()
}

// UserErrorReporter reports a detailed error message to the user.
//
// PrintError() uses a UserErrorReporter to print multi-line user error details
// along with the actual error.
type UserErrorReporter interface {
	// Report a user-friendly error through w.
	ReportUserError(w io.Writer)
}

// PrintError reports errors back to the user.
//
// Detailed error information is printed if err is a UserErrorReporter.
func PrintError(a subcommands.Application, err error) {
	if u, ok := err.(UserErrorReporter); ok {
		u.ReportUserError(a.GetErr())
	} else {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
	}
}

// NewUsageError creates a new error that also reports flags usage error
// details.
func NewUsageError(flags flag.FlagSet, format string, a ...any) error {
	return &usageError{
		error: fmt.Errorf(format, a...),
		flags: flags,
	}
}

type usageError struct {
	error
	flags flag.FlagSet
	quiet bool
}

func (e *usageError) ReportUserError(w io.Writer) {
	fmt.Fprintf(w, "%s\n\nUsage:\n\n", e.error)
	if !e.quiet {
		e.flags.Usage()
	} else {
		fmt.Fprintf(w, "please run `$command -help` to check the usage\n")
	}
}

// NewQuietUsageError creates a new error that only reports flags usage error details
func NewQuietUsageError(flags flag.FlagSet, format string, a ...any) error {
	return &usageError{
		error: fmt.Errorf(format, a...),
		flags: flags,
		quiet: true,
	}
}
