// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gitiles

import (
	"context"
	"regexp"
	"testing"

	"google.golang.org/grpc/codes"

	gitilespb "go.chromium.org/luci/common/proto/gitiles"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/grpc/grpcutil"

	"infra/chromium/bootstrapper/clients/gitiles"
)

func TestFactory(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ftt.Run("Factory", t, func(t *ftt.Test) {

		t.Run("returns an RPC client by default", func(t *ftt.Test) {
			factory := Factory(nil)

			client, err := factory(ctx, "fake-host")

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, client, should.NotBeNil)
		})

		t.Run("fails for a nil host", func(t *ftt.Test) {
			factory := Factory(map[string]*Host{
				"fake-host": nil,
			})

			client, err := factory(ctx, "fake-host")

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, client, should.BeNil)
		})

		t.Run("returns RPC client for provided host", func(t *ftt.Test) {
			host := &Host{}
			factory := Factory(map[string]*Host{
				"fake-host": host,
			})

			client, err := factory(ctx, "fake-host")

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, client, should.Resemble(&Client{
				hostname: "fake-host",
				gitiles:  host,
			}))
		})
	})
}

func TestLog(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ftt.Run("gitilesClient.Log", t, func(t *ftt.Test) {

		t.Run("returns requested number of revisions by default", func(t *ftt.Test) {
			client, _ := Factory(nil)(ctx, "fake-host")

			response, err := client.Log(ctx, &gitilespb.LogRequest{
				Project:    "fake/project",
				Committish: "refs/heads/fake-branch",
				PageSize:   4,
			})

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, response, should.NotBeNil)
			assert.Loosely(t, response.Log, should.HaveLength(4))
			assert.Loosely(t, response.Log[0].Id, should.NotBeEmpty)
			assert.Loosely(t, response.Log[1].Id, should.NotBeEmpty)
			assert.Loosely(t, response.Log[2].Id, should.NotBeEmpty)
			assert.Loosely(t, response.Log[3].Id, should.NotBeEmpty)
		})

		t.Run("fails for a nil project", func(t *ftt.Test) {
			client, _ := Factory(map[string]*Host{
				"fake-host": {
					Projects: map[string]*Project{
						"fake/project": nil,
					},
				},
			})(ctx, "fake-host")

			response, err := client.Log(ctx, &gitilespb.LogRequest{
				Project:    "fake/project",
				Committish: "refs/heads/fake-branch",
				PageSize:   1,
			})

			assert.Loosely(t, err, should.ErrLike(`unknown project "fake/project" on host "fake-host"`))
			assert.Loosely(t, response, should.BeNil)
		})

		t.Run("fails for an empty ref revision", func(t *ftt.Test) {
			client, _ := Factory(map[string]*Host{
				"fake-host": {
					Projects: map[string]*Project{
						"fake/project": {
							Refs: map[string]string{
								"refs/heads/fake-branch": "",
							},
						},
					},
				},
			})(ctx, "fake-host")

			response, err := client.Log(ctx, &gitilespb.LogRequest{
				Project:    "fake/project",
				Committish: "refs/heads/fake-branch",
				PageSize:   1,
			})

			assert.Loosely(t, err, should.ErrLike(`unknown ref "refs/heads/fake-branch" for project "fake/project" on host "fake-host"`))
			assert.Loosely(t, response, should.BeNil)
		})

		t.Run("returns log for provided revision", func(t *ftt.Test) {
			client, _ := Factory(map[string]*Host{
				"fake-host": {
					Projects: map[string]*Project{
						"fake/project": {
							Refs: map[string]string{
								"refs/heads/fake-branch": "fake-revision",
							},
							Revisions: map[string]*Revision{
								"fake-revision": {
									Parent: "fake-parent-revision",
								},
							},
						},
					},
				},
			})(ctx, "fake-host")

			response, err := client.Log(ctx, &gitilespb.LogRequest{
				Project:    "fake/project",
				Committish: "refs/heads/fake-branch",
				PageSize:   3,
			})

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, response, should.NotBeNil)
			assert.Loosely(t, response.Log, should.HaveLength(3))
			assert.Loosely(t, response.Log[0].Id, should.Equal("fake-revision"))
			assert.Loosely(t, response.Log[1].Id, should.Equal("fake-parent-revision"))
			assert.Loosely(t, response.Log[2].Id, should.NotBeEmpty)
		})

		t.Run("returns log for known revision", func(t *ftt.Test) {
			client, _ := Factory(map[string]*Host{
				"fake-host": {
					Projects: map[string]*Project{
						"fake/project": {
							Revisions: map[string]*Revision{
								"fake-revision": {
									Parent: "fake-parent-revision",
								},
							},
						},
					},
				},
			})(ctx, "fake-host")

			response, err := client.Log(ctx, &gitilespb.LogRequest{
				Project:    "fake/project",
				Committish: "fake-revision",
				PageSize:   3,
			})

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, response, should.NotBeNil)
			assert.Loosely(t, response.Log, should.HaveLength(3))
			assert.Loosely(t, response.Log[0].Id, should.Equal("fake-revision"))
			assert.Loosely(t, response.Log[1].Id, should.Equal("fake-parent-revision"))
			assert.Loosely(t, response.Log[2].Id, should.NotBeEmpty)
		})

		t.Run("fails for path by default", func(t *ftt.Test) {
			client, _ := Factory(nil)(ctx, "fake-host")

			response, err := client.Log(ctx, &gitilespb.LogRequest{
				Project:    "fake/project",
				Committish: "fake-revision",
				Path:       "fake/path",
				PageSize:   3,
			})

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, grpcutil.Code(err), should.Equal(codes.NotFound))
			assert.Loosely(t, response, should.BeNil)
		})

		t.Run("returns log for path", func(t *ftt.Test) {
			client, _ := Factory(map[string]*Host{
				"fake-host": {
					Projects: map[string]*Project{
						"fake/project": {
							Revisions: map[string]*Revision{
								"fake-revision-not-touching-path": {
									Parent: "fake-revision-touching-path",
								},
								"fake-revision-touching-path": {
									Parent: "fake-revision-not-touching-path-2",
									Files: map[string]*PathObject{
										"fake/path/foo": File("modified-foo-content"),
									},
								},
								"fake-revision-not-touching-path-2": {
									Parent: "fake-revision-touching-path-2",
									Files: map[string]*PathObject{
										"some/other/path": File("some-other-content"),
									},
								},
								"fake-revision-touching-path-2": {
									Parent: "fake-revision-touching-path-3",
									Files: map[string]*PathObject{
										"fake/path/bar": File("new-bar-content"),
									},
								},
								"fake-revision-touching-path-3": {
									Parent: "fake-revision-touching-path-4",
									Files: map[string]*PathObject{
										"fake/path":     nil,
										"fake/path/foo": File("new-foo-content"),
									},
								},
								"fake-revision-touching-path-4": {
									Files: map[string]*PathObject{
										"fake/path": File("new-fake-path-content"),
									},
								},
							},
						},
					},
				},
			})(ctx, "fake-host")

			response, err := client.Log(ctx, &gitilespb.LogRequest{
				Project:    "fake/project",
				Committish: "fake-revision-not-touching-path",
				Path:       "fake/path",
				PageSize:   5,
			})

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, response, should.NotBeNil)
			assert.Loosely(t, response.Log, should.HaveLength(4))
			assert.Loosely(t, response.Log[0].Id, should.Equal("fake-revision-touching-path"))
			assert.Loosely(t, response.Log[1].Id, should.Equal("fake-revision-touching-path-2"))
			assert.Loosely(t, response.Log[2].Id, should.Equal("fake-revision-touching-path-3"))
			assert.Loosely(t, response.Log[3].Id, should.Equal("fake-revision-touching-path-4"))
		})

	})
}

func downloadFileRequest(project, revision, path string) *gitilespb.DownloadFileRequest {
	return &gitilespb.DownloadFileRequest{
		Project:    project,
		Committish: revision,
		Path:       path,
		Format:     gitilespb.DownloadFileRequest_TEXT,
	}
}

func downloadFileJSONRequest(project, revision, path string) *gitilespb.DownloadFileRequest {
	return &gitilespb.DownloadFileRequest{
		Project:    project,
		Committish: revision,
		Path:       path,
		Format:     gitilespb.DownloadFileRequest_JSON,
	}
}

func TestDownloadFile(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ftt.Run("gitilesClient.DownloadFile", t, func(t *ftt.Test) {

		t.Run("fails by default", func(t *ftt.Test) {
			client, _ := Factory(nil)(ctx, "fake-host")

			response, err := client.DownloadFile(ctx, downloadFileRequest("fake/project", "fake-revision", "fake/file"))

			assert.Loosely(t, err, should.ErrLike(`unknown file "fake/file"`))
			assert.Loosely(t, response, should.BeNil)
		})

		t.Run("fails for a nil project", func(t *ftt.Test) {
			client, _ := Factory(map[string]*Host{
				"fake-host": {
					Projects: map[string]*Project{
						"fake/project": nil,
					},
				},
			})(ctx, "fake-host")

			response, err := client.DownloadFile(ctx, downloadFileRequest("fake/project", "fake-revision", "fake/file"))

			assert.Loosely(t, err, should.ErrLike(`unknown project "fake/project" on host "fake-host"`))
			assert.Loosely(t, response, should.BeNil)
		})

		t.Run("fails for a nil revision", func(t *ftt.Test) {
			client, _ := Factory(map[string]*Host{
				"fake-host": {
					Projects: map[string]*Project{
						"fake/project": {
							Revisions: map[string]*Revision{
								"fake-revision": nil,
							},
						},
					},
				},
			})(ctx, "fake-host")

			response, err := client.DownloadFile(ctx, downloadFileRequest("fake/project", "fake-revision", "fake/file"))

			assert.Loosely(t, err, should.ErrLike(`unknown revision "fake-revision" of project "fake/project" on host "fake-host"`))
			assert.Loosely(t, response, should.BeNil)
		})

		t.Run("TEXT format", func(t *ftt.Test) {

			t.Run("returns contents for provided file at revision", func(t *ftt.Test) {
				client, _ := Factory(map[string]*Host{
					"fake-host": {
						Projects: map[string]*Project{
							"fake/project": {
								Revisions: map[string]*Revision{
									"fake-revision": {
										Files: map[string]*PathObject{
											"fake/file": File("fake-contents"),
										},
									},
								},
							},
						},
					},
				})(ctx, "fake-host")

				response, err := client.DownloadFile(ctx, downloadFileRequest("fake/project", "fake-revision", "fake/file"))

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, response, should.NotBeNil)
				assert.Loosely(t, response.Contents, should.Equal("fake-contents"))
			})

			t.Run("returns contents for provided file at revision where file is not affected", func(t *ftt.Test) {
				client, _ := Factory(map[string]*Host{
					"fake-host": {
						Projects: map[string]*Project{
							"fake/project": {
								Revisions: map[string]*Revision{
									"fake-revision-1": {
										Files: map[string]*PathObject{
											"fake/file": File("fake-contents"),
										},
									},
									"fake-revision-2": {
										Parent: "fake-revision-1",
									},
								},
							},
						},
					},
				})(ctx, "fake-host")

				response, err := client.DownloadFile(ctx, downloadFileRequest("fake/project", "fake-revision-2", "fake/file"))

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, response, should.NotBeNil)
				assert.Loosely(t, response.Contents, should.Equal("fake-contents"))
			})

			t.Run("fails for nil contents", func(t *ftt.Test) {
				client, _ := Factory(map[string]*Host{
					"fake-host": {
						Projects: map[string]*Project{
							"fake/project": {
								Revisions: map[string]*Revision{
									"fake-revision": {
										Files: map[string]*PathObject{
											"fake/file": nil,
										},
									},
								},
							},
						},
					},
				})(ctx, "fake-host")

				response, err := client.DownloadFile(ctx, downloadFileRequest("fake/project", "fake-revision", "fake/file"))

				assert.Loosely(t, err, should.ErrLike(`unknown file "fake/file" at revision "fake-revision" of project "fake/project" on host "fake-host"`))
				assert.Loosely(t, response, should.BeNil)
			})

		})

		t.Run("JSON format", func(t *ftt.Test) {

			t.Run("returns contents for provided file at revision", func(t *ftt.Test) {
				client, _ := Factory(map[string]*Host{
					"fake-host": {
						Projects: map[string]*Project{
							"fake/project": {
								Revisions: map[string]*Revision{
									"fake-revision": {
										Files: map[string]*PathObject{
											"fake/submodule": Submodule("fake-submodule-revision"),
										},
									},
								},
							},
						},
					},
				})(ctx, "fake-host")

				response, err := client.DownloadFile(ctx, downloadFileJSONRequest("fake/project", "fake-revision", "fake/submodule"))

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, response, should.NotBeNil)
				assert.Loosely(t, response.Contents, should.Equal(`{"revision": "fake-submodule-revision"}`))
			})

			t.Run("returns contents for provided file at revision where file is not affected", func(t *ftt.Test) {
				client, _ := Factory(map[string]*Host{
					"fake-host": {
						Projects: map[string]*Project{
							"fake/project": {
								Revisions: map[string]*Revision{
									"fake-revision-1": {
										Files: map[string]*PathObject{
											"fake/submodule": Submodule("fake-submodule-revision"),
										},
									},
									"fake-revision-2": {
										Parent: "fake-revision-1",
									},
								},
							},
						},
					},
				})(ctx, "fake-host")

				response, err := client.DownloadFile(ctx, downloadFileJSONRequest("fake/project", "fake-revision-2", "fake/submodule"))

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, response, should.NotBeNil)
				assert.Loosely(t, response.Contents, should.Equal(`{"revision": "fake-submodule-revision"}`))
			})

			t.Run("fails for nil contents", func(t *ftt.Test) {
				client, _ := Factory(map[string]*Host{
					"fake-host": {
						Projects: map[string]*Project{
							"fake/project": {
								Revisions: map[string]*Revision{
									"fake-revision": {
										Files: map[string]*PathObject{
											"fake/submodule": nil,
										},
									},
								},
							},
						},
					},
				})(ctx, "fake-host")

				response, err := client.DownloadFile(ctx, downloadFileJSONRequest("fake/project", "fake-revision", "fake/submodule"))

				assert.Loosely(t, err, should.ErrLike(`unknown file "fake/submodule" at revision "fake-revision" of project "fake/project" on host "fake-host"`))
				assert.Loosely(t, response, should.BeNil)
			})

		})

	})
}

func TestDownloadDiff(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ftt.Run("gitilesClient.DownloadFile", t, func(t *ftt.Test) {

		t.Run("returns an empty diff by default", func(t *ftt.Test) {
			client, _ := Factory(nil)(ctx, "fake-host")

			response, err := client.DownloadDiff(ctx, &gitilespb.DownloadDiffRequest{
				Project:    "fake/project",
				Committish: "fake-revision",
			})

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, response, should.NotBeNil)
			assert.Loosely(t, response.Contents, should.BeEmpty)
		})

		t.Run("fails for a nil project", func(t *ftt.Test) {
			client, _ := Factory(map[string]*Host{
				"fake-host": {
					Projects: map[string]*Project{
						"fake/project": nil,
					},
				},
			})(ctx, "fake-host")

			response, err := client.DownloadDiff(ctx, &gitilespb.DownloadDiffRequest{
				Project:    "fake/project",
				Committish: "fake-revision",
			})

			assert.Loosely(t, err, should.ErrLike(`unknown project "fake/project" on host "fake-host"`))
			assert.Loosely(t, response, should.BeNil)
		})

		t.Run("fails for a nil revision", func(t *ftt.Test) {
			client, _ := Factory(map[string]*Host{
				"fake-host": {
					Projects: map[string]*Project{
						"fake/project": {
							Revisions: map[string]*Revision{
								"fake-revision": nil,
							},
						},
					},
				},
			})(ctx, "fake-host")

			response, err := client.DownloadDiff(ctx, &gitilespb.DownloadDiffRequest{
				Project:    "fake/project",
				Committish: "fake-revision",
			})

			assert.Loosely(t, err, should.ErrLike(`unknown revision "fake-revision" of project "fake/project" on host "fake-host"`))
			assert.Loosely(t, response, should.BeNil)
		})

		t.Run("returns difference between revision and parent", func(t *ftt.Test) {
			client, _ := Factory(map[string]*Host{
				"fake-host": {
					Projects: map[string]*Project{
						"fake/project": {
							Revisions: map[string]*Revision{
								"fake-revision-1": {
									Files: map[string]*PathObject{
										"to-re-add": File("fake-contents-to-be-re-added\n"),
									},
								},
								"fake-revision-2": {
									Files: map[string]*PathObject{
										"to-modify":         File("fake-contents-1\n"),
										"to-make-non-empty": File(""),
										"to-clear":          File("fake-contents-to-be-removed\n"),
										"to-delete":         File("fake-contents-for-file-to-be-deleted\n"),
										"to-re-add":         nil,
									},
									Parent: "fake-revision-1",
								},
								"fake-revision-3": {
									Files: map[string]*PathObject{
										"to-modify":         File("fake-contents-2\n"),
										"to-make-non-empty": File("fake-contents-added\n"),
										"to-add":            File("fake-contents-for-new-file\n"),
										"to-clear":          File(""),
										"to-delete":         nil,
										"to-re-add":         File("fake-contents-to-be-re-added\n"),
									},
									Parent: "fake-revision-2",
								},
							},
						},
					},
				},
			})(ctx, "fake-host")

			t.Run("for all files if no path is specified", func(t *ftt.Test) {
				response, err := client.DownloadDiff(ctx, &gitilespb.DownloadDiffRequest{
					Project:    "fake/project",
					Committish: "fake-revision-3",
				})

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, response, should.NotBeNil)
				// (?m) - multiline regex mode: ^ matches line start
				contents := regexp.MustCompile(`(?m)^index [0-9a-f]+\.\.[0-9a-f]+`).ReplaceAllLiteralString(response.Contents, "index A..B")
				assert.Loosely(t, contents, should.Equal(
					`diff --git a/to-add b/to-add
new file mode 100644
index A..B
--- /dev/null
+++ b/to-add
@@ -0,0 +1 @@
+fake-contents-for-new-file
diff --git a/to-clear b/to-clear
index A..B 100644
--- a/to-clear
+++ b/to-clear
@@ -1 +0,0 @@
-fake-contents-to-be-removed
diff --git a/to-delete b/to-delete
deleted file mode 100644
index A..B
--- a/to-delete
+++ /dev/null
@@ -1 +0,0 @@
-fake-contents-for-file-to-be-deleted
diff --git a/to-make-non-empty b/to-make-non-empty
index A..B 100644
--- a/to-make-non-empty
+++ b/to-make-non-empty
@@ -0,0 +1 @@
+fake-contents-added
diff --git a/to-modify b/to-modify
index A..B 100644
--- a/to-modify
+++ b/to-modify
@@ -1 +1 @@
-fake-contents-1
+fake-contents-2
diff --git a/to-re-add b/to-re-add
new file mode 100644
index A..B
--- /dev/null
+++ b/to-re-add
@@ -0,0 +1 @@
+fake-contents-to-be-re-added
`))

			})

			t.Run("for individual file if path is specified", func(t *ftt.Test) {
				response, err := client.DownloadDiff(ctx, &gitilespb.DownloadDiffRequest{
					Project:    "fake/project",
					Committish: "fake-revision-3",
					Path:       "to-modify",
				})

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, response, should.NotBeNil)
				// (?m) - multiline regex mode: ^ matches line start
				contents := regexp.MustCompile(`(?m)^index [0-9a-f]+\.\.[0-9a-f]+`).ReplaceAllLiteralString(response.Contents, "index A..B")
				assert.Loosely(t, contents, should.Equal(
					`diff --git a/to-modify b/to-modify
index A..B 100644
--- a/to-modify
+++ b/to-modify
@@ -1 +1 @@
-fake-contents-1
+fake-contents-2
`))
			})

		})

		t.Run("returns difference between revisions if base is specified", func(t *ftt.Test) {
			client, _ := Factory(map[string]*Host{
				"fake-host": {
					Projects: map[string]*Project{
						"fake/project": {
							Revisions: map[string]*Revision{
								"fake-revision-1": {
									Files: map[string]*PathObject{
										"to-modify": File("fake-contents-A\n"),
									},
								},
								"fake-revision-2": {
									Files: map[string]*PathObject{
										"to-modify": File("fake-contents-A\n"),
									},
									Parent: "fake-revision-1",
								},
								"fake-revision-3": {
									Files: map[string]*PathObject{
										"to-modify": File("fake-contents-B\n"),
									},
									Parent: "fake-revision-1",
								},
							},
						},
					},
				},
			})(ctx, "fake-host")

			response, err := client.DownloadDiff(ctx, &gitilespb.DownloadDiffRequest{
				Project:    "fake/project",
				Committish: "fake-revision-2",
				Base:       "fake-revision-3",
			})

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, response, should.NotBeNil)
			// (?m) - multiline regex mode: ^ matches line start
			contents := regexp.MustCompile(`(?m)^index [0-9a-f]+\.\.[0-9a-f]+`).ReplaceAllLiteralString(response.Contents, "index A..B")
			assert.Loosely(t, contents, should.Equal(
				`diff --git a/to-modify b/to-modify
index A..B 100644
--- a/to-modify
+++ b/to-modify
@@ -1 +1 @@
-fake-contents-B
+fake-contents-A
`))
		})

	})

}

func TestIntegration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ftt.Run("gitiles using fake factory", t, func(t *ftt.Test) {

		t.Run("succeeds when calling FetchLatestRevision", func(t *ftt.Test) {
			ctx := gitiles.UseGitilesClientFactory(ctx, Factory(nil))
			client := gitiles.NewClient(ctx)

			revision, err := client.FetchLatestRevision(ctx, "fake-host", "fake/project", "refs/heads/fake-branch")

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, revision, should.NotBeEmpty)
		})

		t.Run("succeeds when calling GetSubmoduleRevision", func(t *ftt.Test) {
			ctx := gitiles.UseGitilesClientFactory(ctx, Factory(map[string]*Host{
				"fake-host": {
					Projects: map[string]*Project{
						"fake/project": {
							Revisions: map[string]*Revision{
								"fake-revision": {
									Files: map[string]*PathObject{
										"fake/submodule": Submodule("fake-submodule-revision"),
									},
								},
							},
						},
					},
				},
			}))
			client := gitiles.NewClient(ctx)

			revision, err := client.GetSubmoduleRevision(ctx, "fake-host", "fake/project", "fake-revision", "fake/submodule")

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, revision, should.Equal("fake-submodule-revision"))

		})

		t.Run("succeeds when calling DownloadFile", func(t *ftt.Test) {
			ctx := gitiles.UseGitilesClientFactory(ctx, Factory(map[string]*Host{
				"fake-host": {
					Projects: map[string]*Project{
						"fake/project": {
							Revisions: map[string]*Revision{
								"fake-revision": {
									Files: map[string]*PathObject{
										"fake/file": File("fake-contents"),
									},
								},
							},
						},
					},
				},
			}))
			client := gitiles.NewClient(ctx)

			contents, err := client.DownloadFile(ctx, "fake-host", "fake/project", "fake-revision", "fake/file")

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, contents, should.Equal("fake-contents"))
		})

		t.Run("succeeds when calling DownloadDiff", func(t *ftt.Test) {

			t.Run("with diff against parent when called for modified path", func(t *ftt.Test) {
				ctx := gitiles.UseGitilesClientFactory(ctx, Factory(map[string]*Host{
					"fake-host": {
						Projects: map[string]*Project{
							"fake/project": {
								Revisions: map[string]*Revision{
									"fake-revision-1": {
										Files: map[string]*PathObject{
											"fake/file": File("fake-contents-1"),
										},
									},
									"fake-revision-2": {
										Files: map[string]*PathObject{
											"fake/file": File("fake-contents-2"),
										},
										Parent: "fake-revision-1",
									},
								},
							},
						},
					},
				}))
				client := gitiles.NewClient(ctx)

				contents, err := client.DownloadDiff(ctx, "fake-host", "fake/project", "fake-revision-2", gitiles.PARENT, "fake/file")

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, contents, should.NotBeEmpty)
			})

			t.Run("with no diff against parent when called for unmodified path", func(t *ftt.Test) {
				ctx := gitiles.UseGitilesClientFactory(ctx, Factory(map[string]*Host{
					"fake-host": {
						Projects: map[string]*Project{
							"fake/project": {
								Revisions: map[string]*Revision{
									"fake-revision-1": {
										Files: map[string]*PathObject{
											"fake/file": File("fake-contents-1"),
										},
									},
									"fake-revision-2": {
										Files: map[string]*PathObject{
											"fake/file": File("fake-contents-2"),
										},
										Parent: "fake-revision-1",
									},
								},
							},
						},
					},
				}))
				client := gitiles.NewClient(ctx)

				contents, err := client.DownloadDiff(ctx, "fake-host", "fake/project", "fake-revision-2", gitiles.PARENT, "other/fake/file")

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, contents, should.BeEmpty)
			})

			t.Run("with diff against revision when called for modified path", func(t *ftt.Test) {
				ctx := gitiles.UseGitilesClientFactory(ctx, Factory(map[string]*Host{
					"fake-host": {
						Projects: map[string]*Project{
							"fake/project": {
								Revisions: map[string]*Revision{
									"fake-revision-1": {
										Files: map[string]*PathObject{
											"fake/file": File("fake-contents-A"),
										},
									},
									"fake-revision-2": {
										Files: map[string]*PathObject{
											"fake/file": File("fake-contents-A"),
										},
										Parent: "fake-revision-1",
									},
									"fake-revision-3": {
										Files: map[string]*PathObject{
											"fake/file": File("fake-contents-B"),
										},
									},
								},
							},
						},
					},
				}))
				client := gitiles.NewClient(ctx)

				contents, err := client.DownloadDiff(ctx, "fake-host", "fake/project", "fake-revision-2", "fake-revision-3", "fake/file")

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, contents, should.NotBeEmpty)
			})

			t.Run("with no diff against revision when called for unmodified path", func(t *ftt.Test) {
				ctx := gitiles.UseGitilesClientFactory(ctx, Factory(map[string]*Host{
					"fake-host": {
						Projects: map[string]*Project{
							"fake/project": {
								Revisions: map[string]*Revision{
									"fake-revision-1": {
										Files: map[string]*PathObject{
											"fake/file": File("fake-contents-A"),
										},
									},
									"fake-revision-2": {
										Files: map[string]*PathObject{
											"fake/file": File("fake-contents-B"),
										},
										Parent: "fake-revision-1",
									},
									"fake-revision-3": {
										Files: map[string]*PathObject{
											"fake/file": File("fake-contents-B"),
										},
									},
								},
							},
						},
					},
				}))
				client := gitiles.NewClient(ctx)

				contents, err := client.DownloadDiff(ctx, "fake-host", "fake/project", "fake-revision-2", "fake-revision-3", "fake/file")

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, contents, should.BeEmpty)
			})

		})

	})
}
