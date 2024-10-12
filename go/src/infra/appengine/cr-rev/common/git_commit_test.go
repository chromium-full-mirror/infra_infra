package common

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestGitCommit(t *testing.T) {
	ftt.Run("ID generation", t, func(t *ftt.Test) {
		commit := &GitCommit{
			Repository: GitRepository{
				Host: "foo",
				Name: "bar",
			},
			Hash: "0000000000000000000000000000000000000000",
		}
		assert.Loosely(t, commit.ID(), should.Equal("foo-bar-0000000000000000000000000000000000000000"))
	})

	ftt.Run("extract Footers", t, func(t *ftt.Test) {
		t.Run("nothing to extract", func(t *ftt.Test) {
			commit := &GitCommit{
				CommitMessage: "foo\n\nbar",
			}
			assert.Loosely(t, commit.GetFooters("Foo"), should.BeEmpty)
			assert.Loosely(t, commit.GetFooters("Bar"), should.BeEmpty)
			assert.Loosely(t, commit.GetFooters("Baz"), should.BeEmpty)
		})

		t.Run("empty value", func(t *ftt.Test) {
			commit := &GitCommit{
				CommitMessage: "foo\n\nbar:",
			}
			assert.Loosely(t, commit.GetFooters("Foo"), should.BeEmpty)
			assert.Loosely(t, commit.GetFooters("Bar"), should.Resemble([]string{""}))
		})

		t.Run("multiple values", func(t *ftt.Test) {
			commit := &GitCommit{
				CommitMessage: "foo\n\nBar: 42\nBar: 43",
			}
			assert.Loosely(t, commit.GetFooters("Bar"), should.Resemble([]string{"43", "42"}))
		})

		t.Run("quoted values not extracted", func(t *ftt.Test) {
			commit := &GitCommit{
				CommitMessage: "foo\n\n> Bar: 42\nBar: 43",
			}
			assert.Loosely(t, commit.GetFooters("Bar"), should.Resemble([]string{"43"}))
		})
	})

	ftt.Run("position footer", t, func(t *ftt.Test) {
		t.Run("no number", func(t *ftt.Test) {
			commit := &GitCommit{
				CommitMessage: "foo",
			}
			_, err := commit.GetPositionNumber()
			assert.Loosely(t, err, should.Equal(ErrNoPositionFooter))
		})

		t.Run("gitnumberer syntax", func(t *ftt.Test) {
			commit := &GitCommit{
				CommitMessage: "foo\n\nCr-Commit-Position: refs/heads/main@{#42}",
			}
			position, err := commit.GetPositionNumber()
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, position.Number, should.Equal(42))
			assert.Loosely(t, position.Name, should.Equal("refs/heads/main"))
		})

		t.Run("svn-id syntax", func(t *ftt.Test) {
			commit := &GitCommit{
				CommitMessage: "foo\n\ngit-svn-id: svn://svn.chromium.org/chrome/trunk/src@42 00000000-0000-0000-0000-000000000000",
			}
			position, err := commit.GetPositionNumber()
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, position.Number, should.Equal(42))
			assert.Loosely(t, position.Name, should.Equal("svn://svn.chromium.org/chrome/trunk/src"))
		})

		t.Run("with quoted text position", func(t *ftt.Test) {
			commit := &GitCommit{
				CommitMessage: "foo\n\n>Cr-Commit-Position: refs/heads/main@{#42}",
			}
			_, err := commit.GetPositionNumber()
			assert.Loosely(t, err, should.Equal(ErrNoPositionFooter))
		})

		t.Run("gitnumberer syntax with quoted text", func(t *ftt.Test) {
			commit := &GitCommit{
				CommitMessage: "foo\n\n>Cr-Commit-Position: refs/heads/foo@{#42}\nCr-Commit-Position: refs/heads/main@{#43}",
			}
			position, err := commit.GetPositionNumber()
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, position.Number, should.Equal(43))
			assert.Loosely(t, position.Name, should.Equal("refs/heads/main"))
		})

		t.Run("gitnumberer syntax with quoted text2", func(t *ftt.Test) {
			commit := &GitCommit{
				CommitMessage: "foo\n\nCr-Commit-Position: refs/heads/foo@{#42}\n>Cr-Commit-Position: refs/heads/main@{#43}",
			}
			position, err := commit.GetPositionNumber()
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, position.Number, should.Equal(42))
			assert.Loosely(t, position.Name, should.Equal("refs/heads/foo"))
		})

		t.Run("multiple gitnumberer", func(t *ftt.Test) {
			commit := &GitCommit{
				CommitMessage: "foo\n\nCr-Commit-Position: refs/heads/foo@{#42}\nCr-Commit-Position: refs/heads/main@{#43}",
			}
			position, err := commit.GetPositionNumber()
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, position.Number, should.Equal(42))
		})

		t.Run("invalid format", func(t *ftt.Test) {
			commit := &GitCommit{
				CommitMessage: "foo\n\nCr-Commit-Position: foo",
			}
			_, err := commit.GetPositionNumber()
			assert.Loosely(t, err, should.Equal(ErrInvalidPositionFooter))
		})
	})
}
