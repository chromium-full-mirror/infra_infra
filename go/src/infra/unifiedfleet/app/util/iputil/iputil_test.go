// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iputil

import (
	"fmt"
	"math/big"
	"net"
	"testing"

	"go.chromium.org/luci/common/testing/typed"
)

func TestIncrByte(t *testing.T) {
	x, overflow := incrByte(255)
	if x != 0 {
		t.Error("255 should overflow to 0")
	}
	if !overflow {
		t.Error("255 should overflow")
	}
}

func TestRawIncr(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		ip       net.IP
		want     net.IP
		overflow bool
	}{
		{
			name:     "basic increment",
			ip:       net.IPv4(127, 0, 0, 1),
			want:     net.IPv4(127, 0, 0, 2),
			overflow: false,
		},
		{
			name:     "edge of ipv4 space",
			ip:       net.IPv4(255, 255, 255, 255),
			want:     MustParseIP("::1:0:0:0"),
			overflow: false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, overflow := RawIncr(tt.ip)

			if diff := typed.Diff(got, tt.want); diff != "" {
				t.Errorf("unexpected diff (-want +got): %s", diff)
			}
			if diff := typed.Diff(overflow, tt.overflow); diff != "" {
				t.Errorf("unexpected diff (-want +got): %s", diff)
			}
		})
	}
}

// TestPad that pad correctly adds and truncates paths.
func TestPad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		n    int
		out  string
	}{
		{
			name: "happy path",
			in:   "ab",
			n:    1,
			out:  "b",
		},
		{
			name: "less trivial happy path",
			in:   "abcdef",
			n:    3,
			out:  "def",
		},
		{
			name: "null bytes",
			in:   "abc",
			n:    4,
			out:  "\x00abc",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := string(pad([]byte(tt.in), tt.n))
			if diff := typed.Diff(got, tt.out); diff != "" {
				t.Errorf("unexpected diff (-want +got): %s", diff)
			}
		})
	}
}

// TestAddIP tests performing IP arithmetic.
func TestAddIP(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		ip     net.IP
		offset int64
		want   net.IP
		ok     bool
	}{
		{
			name:   "0.0.0.1 increment",
			ip:     MustParseIP("::1"),
			offset: 1,
			want:   MustParseIP("::2"),
			ok:     true,
		},
		{
			name:   "0.0.0.1 decrement",
			ip:     MustParseIP("::1"),
			offset: -1,
			want:   MustParseIP("::"),
			ok:     true,
		},
		{
			name:   "0.0.0.1 decerment by too much",
			ip:     MustParseIP("::1"),
			offset: -2,
			want:   net.IP{},
			ok:     false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := AddToIP(tt.ip, big.NewInt(tt.offset))

			if diff := typed.Diff(got.String(), tt.want.String()); diff != "" {
				t.Errorf("unexpected diff (-want +got): %s", diff)
			}
			if diff := typed.Diff(got != nil, tt.ok); diff != "" {
				t.Errorf("unexpected diff (-want +got): %s", diff)
			}
		})
	}
}

func TestValidateSameFamily(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		ips  []net.IP
		ok   bool
	}{
		{
			name: "empty",
			ips:  nil,
			ok:   true,
		},
		{
			name: "singleton",
			ips:  []net.IP{MustParseIP("127.0.0.1")},
			ok:   true,
		},
		{
			name: "mismatch",
			ips: []net.IP{
				MustParseIP("127.0.0.1"),
				MustParseIP("aaaa::7"),
			},
			ok: false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateSameFamily(tt.ips...)
			switch {
			case err == nil && !tt.ok:
				t.Error("error is unexpectedly nil")
			case err != nil && tt.ok:
				t.Errorf("unexpected error: %s", err)
			}
		})
	}
}

// TestIPDiff tests taking differences of IPs.
func TestIPDiff(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		x          net.IP
		y          net.IP
		difference string
	}{
		{
			name:       "simple diff",
			x:          MustParseIP("127.0.0.2"),
			y:          MustParseIP("127.0.0.1"),
			difference: "1",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IPDiff(tt.x, tt.y).String()
			if diff := typed.Diff(got, tt.difference); diff != "" {
				t.Errorf("unexpected diff (-want +got): %s", diff)
			}
		})
	}
}

func TestIPIterSmokeTest(t *testing.T) {
	t.Parallel()

	tally := 0
	err := IPIter(MustParseIP("::1"), MustParseIP("::3"), func(ip net.IP) error {
		tally++
		fmt.Printf("%s\n", ip.String())
		return nil
	})

	if diff := typed.Diff(3, tally); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
}
