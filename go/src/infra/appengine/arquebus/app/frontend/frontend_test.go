package frontend

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/julienschmidt/httprouter"

	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"
	"go.chromium.org/luci/server/router"
	"go.chromium.org/luci/server/secrets"
	"go.chromium.org/luci/server/secrets/testsecrets"
	"go.chromium.org/luci/server/templates"

	"infra/appengine/arquebus/app/backend"
	"infra/appengine/arquebus/app/backend/model"
	"infra/appengine/arquebus/app/config"
	"infra/appengine/arquebus/app/util"
)

func makeParams(items ...string) httprouter.Params {
	if len(items)%2 != 0 {
		return nil
	}
	params := make([]httprouter.Param, len(items)/2)
	for i := range params {
		params[i] = httprouter.Param{
			Key:   items[2*i],
			Value: items[2*i+1],
		}
	}
	return params
}

func testContext() context.Context {
	c := util.CreateTestContext()
	c = secrets.Use(c, &testsecrets.Store{})
	c = templates.Use(
		c, prepareTemplates("../appengine/templates"), &templates.Extra{
			Request: &http.Request{
				URL: &url.URL{Path: "/assigner/test-assigner"},
			},
		},
	)
	c = auth.WithState(c, &authtest.FakeState{Identity: "valid_user@test.com"})
	datastore.GetTestable(c).AutoIndex(true)
	return c
}

// createAssigner creates a sample Assigner entity.
func createAssigner(c context.Context, t testing.TB, id string) *model.Assigner {
	t.Helper()

	var cfg config.Assigner
	assert.Loosely(t, proto.UnmarshalText(util.SampleValidAssignerCfg, &cfg), should.BeNil, truth.LineContext())
	cfg.Id = id

	assert.Loosely(t, backend.UpdateAssigners(c, []*config.Assigner{&cfg}, "revision-1"), should.BeNil, truth.LineContext())
	datastore.GetTestable(c).CatchupIndexes()
	assigner, err := backend.GetAssigner(c, cfg.Id)
	assert.Loosely(t, assigner.ID, should.Equal(cfg.Id), truth.LineContext())
	assert.Loosely(t, err, should.BeNil, truth.LineContext())
	assert.Loosely(t, assigner, should.NotBeNil, truth.LineContext())

	return assigner
}

func createScheduledTask(c context.Context, t testing.TB, assigner *model.Assigner) *model.Task {
	t.Helper()

	task := &model.Task{
		AssignerKey:   model.GenAssignerKey(c, assigner),
		Status:        model.TaskStatus_Scheduled,
		ExpectedStart: testclock.TestTimeUTC,
	}
	assert.Loosely(t, datastore.Put(c, task), should.BeNil, truth.LineContext())
	return task
}

func TestFrontend(t *testing.T) {
	t.Parallel()
	assignerID := "test-assigner"

	ftt.Run("frontend", t, func(t *ftt.Test) {
		w := httptest.NewRecorder()
		c := &router.Context{
			Writer:  w,
			Request: (&http.Request{}).WithContext(testContext()),
		}

		t.Run("index", func(t *ftt.Test) {
			indexPage(c)
			assert.Loosely(t, w.Code, should.Equal(200))
		})

		t.Run("assigner", func(t *ftt.Test) {
			createAssigner(c.Request.Context(), t, assignerID)

			t.Run("found", func(t *ftt.Test) {
				c.Params = makeParams("AssignerID", assignerID)
				assignerPage(c)
				assert.Loosely(t, w.Code, should.Equal(200))
			})

			t.Run("not found, if assignerID not given", func(t *ftt.Test) {
				assignerPage(c)
				assert.Loosely(t, w.Code, should.Equal(404))
			})

			t.Run("not found, if non-existing assignerID given", func(t *ftt.Test) {
				c.Params = makeParams("AssignerID", "foo")
				assignerPage(c)
				assert.Loosely(t, w.Code, should.Equal(404))
			})
		})

		t.Run("task", func(t *ftt.Test) {
			assigner := createAssigner(c.Request.Context(), t, assignerID)
			task := createScheduledTask(c.Request.Context(), t, assigner)

			t.Run("found", func(t *ftt.Test) {
				c.Params = makeParams(
					"AssignerID", assignerID,
					"TaskID", strconv.FormatInt(task.ID, 10),
				)
				taskPage(c)
				assert.Loosely(t, w.Code, should.Equal(200))
			})

			t.Run("not found, if no params given", func(t *ftt.Test) {
				taskPage(c)
				assert.Loosely(t, w.Code, should.Equal(404))
			})

			t.Run("not found, if taskID not given", func(t *ftt.Test) {
				c.Params = makeParams(
					"AssignerID", assignerID,
				)
				taskPage(c)
				assert.Loosely(t, w.Code, should.Equal(404))
			})

			t.Run("not found, if assignerID not given", func(t *ftt.Test) {
				c.Params = makeParams(
					"TaskID", strconv.FormatInt(task.ID, 10),
				)
				taskPage(c)
				assert.Loosely(t, w.Code, should.Equal(404))
			})

			t.Run("not found, if non-existing taskID given", func(t *ftt.Test) {
				c.Params = makeParams(
					"AssignerID", assignerID,
					"TaskID", strconv.FormatInt(task.ID+1, 10),
				)
				taskPage(c)
				assert.Loosely(t, w.Code, should.Equal(404))
			})
		})
	})
}
