package testexpectations

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/impl/dummy"
	"go.chromium.org/luci/gae/service/info"
	"go.chromium.org/luci/gae/service/urlfetch"

	testhelper "infra/appengine/sheriff-o-matic/som/client/test"
)

func TestUpdateExpectations(t *testing.T) {
	ftt.Run("Update with empty expectation returns error", t, func(t *ftt.Test) {
		fs := &FileSet{
			Files: []*File{
				{
					Path:         "/some/path",
					Expectations: []*ExpectationStatement{},
				},
			},
		}

		err := fs.UpdateExpectation(&ExpectationStatement{})
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Update basic, single file, existing test", t, func(t *ftt.Test) {
		fs := &FileSet{
			Files: []*File{
				{
					Path: "/some/path",
					Expectations: []*ExpectationStatement{
						{
							TestName:     "/third_party/test_name",
							Expectations: []string{"PASS"},
							Dirty:        true,
						},
					},
				},
			},
		}

		cl := fs.ToCL()
		assert.Loosely(t, cl, should.NotBeNil)
		assert.Loosely(t, len(cl), should.Equal(1))

		assert.Loosely(t, cl["/some/path"], should.Equal("/third_party/test_name [ PASS ]"))

		err := fs.UpdateExpectation(&ExpectationStatement{
			TestName:     "/third_party/test_name",
			Expectations: []string{"PASS", "FAIL"},
		})
		assert.Loosely(t, err, should.BeNil)

		cl = fs.ToCL()
		assert.Loosely(t, cl, should.NotBeNil)
		assert.Loosely(t, len(cl), should.Equal(1))

		assert.Loosely(t, cl["/some/path"], should.Equal("/third_party/test_name [ PASS FAIL ]"))
	})

	ftt.Run("Update basic, single file, new test", t, func(t *ftt.Test) {
		fs := &FileSet{
			Files: []*File{
				{
					Path: defaultExpectationsFile,
					Expectations: []*ExpectationStatement{
						{
							TestName:     "/third_party/test_name",
							Expectations: []string{"PASS"},
							Original:     "/third_party/test_name [ PASS ]",
						},
					},
				},
			},
		}

		cl := fs.ToCL()
		assert.Loosely(t, cl, should.NotBeNil)
		assert.Loosely(t, len(cl), should.BeZero)

		err := fs.UpdateExpectation(&ExpectationStatement{
			TestName:     "/third_party/new_test_name",
			Expectations: []string{"PASS", "FAIL"},
			Dirty:        true,
		})
		assert.Loosely(t, err, should.BeNil)

		cl = fs.ToCL()
		assert.Loosely(t, cl, should.NotBeNil)
		assert.Loosely(t, len(cl), should.Equal(1))
		assert.Loosely(t, cl[defaultExpectationsFile], should.Equal("/third_party/test_name [ PASS ]\n/third_party/new_test_name [ PASS FAIL ]"))
	})

	ftt.Run("Update basic, multiple files, new test", t, func(t *ftt.Test) {
		fs := &FileSet{
			Files: []*File{
				{
					Path: "/some/path",
					Expectations: []*ExpectationStatement{
						{
							TestName:     "/third_party/test_name",
							Expectations: []string{"PASS"},
							Original:     "/third_party/test_name [ PASS ]",
						},
					},
				},
				{
					Path:         defaultExpectationsFile,
					Expectations: []*ExpectationStatement{},
				},
			},
		}

		cl := fs.ToCL()
		assert.Loosely(t, cl, should.NotBeNil)
		assert.Loosely(t, len(cl), should.BeZero)

		err := fs.UpdateExpectation(&ExpectationStatement{
			TestName:     "/third_party/new_test_name",
			Expectations: []string{"PASS", "FAIL"},
			Dirty:        true,
		})
		assert.Loosely(t, err, should.BeNil)

		cl = fs.ToCL()
		assert.Loosely(t, cl, should.NotBeNil)
		assert.Loosely(t, len(cl), should.Equal(1))

		assert.Loosely(t, cl[defaultExpectationsFile], should.Equal("/third_party/new_test_name [ PASS FAIL ]"))
	})

	ftt.Run("Update basic, multiple files and tests", t, func(t *ftt.Test) {
		fs := &FileSet{
			Files: []*File{
				{
					Path: "/some/path1",
					Expectations: []*ExpectationStatement{
						{
							Original:     "/third_party/test_name1 [ PASS ]",
							TestName:     "/third_party/test_name1",
							Expectations: []string{"PASS"},
						},
						{
							Original:     "/third_party/test_name1b [ FAIL ]",
							TestName:     "/third_party/test_name1b",
							Expectations: []string{"FAIL"},
						},
					},
				},
				{
					Path: "/some/path2",
					Expectations: []*ExpectationStatement{
						{
							Original:     "/third_party/test_name2 [ PASS ]",
							TestName:     "/third_party/test_name2",
							Expectations: []string{"PASS"},
						},
					},
				},
			},
		}

		cl := fs.ToCL()
		assert.Loosely(t, cl, should.NotBeNil)
		assert.Loosely(t, len(cl), should.BeZero)

		err := fs.UpdateExpectation(&ExpectationStatement{
			TestName:     "/third_party/test_name1",
			Expectations: []string{"PASS", "FAIL"},
		})
		assert.Loosely(t, err, should.BeNil)

		cl = fs.ToCL()
		assert.Loosely(t, cl, should.NotBeNil)
		assert.Loosely(t, len(cl), should.Equal(1))

		assert.Loosely(t, cl["/some/path1"], should.Equal(strings.Join([]string{
			"/third_party/test_name1 [ PASS FAIL ]",
			"/third_party/test_name1b [ FAIL ]",
		}, "\n")))

		assert.Loosely(t, cl["/some/path2"], should.BeEmpty)
	})

	ftt.Run("Update basic, comments, multiple files and tests", t, func(t *ftt.Test) {
		fs := &FileSet{
			Files: []*File{
				{
					Path: "/some/path1",
					Expectations: []*ExpectationStatement{
						{
							Original: "# a comment",
							Comment:  "# a comment",
						},
						{
							Original: "",
							Comment:  "",
						},
						{
							Original:     "/third_party/test_name1 [ PASS ]",
							TestName:     "/third_party/test_name1",
							Expectations: []string{"PASS"},
						},
						{
							Original:     "/third_party/test_name1b [ FAIL ]",
							TestName:     "/third_party/test_name1b",
							Expectations: []string{"FAIL"},
						},
					},
				},
				{
					Path: "/some/path2",
					Expectations: []*ExpectationStatement{
						{
							Original:     "/third_party/test_name2 [ PASS ]",
							TestName:     "/third_party/test_name2",
							Expectations: []string{"PASS"},
						},
					},
				},
			},
		}

		cl := fs.ToCL()
		assert.Loosely(t, cl, should.NotBeNil)
		assert.Loosely(t, len(cl), should.BeZero)

		err := fs.UpdateExpectation(&ExpectationStatement{
			TestName:     "/third_party/test_name1",
			Expectations: []string{"PASS", "FAIL"},
		})
		assert.Loosely(t, err, should.BeNil)

		cl = fs.ToCL()
		assert.Loosely(t, cl, should.NotBeNil)
		assert.Loosely(t, len(cl), should.Equal(1))

		assert.Loosely(t, cl["/some/path1"], should.Equal(strings.Join([]string{
			"# a comment",
			"",
			"/third_party/test_name1 [ PASS FAIL ]",
			"/third_party/test_name1b [ FAIL ]",
		}, "\n")))

		assert.Loosely(t, cl["/some/path2"], should.BeEmpty)
	})
}

func TestExpectationStatement(t *testing.T) {
	ftt.Run("modifiers", t, func(t *ftt.Test) {
		t.Run("expanded match", func(t *ftt.Test) {
			es := &ExpectationStatement{
				Original:     "[ Mac ] /third_party/test_dir/foo_bar/baz.html [ FAIL ]",
				TestName:     "/third_party/test_dir/foo_bar/baz.html",
				Expectations: []string{"FAIL"},
				Modifiers:    []string{"Mac"},
			}

			assert.Loosely(t, es.ExpandModifiers(), should.Match([]string{"Mac", "retina", "mac10.9", "mac10.11", "mac10.12"}))
			assert.Loosely(t, es.ModifierMatch("Mac10.9"), should.Equal(true))
		})

		t.Run("applies", func(t *ftt.Test) {
			t.Run("narrow specifiers", func(t *ftt.Test) {
				es := &ExpectationStatement{
					Original:     "[ Mac ] /third_party/test_dir/foo_bar/baz.html [ FAIL ]",
					TestName:     "/third_party/test_dir/foo_bar/baz.html",
					Expectations: []string{"FAIL"},
					Modifiers:    []string{"Mac"},
				}

				assert.Loosely(t, es.Applies("/third_party/test_dir/foo_bar/baz.html", "Mac"), should.Equal(true))
			})
		})
	})
}

func TestForTest(t *testing.T) {
	ftt.Run("basic", t, func(t *ftt.Test) {
		fs := &FileSet{
			Files: []*File{
				{
					Path: "/some/path1",
					Expectations: []*ExpectationStatement{
						{
							Original:     "/third_party/test_name1 [ PASS ]",
							TestName:     "/third_party/test_name1",
							Expectations: []string{"PASS"},
						},
						{
							Original:     "/third_party/test_name1b [ FAIL ]",
							TestName:     "/third_party/test_name1b",
							Expectations: []string{"FAIL"},
						},
					},
				},
				{
					Path: "/some/path2",
					Expectations: []*ExpectationStatement{
						{
							Original:     "/third_party/test_name2 [ PASS ]",
							TestName:     "/third_party/test_name2",
							Expectations: []string{"PASS"},
						},
					},
				},
				{
					Path: "/some/path3",
					Expectations: []*ExpectationStatement{
						{
							Original:     "/third_party/test_dir/foo_bar [ PASS ]",
							TestName:     "/third_party/test_dir/foo_bar",
							Expectations: []string{"PASS"},
						},
					},
				},
				{
					Path: "/some/path3",
					Expectations: []*ExpectationStatement{
						{
							Original:     "[ Mac ] /third_party/test_dir/foo_bar/baz.html [ FAIL ]",
							TestName:     "/third_party/test_dir/foo_bar/baz.html",
							Expectations: []string{"FAIL"},
							Modifiers:    []string{"Mac"},
						},
					},
				},
				{
					Path: "/some/path3",
					Expectations: []*ExpectationStatement{
						{
							Original:     "[ Mac10.12 ] /third_party/test_dir/zippy.html [ FAIL ]",
							TestName:     "/third_party/test_dir/zippy.html",
							Expectations: []string{"FAIL"},
							Modifiers:    []string{"Mac10.12"},
						},
					},
				},
			},
		}

		t.Run("no matches", func(t *ftt.Test) {
			matches := fs.ForTest("foo", &BuilderConfig{})
			assert.Loosely(t, len(matches), should.BeZero)
		})

		t.Run("one match", func(t *ftt.Test) {
			matches := fs.ForTest("/third_party/test_name2", &BuilderConfig{})
			assert.Loosely(t, len(matches), should.Equal(1))
		})

		t.Run("multiple matches", func(t *ftt.Test) {
			matches := fs.ForTest("/third_party/test_dir/foo_bar/zippy.html", &BuilderConfig{})
			assert.Loosely(t, len(matches), should.Equal(1))
			assert.Loosely(t, matches[0].Original, should.Equal("/third_party/test_dir/foo_bar [ PASS ]"))
		})

		t.Run("modifier expansion", func(t *ftt.Test) {
			matches := fs.ForTest("/third_party/test_dir/foo_bar/baz.html", &BuilderConfig{Specifiers: []string{"Mac10.11"}})
			t.Logf("matches: %v", matches)
			assert.Loosely(t, len(matches), should.Equal(2))
			// Eventually should only return 1 result. For now, most specific first.
			assert.Loosely(t, matches[0].Original, should.Equal("[ Mac ] /third_party/test_dir/foo_bar/baz.html [ FAIL ]"))
			assert.Loosely(t, matches[1].Original, should.Equal("/third_party/test_dir/foo_bar [ PASS ]"))

			// The first (and eventually, only) ExpectationStatement should override the rest.
			assert.Loosely(t, matches[0].Overrides(matches[1]), should.BeTrue)
		})

		t.Run("rule has similar modifier, but doesn't apply", func(t *ftt.Test) {
			matches := fs.ForTest("/third_party/test_dir/zippy.html", &BuilderConfig{Specifiers: []string{"Mac10.11", "Release"}})
			assert.Loosely(t, len(matches), should.BeZero)
			matches = fs.ForTest("/third_party/test_dir/zippy.html", &BuilderConfig{Specifiers: []string{"Mac10.12"}})
			assert.Loosely(t, len(matches), should.Equal(1))
		})
	})
}

type giMock struct {
	info.RawInterface
	token  string
	expiry time.Time
	err    error
}

func (gi giMock) AccessToken(scopes ...string) (token string, expiry time.Time, err error) {
	return gi.token, gi.expiry, gi.err
}

func TestLoadAll(t *testing.T) {
	c := gaetesting.TestingContext()
	c = info.SetFactory(c, func(ic context.Context) info.RawInterface {
		return giMock{dummy.Info(), "", time.Now(), nil}
	})
	c = urlfetch.Set(c, &testhelper.MockGitilesTransport{
		Responses: map[string]string{
			"http://foo.bar": `unused`,
		},
	})

	ftt.Run("load all, error", t, func(t *ftt.Test) {
		all, err := LoadAll(c)
		assert.Loosely(t, all, should.BeNil)
		assert.Loosely(t, err, should.NotBeNil)
	})
}
