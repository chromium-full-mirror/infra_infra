// Copyright 2021 The Chromium Authors.
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

package cli

import (
	"context"
	"errors"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func generateMockGetEmail(email string, err error) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		return email, err
	}
}

func TestFilter(t *testing.T) {
	t.Parallel()
	ftt.Run("filter should return lj.filter if it's not empty.", t, func(t *ftt.Test) {
		lj := listJobs{}
		lj.filter = "user=email@example.com"
		testEmail := "not_this_email@example.com"
		ctx := context.Background()
		actual, err := filter(ctx, &lj, generateMockGetEmail(testEmail, nil))
		expected := "user=email@example.com"
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, actual, should.Equal(expected))
	})
	ftt.Run("filter should return 'user=email@example.com' if it's empty.", t, func(t *ftt.Test) {
		lj := listJobs{}
		lj.filter = ""
		testEmail := "email@example.com"
		ctx := context.Background()
		actual, err := filter(ctx, &lj, generateMockGetEmail(testEmail, nil))
		expected := "user=email@example.com"
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, actual, should.Equal(expected))
	})
	ftt.Run("filter should return the empty string if getEmail returns an error.", t, func(t *ftt.Test) {
		lj := listJobs{}
		lj.filter = ""
		testEmail := "not_this_email@example.com"
		ctx := context.Background()
		actual, err := filter(ctx, &lj, generateMockGetEmail(testEmail, errors.New("Mock")))
		expected := ""
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, actual, should.Equal(expected))
	})
}
