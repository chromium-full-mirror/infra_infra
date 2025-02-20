package handler

import (
	"testing"
	"time"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	"go.chromium.org/infra/appengine/sheriff-o-matic/som/model"
)

func TestFlushAlerts(t *testing.T) {
	ftt.Run("test flush alerts", t, func(t *ftt.Test) {
		c := gaetesting.TestingContext()
		cl := testclock.New(testclock.TestRecentTimeUTC)
		c = clock.Set(c, cl)
		now := clock.Get(c).Now()

		alertIdx := datastore.IndexDefinition{
			Kind: "AlertJSONNonGrouping",
			SortBy: []datastore.IndexColumn{
				{
					Property:   "Resolved",
					Descending: false,
				},
				{
					Property:   "ResolvedDate",
					Descending: false,
				},
			},
		}

		indexes := []*datastore.IndexDefinition{&alertIdx}
		datastore.GetTestable(c).AddIndexes(indexes...)

		alerts := []*model.AlertJSONNonGrouping{
			{
				ID:           "alert1",
				Resolved:     false,
				ResolvedDate: now.Add(-time.Hour * 24 * 8),
			},
			{
				ID:           "alert2",
				Resolved:     true,
				ResolvedDate: now.Add(-time.Hour * 24 * 8),
			},
			{
				ID:           "alert3",
				Resolved:     true,
				ResolvedDate: now.Add(-time.Hour * 24 * 6),
			},
		}

		assert.Loosely(t, datastore.Put(c, alerts), should.BeNil)
		datastore.GetTestable(c).CatchupIndexes()
		num, err := flushOldAlerts(c)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, num, should.Equal(1))

		q := datastore.NewQuery("AlertJSONNonGrouping")
		result := []*model.AlertJSONNonGrouping{}
		err = datastore.GetAll(c, q, &result)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(result), should.Equal(2))
		assert.Loosely(t, result[0].ID, should.Equal("alert1"))
		assert.Loosely(t, result[1].ID, should.Equal("alert3"))
	})
}
