// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build !windows
// +build !windows

package main

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestParseLine(t *testing.T) {
	t.Parallel()
	tests := []struct {
		line string
		want *record
	}{
		{
			line: `{"access_time":"2021-06-09T13:24:39-07:00","bytes_sent":369,"content_length":369,"host":"100.115.168.189","method":"GET","proxy_host":"gs_archive_servers","referer":"","remote_addr":"127.0.0.1","remote_user":"","request":"GET /static/abc HTTP/1.1","request_time":0.123,"status":200,"uri":"/download/abc","user_agent":"curl","upstream":"","upstream_cache_status":"HIT","upstream_response_time":"","swarming_task_id": "id1","bbid": "id2","x_forwarded_for":""}`,
			want: &record{
				Timestamp:     time.Date(2021, 06, 9, 20, 24, 39, 0, time.UTC),
				ClientIP:      "127.0.0.1",
				HTTPMethod:    "GET",
				Path:          "/download/abc",
				Status:        200,
				BodyBytesSent: 369,
				ExpectedSize:  369,
				RequestTime:   0.123,
				CacheStatus:   "HIT",
				ProxyHost:     "gs_archive_servers",
				Host:          "100.115.168.189",
			},
		},
		{
			line: "an invalid json line",
			want: nil,
		},
		{
			line: `{"status":200,"uri":"/check_health_abc"}`,
			want: nil,
		},
		{
			line: `{"status":200,"uri":"/static/quick-provision"}`,
			want: nil,
		},
		{
			line: `{"status":200,"uri":"/download/download-test.txt"}`,
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			extraIgnoredPaths = []string{"/download/download-test.txt"}
			got := parseLine(tc.line)
			if diff := cmp.Diff(tc.want, got, cmp.AllowUnexported(record{})); diff != "" {
				t.Errorf("parseLine returned unexpected diff (-want +got):\n%s", diff)
			}
		})
	}
}
