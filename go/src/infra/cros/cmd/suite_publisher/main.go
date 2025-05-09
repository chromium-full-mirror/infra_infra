// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"log"
	"os"

	"cloud.google.com/go/bigquery"
	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/hardcoded/chromeinfra"
)

var (
	// StdoutLog contains the stdout logger for this package.
	StdoutLog *log.Logger
	// StderrLog contains the stderr logger for this package.
	StderrLog *log.Logger
)

// LogOut logs to stdout.
func LogOut(format string, a ...any) {
	if StdoutLog != nil {
		StdoutLog.Printf(format, a...)
	}
}

// LogErr logs to stderr.
func LogErr(format string, a ...any) {
	if StderrLog != nil {
		StderrLog.Printf(format, a...)
	}
}

func SetUpLogging(a subcommands.Application) {
	StdoutLog = a.(*suitePublisherApplication).stdoutLog
	StderrLog = a.(*suitePublisherApplication).stderrLog
}

// GetApplication returns an instance of the application.
func GetApplication(authOpts auth.Options) *subcommands.DefaultApplication {
	return &subcommands.DefaultApplication{
		Name: "suite_publisher",
		Commands: []*subcommands.Command{
			authcli.SubcommandInfo(authOpts, "auth-info", false),
			authcli.SubcommandLogin(authOpts, "auth-login", false),
			authcli.SubcommandLogout(authOpts, "auth-logout", false),
			cmdSuitePublisher(authOpts),
		},
	}
}

type suitePublisherApplication struct {
	*subcommands.DefaultApplication
	stdoutLog *log.Logger
	stderrLog *log.Logger
}

func main() {
	opts := chromeinfra.DefaultAuthOptions()
	scopes := []string{
		auth.OAuthScopeEmail,
		bigquery.Scope,
	}
	opts.Scopes = scopes
	s := &suitePublisherApplication{
		GetApplication(opts),
		log.New(os.Stdout, "", log.LstdFlags),
		log.New(os.Stderr, "", log.LstdFlags)}
	SetUpLogging(s)
	os.Exit(subcommands.Run(s, nil))
}
