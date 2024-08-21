// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package acl

import (
	"context"
	"fmt"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"
)

func TestRPCAccessInterceptor(t *testing.T) {
	t.Parallel()

	interceptor := RPCAccessInterceptor.Unary()

	check := func(ctx context.Context, service, method string) codes.Code {
		info := &grpc.UnaryServerInfo{
			FullMethod: fmt.Sprintf("/%s/%s", service, method),
		}
		_, err := interceptor(ctx, nil, info, func(context.Context, interface{}) (interface{}, error) {
			return nil, nil
		})
		return status.Code(err)
	}

	ftt.Run("Anonymous", t, func(t *ftt.Test) {
		ctx := auth.WithState(context.Background(), &authtest.FakeState{})

		assert.Loosely(t, check(ctx, "unknown.API", "Something"), should.Equal(codes.PermissionDenied))
		assert.Loosely(t, check(ctx, "grpc.reflection.v1alpha.ServerReflection", "Something"), should.Equal(codes.PermissionDenied))
		assert.Loosely(t, check(ctx, "grpc.reflection.v1.ServerReflection", "Something"), should.Equal(codes.PermissionDenied))
		assert.Loosely(t, check(ctx, "chromiumos.test.api.VMLeaserService", "Something"), should.Equal(codes.PermissionDenied))
	})

	ftt.Run("Authenticated, but not authorized", t, func(t *ftt.Test) {
		ctx := auth.WithState(context.Background(), &authtest.FakeState{
			Identity:       "user:someone@example.com",
			IdentityGroups: []string{"some-random-group"},
		})

		assert.Loosely(t, check(ctx, "unknown.API", "Something"), should.Equal(codes.PermissionDenied))
		assert.Loosely(t, check(ctx, "grpc.reflection.v1alpha.ServerReflection", "Something"), should.Equal(codes.PermissionDenied))
		assert.Loosely(t, check(ctx, "grpc.reflection.v1.ServerReflection", "Something"), should.Equal(codes.PermissionDenied))
		assert.Loosely(t, check(ctx, "chromiumos.test.api.VMLeaserService", "Something"), should.Equal(codes.PermissionDenied))
	})

	ftt.Run("Authorized", t, func(t *ftt.Test) {
		ctx := auth.WithState(context.Background(), &authtest.FakeState{
			Identity:       "user:someone@example.com",
			IdentityGroups: []string{VMLabGroup},
		})

		assert.Loosely(t, check(ctx, "unknown.API", "Something"), should.Equal(codes.PermissionDenied))
		assert.Loosely(t, check(ctx, "grpc.reflection.v1alpha.ServerReflection", "Something"), should.Equal(codes.OK))
		assert.Loosely(t, check(ctx, "grpc.reflection.v1.ServerReflection", "Something"), should.Equal(codes.OK))
		assert.Loosely(t, check(ctx, "chromiumos.test.api.VMLeaserService", "Something"), should.Equal(codes.OK))
	})
}
