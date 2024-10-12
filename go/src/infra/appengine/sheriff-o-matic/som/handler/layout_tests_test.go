package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/impl/dummy"
	"go.chromium.org/luci/gae/service/info"
	"go.chromium.org/luci/gae/service/urlfetch"
	"go.chromium.org/luci/server/router"

	testhelper "infra/appengine/sheriff-o-matic/som/client/test"
	te "infra/appengine/sheriff-o-matic/som/testexpectations"
)

const (
	gitilesPrefix = "https://chromium.googlesource.com/chromium/src/+/HEAD/"
)

// TODO(seanmccullough): Clean up this mocking mess.
type mck struct {
	giMock
}

func (m mck) ModuleHostname(a, b, c string) (string, error) {
	return "", nil
}

func TestGetLayoutTestsHandler(t *testing.T) {
	ftt.Run("get layout tests", t, func(t *ftt.Test) {
		c := gaetesting.TestingContext()
		c = info.SetFactory(c, func(ic context.Context) info.RawInterface {
			return giMock{dummy.Info(), "", time.Now(), nil}
		})
		gt := &testhelper.MockGitilesTransport{Responses: map[string]string{}}
		c = urlfetch.Set(c, gt)
		w := httptest.NewRecorder()

		ctx := &router.Context{
			Writer:  w,
			Request: makeGetRequest(c),
		}

		t.Run("load all, error", func(t *ftt.Test) {
			GetLayoutTestsHandler(ctx)

			assert.Loosely(t, w.Code, should.Equal(http.StatusInternalServerError))
		})

		t.Run("load all, no error", func(t *ftt.Test) {
			for _, path := range te.LayoutTestExpectations {
				gt.Responses[gitilesPrefix+path+"?format=TEXT"] = `unused`
			}
			GetLayoutTestsHandler(ctx)

			assert.Loosely(t, w.Code, should.Equal(http.StatusOK))
		})
	})
}

func TestPostLayoutTestExpectationChangeHandler(t *testing.T) {
	ftt.Run("basic", t, func(t *ftt.Test) {
		c := gaetesting.TestingContext()
		gt := &testhelper.MockGitilesTransport{Responses: map[string]string{}}
		for _, path := range te.LayoutTestExpectations {
			gt.Responses["https://chromium.googlesource.com/chromium/src/+/HEAD/"+path+"?format=TEXT"] = ""
		}

		c = urlfetch.Set(c, gt)

		c = info.SetFactory(c, func(ic context.Context) info.RawInterface {
			return mck{giMock{dummy.Info(), "", time.Now(), nil}}
		})

		t.Run("empty body, error", func(t *ftt.Test) {
			w := httptest.NewRecorder()
			ctx := &router.Context{
				Writer:  w,
				Request: makePostRequest(c, ""),
			}

			PostLayoutTestExpectationChangeHandler(ctx)
			assert.Loosely(t, w.Code, should.Equal(http.StatusInternalServerError))
		})

		t.Run("valid body", func(t *ftt.Test) {
			w := httptest.NewRecorder()
			body, err := json.Marshal(&shortExp{
				TestName: "test_test/test.html",
			})
			assert.Loosely(t, err, should.BeNil)
			ctx := &router.Context{
				Writer:  w,
				Request: makePostRequest(c, string(body)),
			}

			PostLayoutTestExpectationChangeHandler(ctx)
			assert.Loosely(t, w.Code, should.NotEqual(http.StatusInternalServerError))
		})
	})
}
