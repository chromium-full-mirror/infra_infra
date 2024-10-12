package main

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestGetGerritUrl(t *testing.T) {
	ftt.Run("returns CL", t, func(t *ftt.Test) {
		url := getGerritUrl("https://www.example.com/", "42")
		assert.Loosely(t, url, should.Equal("https://www.example.com/c/42"))
	})

	ftt.Run("returns dashboard", t, func(t *ftt.Test) {
		url := getGerritUrl("https://www.example.com/", "")
		assert.Loosely(t, url, should.Equal("https://www.example.com/dashboard/self"))
	})
}
