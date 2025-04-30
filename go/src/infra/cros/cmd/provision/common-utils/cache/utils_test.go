// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cache

import (
	"strings"
	"testing"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
)

func TestIPEndpointToHostPort(t *testing.T) {
	tests := []struct {
		name      string
		ip        *labapi.IpEndpoint
		want      string
		wantErr   bool
		errSubstr string
	}{
		{
			name: "valid endpoint",
			ip:   &labapi.IpEndpoint{Address: "192.168.1.1", Port: 8080},
			want: "192.168.1.1:8080",
		},
		{
			name:      "missing address",
			ip:        &labapi.IpEndpoint{Port: 8080},
			wantErr:   true,
			errSubstr: "missing address",
		},
		{
			name:      "missing port",
			ip:        &labapi.IpEndpoint{Address: "192.168.1.1"},
			wantErr:   true,
			errSubstr: "missing port",
		},
		{
			name:      "nil endpoint",
			ip:        nil,
			wantErr:   true,              // Will cause nil pointer dereference in function, caught by test check
			errSubstr: "missing address", // Function checks address first
		},
		{
			name:      "empty endpoint",
			ip:        &labapi.IpEndpoint{},
			wantErr:   true,
			errSubstr: "missing address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := IPEndpointToHostPort(tt.ip); (err != nil) != tt.wantErr {
				t.Errorf("IPEndpointToHostPort() error = %v, wantErr %v", err, tt.wantErr)
				return
			} else if err != nil && tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
				t.Errorf("IPEndpointToHostPort() error = %q, want error containing %q", err.Error(), tt.errSubstr)
			} else if !tt.wantErr && got != tt.want {
				t.Errorf("IPEndpointToHostPort() = %v, want %v", got, tt.want)
			}
		})
	}
}
