// Copyright 2022 The Chromium Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package weekly

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"text/template"

	"github.com/maruel/subcommands"
	"google.golang.org/grpc/credentials"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/monorail"
)

var (
	MonorailAPIURL = "https://monorail-prod.appspot.com/_ah/api/monorail/v1"
	markdownTempl  = template.Must(template.New("test").Parse(`
| Pri | Issue ID | Summary |
| --- | -------- | ------- |
{{- range .}}
| {{.Pri}} | [{{.ProjectID}}:{{.ID}}](https://bugs.chromium.org/p/{{.ProjectID}}/issues/detail?id={{.ID}}) | {{.Summary}} |
{{- end}}
	`))
)

// clientFactory encapsulates the dialing and caching of http transports.
type clientFactory struct {
	tlsCreds    credentials.TransportCredentials
	baseAuth    *auth.Authenticator
	idTokenAuth *auth.Authenticator

	initOnce sync.Once
}

func (f *clientFactory) init(ctx context.Context, opts auth.Options) {
	f.initOnce.Do(func() {
		f.tlsCreds = credentials.NewTLS(nil)

		f.baseAuth = auth.NewAuthenticator(ctx, auth.SilentLogin, opts)

		opts.UseIDTokens = true
		f.idTokenAuth = auth.NewAuthenticator(ctx, auth.InteractiveLogin, opts)
	})
}

func (f *clientFactory) http() (*http.Client, error) {
	return f.baseAuth.Client()
}

type reportCmd struct {
	subcommands.CommandRunBase
	clientFactory clientFactory
	authFlags     authcli.Flags
	params        Param

	days     int
	Monorail monorail.MonorailClient
}

func (r *reportCmd) httpClient(ctx context.Context) (*http.Client, error) {
	opts, err := r.authFlags.Options()
	if err != nil {
		return nil, err
	}
	r.clientFactory.init(ctx, opts)
	httpClient, err := r.clientFactory.http()
	switch {
	case errors.Is(err, auth.ErrLoginRequired):
		return nil, errors.New("Login required: run `weekly auth-login` or use the -service-account-json flag")
	case err != nil:
		return nil, err
	}
	return httpClient, nil
}

func (dc *reportCmd) RegisterFlags(p Param) {
	dc.Flags.IntVar(&dc.days, "days", 7, "number of days to look back")
	dc.authFlags.Register(&dc.Flags, p.Auth)
}

type tmplItem struct {
	ProjectID string
	ID        int32
	Summary   string
	Pri       string
}

func (dc *reportCmd) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, nil, env)
	htc, err := dc.httpClient(ctx)
	if err != nil {
		fmt.Fprintf(a.GetErr(), "ERROR: %s\n", err)
		return -1
	}
	dc.Monorail = monorail.NewEndpointsClient(htc, MonorailAPIURL)
	req := &monorail.IssuesListRequest{
		ProjectId: "chromium",
		Can:       monorail.IssuesListRequest_ALL,
		// This URL is borrowed from the link at go/berf-triage-queue and modified to:
		// - include all issue statuses
		// - limit results to issues modified in the past N (default 7) days as set by the -days CLI flag.
		Q: fmt.Sprintf("component:Speed>Dashboard,Speed>Bisection,Speed>Benchmarks>Waterfall pri:0,1,2,3 opened>2021-8-1 -label:Browser-Perf-EngProd modified>today-%d", dc.days),
	}
	resp, err := dc.Monorail.IssuesList(ctx, req)
	if err != nil {
		fmt.Fprintf(a.GetErr(), "ERROR: %s\n", err)
		return -1
	}
	data := []tmplItem{}
	for _, issue := range resp.Items {
		pri := ""
		for _, l := range issue.Labels {
			if strings.HasPrefix(l, "Pri-") {
				pri = l
			}
		}
		data = append(data, tmplItem{issue.ProjectId, issue.Id, issue.Summary, pri})
	}
	markdownTempl.Execute(a.GetOut(), data)
	if err != nil {
		fmt.Fprintf(a.GetErr(), "ERROR: %s\n", err)
		return -1
	}
	return 0
}

func cmdReportIncoming(p Param) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "report-incoming",
		ShortDesc: "Summarize issues from the past week's on-call rotation formatted as a markdown table.",
		CommandRun: func() subcommands.CommandRun {
			c := &reportCmd{params: p}
			c.RegisterFlags(p)
			return c
		},
	}
}
