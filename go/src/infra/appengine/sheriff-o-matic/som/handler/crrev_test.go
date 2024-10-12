package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/server/auth/authtest"
	"go.chromium.org/luci/server/router"

	"infra/appengine/sheriff-o-matic/som/client"
	"infra/appengine/sheriff-o-matic/som/client/test"
)

func TestRevRangeHandler(t *testing.T) {
	fakeCrRev := test.NewFakeServer()
	defer fakeCrRev.Server.Close()

	c := gaetesting.TestingContext()
	c = gologger.StdConfig.Use(c)
	crRev := client.NewCrRev(fakeCrRev.Server.URL)

	ftt.Run("get rev range", t, func(t *ftt.Test) {
		t.Run("ok with positions", func(t *ftt.Test) {
			c = authtest.MockAuthConfig(c)
			w := httptest.NewRecorder()
			getRevRangeHandler(&router.Context{
				Writer: w,
				Request: makeGetRequest(c,
					"startPos", "123", "endPos", "456",
					"endRev", "1a2b3c4d"),
				Params: makeParams(
					"host", "chromium", "repo", "chromium.src"),
			}, crRev)

			assert.Loosely(t, w.Code, should.Equal(301))
		})
		t.Run("ok with revisions", func(t *ftt.Test) {
			c = authtest.MockAuthConfig(c)
			w := httptest.NewRecorder()
			getRevRangeHandler(&router.Context{
				Writer: w,
				Request: makeGetRequest(c,
					"startRev", "2a2b3c4d", "endRev", "1a2b3c4d"),
				Params: makeParams(
					"host", "chromium", "repo", "chromium.src"),
			}, crRev)

			assert.Loosely(t, w.Code, should.Equal(301))
		})
		t.Run("bad oauth", func(t *ftt.Test) {
			w := httptest.NewRecorder()
			getRevRangeHandler(&router.Context{
				Writer: w,
				Request: makeGetRequest(c,
					"startPos", "123", "endPos", "456",
					"endRev", "1a2b3c4d"),
				Params: makeParams(
					"host", "chromium", "repo", "chromium.src"),
			}, crRev)
			assert.Loosely(t, w.Code, should.Equal(http.StatusMovedPermanently))
		})
		t.Run("bad request", func(t *ftt.Test) {
			w := httptest.NewRecorder()

			getRevRangeHandler(&router.Context{
				Writer:  w,
				Request: makeGetRequest(c),
			}, crRev)

			assert.Loosely(t, w.Code, should.Equal(400))
		})
		t.Run("bad start and end params", func(t *ftt.Test) {
			w := httptest.NewRecorder()

			getRevRangeHandler(&router.Context{
				Writer: w,
				Request: makeGetRequest(c,
					"startPos", "123", "endRev", "1a2b3c4d"),
				Params: makeParams(
					"host", "chromium", "repo", "chromium.src"),
			}, crRev)
			assert.Loosely(t, w.Code, should.Equal(400))
		})
		t.Run("bad repo and host", func(t *ftt.Test) {
			w := httptest.NewRecorder()

			getRevRangeHandler(&router.Context{
				Writer: w,
				Request: makeGetRequest(c,
					"startPos", "123", "endPos", "234",
					"startRev", "2a2b3c4d", "endRev", "1a2b3c4d"),
				Params: makeParams(),
			}, crRev)
			assert.Loosely(t, w.Code, should.Equal(400))
		})

	})
}
