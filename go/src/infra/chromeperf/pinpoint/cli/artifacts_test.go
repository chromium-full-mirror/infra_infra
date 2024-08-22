// Copyright 2021 The Chromium Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package cli

import (
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"testing"
)

func TestCasHandling(t *testing.T) {
	ftt.Run("Parse components from CAS URL", t, func(t *ftt.Test) {
		instance, hash, bytes, err := extractCasParamsFromURL("https://cas-viewer.appspot.com/projects/chrome-swarming/instances/default_instance/blobs/327d759be13ebe68392ab8deec4fba29243b96eea2cdc10a2a3b7eac44088123/176/tree")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, string(instance), should.Equal("projects/chrome-swarming/instances/default_instance"))
		assert.Loosely(t, string(hash), should.Equal("327d759be13ebe68392ab8deec4fba29243b96eea2cdc10a2a3b7eac44088123"))
		assert.Loosely(t, int64(bytes), should.Equal(176))
	})
}
