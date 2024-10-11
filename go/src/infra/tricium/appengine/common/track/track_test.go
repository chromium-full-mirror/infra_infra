// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package track

import (
	"context"
	"testing"

	"github.com/golang/protobuf/jsonpb"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	tricium "infra/tricium/api/v1"
)

func TestComment_UnpackComment(t *testing.T) {
	ftt.Run("Test", t, func(t *ftt.Test) {
		ctx := context.Background()
		out := tricium.Data_Comment{}
		t.Run("success", func(t *ftt.Test) {
			c := Comment{}
			data := tricium.Data_Comment{}
			s, err := (&jsonpb.Marshaler{}).MarshalToString(&data)
			assert.Loosely(t, err, should.BeNil)
			c.Comment = []byte(s)
			assert.Loosely(t, c.UnpackComment(ctx, &out), should.BeNil)
		})
		t.Run("failures", func(t *ftt.Test) {
			c := Comment{}
			assert.Loosely(t, c.UnpackComment(ctx, &out), should.NotBeNil)
			c.Comment = []byte{0}
			assert.Loosely(t, c.UnpackComment(ctx, &out), should.NotBeNil)
		})
	})
}

func TestExtractFunctionPlatform(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		functionName := "Lint"
		platform := "UBUNTU"
		f, p, err := ExtractFunctionPlatform(functionName + workerSeparator + platform)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, f, should.Equal(functionName))
		assert.Loosely(t, p, should.Equal(platform))
		_, _, err = ExtractFunctionPlatform(functionName)
		assert.Loosely(t, err, should.NotBeNil)
	})
}
