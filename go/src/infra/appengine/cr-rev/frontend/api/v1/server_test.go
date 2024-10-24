package api

import (
	"testing"

	"google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	"infra/appengine/cr-rev/frontend/redirect"
	"infra/appengine/cr-rev/models"
)

func TestServer(t *testing.T) {
	ctx := gaetesting.TestingContext()
	ds := datastore.GetTestable(ctx)
	ds.Consistent(true)
	ds.AutoIndex(true)
	s := NewServer(redirect.NewRules(redirect.NewGitilesRedirect()))
	commits := []*models.Commit{
		{
			ID:             "chromium-chromium/src-0000000000000000000000000000000000000001",
			CommitHash:     "0000000000000000000000000000000000000001",
			Host:           "chromium",
			Repository:     "chromium/src",
			PositionNumber: 1,
			PositionRef:    "svn://svn.chromium.org/chrome",
		},
		{
			ID:             "chromium-chromium/src-0000000000000000000000000000000000000002",
			CommitHash:     "0000000000000000000000000000000000000002",
			Host:           "chromium",
			Repository:     "chromium/src",
			PositionNumber: 2,
			PositionRef:    "refs/heads/main",
		},
		{
			ID:             "chromium-foo-0000000000000000000000000000000000000003",
			CommitHash:     "0000000000000000000000000000000000000003",
			Host:           "chromium",
			Repository:     "foo",
			PositionNumber: 3,
			PositionRef:    "refs/heads/main",
		},
	}
	datastore.Put(ctx, commits)

	ftt.Run("redirect", t, func(t *ftt.Test) {
		t.Run("empty request", func(t *ftt.Test) {
			_, err := s.Redirect(ctx, &RedirectRequest{})
			assert.Loosely(t, err, should.ErrLike("empty result"))
			s, _ := status.FromError(err)
			assert.Loosely(t, s.Code(), should.Equal(codes.NotFound))
		})
		t.Run("matching path found", func(t *ftt.Test) {
			resp, err := s.Redirect(ctx, &RedirectRequest{
				Query: "/1",
			})
			assert.Loosely(t, err, should.BeNil)
			expected := &RedirectResponse{
				GitHash:     "0000000000000000000000000000000000000001",
				Host:        "chromium",
				Repository:  "chromium/src",
				RedirectUrl: "https://chromium.googlesource.com/chromium/src/+/0000000000000000000000000000000000000001",
			}
			assert.Loosely(t, resp, should.Resemble(expected))
		})
		t.Run("not chromium/src", func(t *ftt.Test) {
			_, err := s.Redirect(ctx, &RedirectRequest{
				Query: "/3",
			})
			assert.Loosely(t, err, should.ErrLike("empty result"))
			s, _ := status.FromError(err)
			assert.Loosely(t, s.Code(), should.Equal(codes.NotFound))
		})
	})

	ftt.Run("Numbering", t, func(t *ftt.Test) {
		t.Run("empty request", func(t *ftt.Test) {
			_, err := s.Numbering(ctx, &NumberingRequest{})
			assert.Loosely(t, err, should.ErrLike("empty result"))
			s, _ := status.FromError(err)
			assert.Loosely(t, s.Code(), should.Equal(codes.NotFound))
		})
		t.Run("not found", func(t *ftt.Test) {
			_, err := s.Numbering(ctx, &NumberingRequest{
				PositionNumber: 3,
				Host:           "chromium",
				Repository:     "chromium/src",
				PositionRef:    "svn://svn.chromium.org/chrome",
			})
			assert.Loosely(t, err, should.ErrLike("empty result"))
			s, _ := status.FromError(err)
			assert.Loosely(t, s.Code(), should.Equal(codes.NotFound))
		})
		t.Run("chromium/src", func(t *ftt.Test) {
			resp, err := s.Numbering(ctx, &NumberingRequest{
				PositionNumber: 1,
				Host:           "chromium",
				Repository:     "chromium/src",
				PositionRef:    "svn://svn.chromium.org/chrome",
			})
			assert.Loosely(t, err, should.BeNil)
			expected := &NumberingResponse{
				GitHash:        "0000000000000000000000000000000000000001",
				PositionNumber: 1,
				Host:           "chromium",
				Repository:     "chromium/src",
			}
			assert.Loosely(t, resp, should.Resemble(expected))
		})
		t.Run("arbitrary repository", func(t *ftt.Test) {
			resp, err := s.Numbering(ctx, &NumberingRequest{
				PositionNumber: 3,
				Host:           "chromium",
				Repository:     "foo",
				PositionRef:    "refs/heads/main",
			})
			assert.Loosely(t, err, should.BeNil)
			expected := &NumberingResponse{
				GitHash:        "0000000000000000000000000000000000000003",
				PositionNumber: 3,
				Host:           "chromium",
				Repository:     "foo",
			}
			assert.Loosely(t, resp, should.Resemble(expected))
		})
	})

	ftt.Run("Commit", t, func(t *ftt.Test) {
		t.Run("empty request", func(t *ftt.Test) {
			_, err := s.Commit(ctx, &CommitRequest{})
			assert.Loosely(t, err, should.ErrLike("empty result"))
			s, _ := status.FromError(err)
			assert.Loosely(t, s.Code(), should.Equal(codes.NotFound))
		})
		t.Run("not found", func(t *ftt.Test) {
			_, err := s.Commit(ctx, &CommitRequest{
				GitHash: "0000000000000000000000000000000000000000",
			})
			assert.Loosely(t, err, should.ErrLike("empty result"))
			s, _ := status.FromError(err)
			assert.Loosely(t, s.Code(), should.Equal(codes.NotFound))
		})
		t.Run("chromium/src", func(t *ftt.Test) {
			resp, err := s.Commit(ctx, &CommitRequest{
				GitHash: "0000000000000000000000000000000000000001",
			})
			assert.Loosely(t, err, should.BeNil)
			expected := &CommitResponse{
				GitHash:        "0000000000000000000000000000000000000001",
				PositionNumber: 1,
				Host:           "chromium",
				Repository:     "chromium/src",
				RedirectUrl:    "https://chromium.googlesource.com/chromium/src/+/0000000000000000000000000000000000000001",
			}
			assert.Loosely(t, resp, should.Resemble(expected))
		})
	})
}
