// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cipd

import (
	"testing"

	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestUnmarshalPackages(t *testing.T) {
	t.Parallel()
	jsonData := []byte(`{
  "result": {
    "": [
      {
        "package": "chromiumos/infra/some_application/linux-amd64",
        "pin": {
          "package": "chromiumos/infra/some_application/linux-amd64",
          "instance_id": "Z5AzvrgQMH45eCuQymTro7yVwwJOny0Tf5vFRks4A-4C"
        },
        "tracking": "latest"
      }
    ]
  }
}`)
	got, err := unmarshalPackages(jsonData)
	if err != nil {
		t.Fatalf("unmarshalPackages returned error: %s", err)
	}
	want := []Package{
		{
			Package: "chromiumos/infra/some_application/linux-amd64",
			Pin: Pin{
				Package:    "chromiumos/infra/some_application/linux-amd64",
				InstanceID: "Z5AzvrgQMH45eCuQymTro7yVwwJOny0Tf5vFRks4A-4C",
			},
			Tracking: "latest",
		},
	}
	assert.That(t, got, should.Match(want))
}
