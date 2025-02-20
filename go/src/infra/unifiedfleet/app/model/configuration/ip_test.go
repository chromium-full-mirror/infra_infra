// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package configuration

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
)

func TestBatchUpdateIPs(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	ftt.Run("happy path", t, func(t *ftt.Test) {
		count := 10
		ips := mockIps(count)

		resp, err := BatchUpdateIPs(ctx, ips)

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.HaveLength(len(ips)))

		getRes, _, err := ListIPs(ctx, 10, "", nil, false)

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, getRes, should.Match(ips))
	})
	ftt.Run("happy path - Updates multiple batches of IPs", t, func(t *ftt.Test) {
		count := 700
		ips := mockIps(count)

		resp, err := BatchUpdateIPs(ctx, ips)

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.HaveLength(len(ips)))

		getRes, _, err := ListIPs(ctx, 700, "", nil, false)

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, getRes, should.HaveLength(count))
	})
}

// TestGetProtos tests converting an IPv6 entity to a proto.
func TestGetProtos(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input *IPEntity
		want  *ufspb.IP
	}{
		{
			name: "sample",
			input: &IPEntity{
				ID:       "fake-vlan:hi/whatever",
				IPv4:     1,
				IPv4Str:  "0.0.0.1",
				IPv6:     []byte("\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x01"),
				Vlan:     "fake-vlan",
				Occupied: true,
				Reserve:  true,
			},
			want: &ufspb.IP{
				Id:       "fake-vlan:hi/whatever",
				Ipv4:     1,
				Ipv6:     []byte("\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x01"),
				Vlan:     "fake-vlan",
				Ipv4Str:  "0.0.0.1",
				Ipv6Str:  "::1",
				Occupied: true,
				Reserve:  true,
			},
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := tt.input.GetProto()
			if err != nil {
				t.Error(err)
			}
			if diff := cmp.Diff(got, tt.want, protocmp.Transform()); diff != "" {
				t.Errorf("unexpected diff (-want +got): %s", diff)
			}
		})
	}
}

func mockIps(count int) []*ufspb.IP {
	protos := make([]*ufspb.IP, count)
	for i := 0; i < count; i++ {
		protos[i] = &ufspb.IP{
			Id:      fmt.Sprint(i),
			Ipv4:    1111,
			Vlan:    "vlan" + fmt.Sprint(i),
			Ipv4Str: "1111",
			Ipv6:    nil,
			Ipv6Str: "",
		}
	}
	return protos
}
