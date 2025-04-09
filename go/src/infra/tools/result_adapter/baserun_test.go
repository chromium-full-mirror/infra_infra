// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	grpc "google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/structpb"

	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	sinkpb "go.chromium.org/luci/resultdb/sink/proto/v1"
)

type mockSinkClient struct {
	sinkpb.SinkClient
	req *sinkpb.UpdateInvocationRequest
}

func (c *mockSinkClient) UpdateInvocation(ctx context.Context, in *sinkpb.UpdateInvocationRequest, opts ...grpc.CallOption) (*sinkpb.Invocation, error) {
	c.req = in
	return &sinkpb.Invocation{}, nil
}

// TestCmd is a smoke test for the subcommand code. It just runs it to make sure
// it doesn't panic.
func TestBaseRun(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	s := &mockSinkClient{}
	r := &baseRun{
		sinkC: s,
	}

	t.Run("reportError", func(t *testing.T) {
		r.reportException(ctx, errors.New("this is an error"))
		// fmt.Println(protojson.Format(s.req))
		exception, err := structpb.NewStruct(map[string]any{
			"@type": "type.googleapis.com/build.util.lib.proto.ExceptionOccurrences",
			"datapoints": []any{
				map[string]any{
					"name":          "this is an error",
					"occurred_time": "<ANY-STRING-VALUE>",
					"stacktrace":    []any{"this is an error"},
				},
			},
		})
		assert.NoErr(t, err)
		assert.Loosely(t, s.req, should.Match(&sinkpb.UpdateInvocationRequest{
			Invocation: &sinkpb.Invocation{
				ExtendedProperties: map[string]*structpb.Struct{
					"exception_occurrences": exception,
				},
			},
			UpdateMask: &fieldmaskpb.FieldMask{
				Paths: []string{"extended_properties.exception_occurrences"},
			},
		}, cmp.FilterPath(func(p cmp.Path) bool {
			// We would accept any non-empty value for occurredTime
			return strings.Contains(p.GoString(), "occurred_time")
		}, cmp.Comparer(func(a, b string) bool {
			return len(a) > 0 && len(b) > 0
		}))))
	})
}
