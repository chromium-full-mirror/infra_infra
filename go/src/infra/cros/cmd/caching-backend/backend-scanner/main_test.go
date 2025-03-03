// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"log"
	"regexp"
	"strings"
	"testing"
)

func TestConfigTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		template  string
		data      interface{}
		wantLines []string
	}{
		{
			name:     "nginx",
			template: nginxTemplate,
			data: &nginxConf{
				CacheSizeInGB:     100,
				Port:              1234,
				L7Port:            4321,
				L7Servers:         []Backend{{"1.1.1.1", false}, {"2.2.2.2", false}},
				OtelTraceEndpoint: "http://localhost:5678",
			},
			wantLines: []string{
				"max_size=100g",
				`server 1.1.1.1:4321;[\r\n\s]+server 2.2.2.2:4321;`,
				"listen \\*:1234", // this is a regexp, so escape the '*'.
			},
		},
		{
			name:     "keepalived",
			template: keepalivedTempalte,
			data: &keepalivedConf{
				ServiceIP:   "8.8.8.8",
				ServicePort: 9999,
				RealServers: []Backend{{"1.1.1.1", false}, {"1.1.2.2", true}, {"2.2.2.2", false}},
				Interface:   "eth0",
				LBAlgo:      "wlc",
			},
			wantLines: []string{
				"interface eth0",
				"8.8.8.8",
				"virtual_server 8.8.8.8 9999{",
				`real_server 1.1.1.1 9999{\n\s+weight 1`,
				"connect_port 9999",
				`real_server 1.1.2.2 9999{\n\s+weight 0`,
				`real_server 2.2.2.2 9999{\n\s+weight 1`,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := genConfig("nginx", tc.template, tc.data)
			if err != nil {
				t.Errorf("genConfig(%q) err %q, want nil", tc.name, err)
			}
			// (?ms) means multiline mode and make '.' matches newline.
			re := "(?ms)" + strings.Join(tc.wantLines, ".*")
			match, err := regexp.MatchString(re, got)
			if err != nil {
				t.Errorf("genConfig(%q) err %q, want nil", tc.name, err)
			}
			if !match {
				log.Printf("%s", got)
				t.Errorf("genConfig(%q) got %q, doesn't match %q", tc.name, got, re)
			}
		})
	}
}
