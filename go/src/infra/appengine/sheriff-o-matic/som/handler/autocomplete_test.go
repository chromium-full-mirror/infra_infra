package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/server/router"
)

func TestGetUserAutocompleteHandler(t *testing.T) {
	ftt.Run("basic", t, func(t *ftt.Test) {
		c := gaetesting.TestingContext()
		r := makeGetRequest(c)
		w := httptest.NewRecorder()

		ctx := &router.Context{
			Writer:  w,
			Request: r,
			Params:  makeParams("query", "def"),
		}

		GetUserAutocompleteHandler(ctx)
		assert.Loosely(t, w.Code, should.Equal(http.StatusOK))
	})
}

func TestAutocompleter(t *testing.T) {
	ftt.Run("basic", t, func(t *ftt.Test) {
		testStrs := []string{"abc", "bcd", "abcd", "def", "fghij"}

		ac := newAutocompleter(testStrs)
		t.Run("single char", func(t *ftt.Test) {
			res := ac.query("a")

			assert.Loosely(t, res, should.NotBeEmpty)
			assert.Loosely(t, res, should.Resemble([]string{"abc", "abcd"}))
		})

		t.Run("multi-char prefix", func(t *ftt.Test) {
			res := ac.query("ab")

			assert.Loosely(t, res, should.NotBeEmpty)
			assert.Loosely(t, res, should.Resemble([]string{"abc", "abcd"}))
		})

		t.Run("multi-char suffix", func(t *ftt.Test) {
			res := ac.query("cd")

			assert.Loosely(t, res, should.NotBeEmpty)
			assert.Loosely(t, res, should.Resemble([]string{"bcd", "abcd"}))
		})

		t.Run("mid-string match", func(t *ftt.Test) {
			res := ac.query("bc")

			assert.Loosely(t, res, should.NotBeEmpty)
			assert.Loosely(t, res, should.Resemble([]string{"abc", "bcd", "abcd"}))

			res = ac.query("ghi")

			assert.Loosely(t, res, should.NotBeEmpty)
			assert.Loosely(t, res, should.Resemble([]string{"fghij"}))
		})

		t.Run("no match", func(t *ftt.Test) {
			res := ac.query("x")
			assert.Loosely(t, res, should.BeEmpty)
		})
	})
}
