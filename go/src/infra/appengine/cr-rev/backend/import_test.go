package main

import (
	"testing"

	"github.com/golang/mock/gomock"

	"go.chromium.org/luci/appengine/gaetesting"
	gitilesProto "go.chromium.org/luci/common/proto/gitiles"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/appengine/cr-rev/backend/gitiles"
	"infra/appengine/cr-rev/backend/repoimport"
	"infra/appengine/cr-rev/common"
	"infra/appengine/cr-rev/config"
)

func TestInitialImport(t *testing.T) {
	ctx := gaetesting.TestingContext()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ftt.Run("invalid host", t, func(t *ftt.Test) {
		cfg := &config.Config{
			Hosts: []*config.Host{
				{
					Name: "invalid/name",
				},
			},
		}
		assert.Loosely(t, func() {
			setupImport(ctx, cfg)
		}, should.Panic)
	})
}

func TestInitialHostImport(t *testing.T) {
	ctx := gaetesting.TestingContext()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	controller := repoimport.NewMockController(mockCtrl)

	ftt.Run("Skip repos", t, func(t *ftt.Test) {
		// Setup gitiles
		fakeGitilesClient := &gitilesProto.Fake{}
		fakeGitilesClient.SetRepository("foo", nil, nil)
		fakeGitilesClient.SetRepository("bar", nil, nil)
		ctx := gitiles.SetClient(ctx, fakeGitilesClient)

		// Setup config
		barRepoConfig := &config.Repository{
			Name: "bar",
			Indexing: &config.Repository_DoNotIndex{
				DoNotIndex: true,
			},
		}
		host := &config.Host{
			Name: "host",
			Repos: []*config.Repository{
				barRepoConfig,
			},
		}

		// setup mock
		controller.EXPECT().Index(common.GitRepository{
			Host: "host",
			Name: "foo",
		}).Times(1)

		initialHostImport(ctx, controller, host)
		// We expect only one Gitiles calls (to list projects):
		assert.Loosely(t, len(fakeGitilesClient.GetCallLogs()), should.Equal(1))
	})
}
