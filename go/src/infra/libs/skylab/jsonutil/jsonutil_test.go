// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package jsonutil

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
)

func TestParseJSONProto(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   []byte
		msg  proto.Message
		out  []proto.Message
		ok   bool
	}{
		{
			name: "empty",
			in:   []byte(""),
			msg:  nil,
			out:  nil,
			ok:   false,
		},
		{
			name: "single asset",
			in:   []byte(`{"name": "a"}`),
			msg:  &ufspb.Asset{},
			out: []proto.Message{
				&ufspb.Asset{
					Name: "a",
				},
			},
			ok: true,
		},
		{
			name: "singleton list of assets",
			in:   []byte(`[{"name": "a"}]`),
			msg:  &ufspb.Asset{},
			out: []proto.Message{
				&ufspb.Asset{
					Name: "a",
				},
			},
			ok: true,
		},
		{
			name: "two assets",
			in:   []byte(`[{"name": "a"}, {"name": "b"}]`),
			msg:  &ufspb.Asset{},
			out: []proto.Message{
				&ufspb.Asset{
					Name: "a",
				},
				&ufspb.Asset{
					Name: "b",
				},
			},
			ok: true,
		},
		{
			name: "three assets",
			in:   []byte(`[{"name": "a"}, {"name": "b"}, {"name": "c"}]`),
			msg:  &ufspb.Asset{},
			out: []proto.Message{
				&ufspb.Asset{
					Name: "a",
				},
				&ufspb.Asset{
					Name: "b",
				},
				&ufspb.Asset{
					Name: "c",
				},
			},
			ok: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, e := ParseJSONProto(tt.in, tt.msg)
			ok := e == nil
			if diff := cmp.Diff(tt.out, out, protocmp.Transform()); diff != "" {
				t.Errorf("unexpected diff (-want +got): %s", diff)
			}
			if diff := cmp.Diff(tt.ok, ok); diff != "" {
				t.Errorf("unexpected diff (-want +got): %s", diff)
				if tt.ok {
					t.Errorf("error is %q", e.Error())
				}
			}
		})
	}
}

func TestSegmentJSONArray(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in  string
		out [][]byte
		err string
	}{
		{
			in:  `["a"]`,
			out: segmentedBytestring(`"a"`),
			err: "",
		},
		{
			in:  `[1, 2, 3, 4]`,
			out: segmentedBytestring("1", "2", "3", "4"),
			err: "",
		},
		{
			in:  `[[], [[]]]`,
			out: segmentedBytestring("[]", "[[]]"),
			err: "",
		},
	}

	for _, tt := range cases {
		t.Run(tt.in, func(t *testing.T) {
			out, err := segmentJSONArray([]byte(tt.in))
			e := errorToString(err)
			if diff := cmp.Diff(tt.out, out); diff != "" {
				t.Errorf("unexpected diff (-want +got): %s", diff)
			}
			if diff := cmp.Diff(tt.err, e); diff != "" {
				t.Errorf("unexpected diff (-want +got): %s", diff)
			}
		})
	}
}

func segmentedBytestring(xs ...string) [][]byte {
	out := [][]byte{}
	for _, x := range xs {
		out = append(out, []byte(x))
	}
	return out
}

func errorToString(e error) string {
	if e == nil {
		return ""
	}
	if e.Error() == "" {
		panic("non-nil error with empty message!")
	}
	return e.Error()
}
