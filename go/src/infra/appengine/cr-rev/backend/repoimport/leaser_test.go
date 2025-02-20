package repoimport

import (
	"context"
	"testing"
	"time"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	"go.chromium.org/infra/appengine/cr-rev/common"
	"go.chromium.org/infra/appengine/cr-rev/models"
)

func TestLeaser(t *testing.T) {
	repo := common.GitRepository{
		Host: "foo",
		Name: "bar",
	}

	doc := &models.Repository{
		ID: models.RepoID{
			Host:       "foo",
			Repository: "bar",
		},
	}

	prepareEnvironment := func() (context.Context, *leaser) {
		ctx := gaetesting.TestingContext()
		ds := datastore.GetTestable(ctx)
		ds.Consistent(true)
		ds.AutoIndex(true)

		testclock := testclock.New(time.Now())
		ctx = clock.Set(ctx, testclock)

		return ctx, newLeaser(repo)
	}
	ftt.Run("Lease", t, func(t *ftt.Test) {
		t.Run("non existing lock", func(t *ftt.Test) {
			ctx, leaser := prepareEnvironment()
			err := leaser.refreshLease(ctx)
			assert.Loosely(t, err, should.ErrLike(datastore.ErrNoSuchEntity))
		})

		t.Run("valid renew", func(t *ftt.Test) {
			ctx, leaser := prepareEnvironment()
			err := leaser.acquireLease(ctx)
			assert.Loosely(t, err, should.BeNil)
			err = leaser.refreshLease(ctx)
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("lock claimed by another process", func(t *ftt.Test) {
			ctx, leaser := prepareEnvironment()
			err := leaser.acquireLease(ctx)
			assert.Loosely(t, err, should.BeNil)

			// Override lock
			datastore.Put(ctx, doc)
			err = leaser.refreshLease(ctx)
			assert.Loosely(t, err, should.ErrLike("some other process claimed the lock"))
		})
	})
}
