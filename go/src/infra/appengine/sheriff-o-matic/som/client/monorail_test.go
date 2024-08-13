package client

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestMonorail(t *testing.T) {
	ftt.Run("Test ParseMonorailIssueName", t, func(t *ftt.Test) {
		t.Run("valid issue name", func(t *ftt.Test) {
			issueName := "projects/chromium/issues/123"
			projectID, bugID, err := ParseMonorailIssueName(issueName)
			assert.Loosely(t, projectID, should.Equal("chromium"))
			assert.Loosely(t, bugID, should.Equal("123"))
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("invalid issue name", func(t *ftt.Test) {
			issueName := "invalid"
			_, _, err := ParseMonorailIssueName(issueName)
			assert.Loosely(t, err, should.NotBeNil)
		})
	})
}
