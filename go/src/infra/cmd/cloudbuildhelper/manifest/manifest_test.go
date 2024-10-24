// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package manifest

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestManifest(t *testing.T) {
	t.Parallel()

	load := func(body, path string) (*Manifest, error) {
		m, err := parse(strings.NewReader(body), filepath.FromSlash(path))
		if err != nil {
			return nil, err
		}
		return m, m.Finalize()
	}

	ftt.Run("Minimal", t, func(t *ftt.Test) {
		m, err := load("name: zzz\ncontextdir: ../../../blarg/", "root/1/2/3/4")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, m, should.Resemble(&Manifest{
			Name:        "zzz",
			ManifestDir: filepath.FromSlash("root/1/2/3/4"),
			ContextDir:  filepath.FromSlash("root/1/blarg"),
			InputsDir:   filepath.FromSlash("root/1/blarg"),
			Sources:     []string{filepath.FromSlash("root/1/blarg")},
		}))
	})

	ftt.Run("No name", t, func(t *ftt.Test) {
		_, err := load("", "some/dir")
		assert.Loosely(t, err, should.ErrLike(`bad "name" field: can't be empty, it's required`))
	})

	ftt.Run("Bad name", t, func(t *ftt.Test) {
		_, err := load(`name: cheat:tag`, "some/dir")
		assert.Loosely(t, err, should.ErrLike(`bad "name" field: "cheat:tag" contains forbidden symbols (any of "\\:@")`))
	})

	ftt.Run("Not yaml", t, func(t *ftt.Test) {
		_, err := load(`im not a YAML`, "")
		assert.Loosely(t, err, should.ErrLike("unmarshal errors"))
	})

	ftt.Run("Deriving contextdir from dockerfile", t, func(t *ftt.Test) {
		m, err := load("name: zzz\ndockerfile: ../../../blarg/Dockerfile", "root/1/2/3/4")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, m, should.Resemble(&Manifest{
			Name:        "zzz",
			ManifestDir: filepath.FromSlash("root/1/2/3/4"),
			Dockerfile:  filepath.FromSlash("root/1/blarg/Dockerfile"),
			ContextDir:  filepath.FromSlash("root/1/blarg"),
			InputsDir:   filepath.FromSlash("root/1/blarg"),
			Sources:     []string{filepath.FromSlash("root/1/blarg")},
		}))
	})

	ftt.Run("Resolving imagepins", t, func(t *ftt.Test) {
		m, err := load("name: zzz\ncontextdir: .\nimagepins: ../../../blarg/pins.yaml", "root/1/2/3/4")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, m, should.Resemble(&Manifest{
			Name:        "zzz",
			ManifestDir: filepath.FromSlash("root/1/2/3/4"),
			ContextDir:  filepath.FromSlash("root/1/2/3/4"),
			InputsDir:   filepath.FromSlash("root/1/2/3/4"),
			Sources:     []string{filepath.FromSlash("root/1/2/3/4")},
			ImagePins:   filepath.FromSlash("root/1/blarg/pins.yaml"),
		}))
	})

	ftt.Run("Empty build step", t, func(t *ftt.Test) {
		_, err := load(`{"name": "zzz", "contextdir": ".", "build": [
			{"dest": "zzz"}
		]}`, "root/1/2/3/4")
		assert.Loosely(t, err, should.ErrLike("bad build step #1: unrecognized or empty"))
	})

	ftt.Run("Ambiguous build step", t, func(t *ftt.Test) {
		_, err := load(`{"name": "zzz", "contextdir": ".", "build": [
			{"copy": "zzz", "go_binary": "zzz"}
		]}`, "root/1/2/3/4")
		assert.Loosely(t, err, should.ErrLike("bad build step #1: ambiguous"))
	})

	ftt.Run("CopyBuildStep", t, func(t *ftt.Test) {
		m, err := load(`{"name": "zzz", "contextdir": "ctx", "build": [
				{"copy": "${manifestdir}/../../../blarg/zzz"}
			]}`, "root/1/2/3/4")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, m.Build, should.HaveLength(1))
		assert.Loosely(t, m.Build[0].Dest, should.Equal(filepath.FromSlash("root/1/2/3/4/ctx/zzz")))
		assert.Loosely(t, m.Build[0].Concrete(), should.Resemble(&CopyBuildStep{
			Copy: filepath.FromSlash("root/1/blarg/zzz"),
		}))
	})

	ftt.Run("GoBuildStep", t, func(t *ftt.Test) {
		m, err := load(`{"name": "zzz", "contextdir": "ctx", "build": [
				{"go_binary": "go.pkg/some/tool"}
			]}`, "root/1/2/3/4")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, m.Build, should.HaveLength(1))
		assert.Loosely(t, m.Build[0].Dest, should.Equal(filepath.FromSlash("root/1/2/3/4/ctx/tool")))
		assert.Loosely(t, m.Build[0].Cwd, should.Equal(filepath.FromSlash("root/1/2/3/4/ctx")))
		assert.Loosely(t, m.Build[0].Concrete(), should.Resemble(&GoBuildStep{
			GoBinary: "go.pkg/some/tool",
		}))
	})

	ftt.Run("RunBuildStep", t, func(t *ftt.Test) {
		m, err := load(`{"name": "zzz", "contextdir": "ctx", "build": [
				{"run": ["a", "b"]}
			]}`, "root/1/2/3/4")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, m.Build, should.HaveLength(1))
		assert.Loosely(t, m.Build[0].Cwd, should.Equal(filepath.FromSlash("root/1/2/3/4/ctx")))
		assert.Loosely(t, m.Build[0].Concrete(), should.Resemble(&RunBuildStep{
			Run: []string{"a", "b"},
		}))
	})

	ftt.Run("GoGAEBundleBuildStep", t, func(t *ftt.Test) {
		m, err := load(`{"name": "zzz", "contextdir": "ctx", "inputsdir": "in", "build": [
				{"go_gae_bundle": "${inputsdir}/pkg", "dest": "${contextdir}/pkg"}
			]}`, "root/1/2/3/4")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, m.Build, should.HaveLength(1))
		assert.Loosely(t, m.Build[0].Concrete(), should.Resemble(&GoGAEBundleBuildStep{
			GoGAEBundle: filepath.FromSlash("root/1/2/3/4/in/pkg"),
		}))
		assert.Loosely(t, m.Build[0].Dest, should.Equal(filepath.FromSlash("root/1/2/3/4/ctx/pkg")))
	})

	ftt.Run("Good infra", t, func(t *ftt.Test) {
		m, err := load(`{"name": "zzz", "contextdir": ".", "infra": {
			"infra1": {
				"storage": "gs://bucket",
				"notify": [
					{
						"kind": "git",
						"repo": "https://repo.example.com",
						"script": "some/script.py"
					}
				]
			},
			"infra2": {
				"storage": "gs://bucket/path"
			}
		}}`, "root/1/2/3/4")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, m.Infra, should.Resemble(map[string]Infra{
			"infra1": {
				Storage: "gs://bucket",
				Notify: []NotifyConfig{
					{
						Kind:   "git",
						Repo:   "https://repo.example.com",
						Script: "some/script.py",
					},
				},
			},
			"infra2": {Storage: "gs://bucket/path"},
		}))
	})

	ftt.Run("Unsupported storage", t, func(t *ftt.Test) {
		_, err := load(`{"name": "zzz", "contextdir": ".", "infra": {
			"infra1": {"storage": "ftp://bucket"}
		}}`, "root/1/2/3/4")
		assert.Loosely(t, err, should.ErrLike(`in infra section "infra1": bad storage "ftp://bucket", only gs:// is supported currently`))
	})

	ftt.Run("No bucket in storage", t, func(t *ftt.Test) {
		_, err := load(`{"name": "zzz", "contextdir": ".", "infra": {
			"infra1": {"storage": "gs:///zzz"}
		}}`, "root/1/2/3/4")
		assert.Loosely(t, err, should.ErrLike(`in infra section "infra1": bad storage "gs:///zzz", bucket name is missing`))
	})

	ftt.Run("Bad notify", t, func(t *ftt.Test) {
		check := func(notify string) error {
			_, err := load(fmt.Sprintf(`{"name": "zzz", "contextdir": ".", "infra": {
			"infra1": {"notify": [%s]}
		}}`, notify), "root/1/2/3/4")
			return err
		}

		t.Run("Bad kind", func(t *ftt.Test) {
			assert.Loosely(t, check(`{"kind": "bad"}`), should.ErrLike(`unsupported notify kind`))
		})

		t.Run("Bad repo", func(t *ftt.Test) {
			assert.Loosely(t, check(`{"kind": "git", "repo": "ftp://zzz"}`),
				should.ErrLike(`should be an https:// URL`))
		})

		t.Run("Bad script path", func(t *ftt.Test) {
			for _, p := range []string{"a/../b", "", "a/./b"} {
				assert.Loosely(t, check(fmt.Sprintf(`{"kind": "git", "repo": "https://zzz", "script": "%s"}`, p)),
					should.ErrLike(`should be a normalized slash-separate path`))
			}
		})

		t.Run("Not relative path", func(t *ftt.Test) {
			for _, p := range []string{"/", "/a/b"} {
				assert.Loosely(t, check(fmt.Sprintf(`{"kind": "git", "repo": "https://zzz", "script": "%s"}`, p)),
					should.ErrLike(`not a path inside the repo`))
			}
		})
	})
}

func TestExtends(t *testing.T) {
	t.Parallel()

	ftt.Run("With temp dir", t, func(t *ftt.Test) {
		dir, err := ioutil.TempDir("", "cloudbuildhelper")
		assert.Loosely(t, err, should.BeNil)
		t.Cleanup(func() { os.RemoveAll(dir) })

		write := func(path string, m Manifest) {
			blob, err := yaml.Marshal(&m)
			assert.Loosely(t, err, should.BeNil)
			p := filepath.Join(dir, filepath.FromSlash(path))
			assert.Loosely(t, os.MkdirAll(filepath.Dir(p), 0777), should.BeNil)
			assert.Loosely(t, ioutil.WriteFile(p, blob, 0666), should.BeNil)
		}

		abs := func(path string) string {
			p, err := filepath.Abs(filepath.Join(dir, filepath.FromSlash(path)))
			assert.Loosely(t, err, should.BeNil)
			return p
		}

		t.Run("Works", func(t *ftt.Test) {
			var falseVal = false

			notifyBase := NotifyConfig{
				Kind:   "git",
				Repo:   "https://base.example.com",
				Script: "base",
			}
			notifyMid := NotifyConfig{
				Kind:   "git",
				Repo:   "https://mid.example.com",
				Script: "mid",
			}

			write("base.yaml", Manifest{
				Name:      "base",
				ImagePins: "pins.yaml",
				Sources:   []string{"base-1", "base-2"},
				Infra: map[string]Infra{
					"base": {
						Storage:  "gs://base-storage",
						Registry: "base-registry",
						Notify:   []NotifyConfig{notifyBase},
					},
				},
				Build: []*BuildStep{
					{CopyBuildStep: CopyBuildStep{Copy: "${manifestdir}/manifest_base.copy"}},
					{CopyBuildStep: CopyBuildStep{Copy: "${contextdir}/context_base.copy"}},
				},
			})

			write("deeper/mid.yaml", Manifest{
				Name:          "mid",
				Extends:       "../base.yaml",
				Deterministic: &falseVal,
				Sources:       []string{"mid-1", "../mid-2", "../base-2"},
				Infra: map[string]Infra{
					"mid": {
						Storage:  "gs://mid-storage",
						Registry: "mid-registry",
						CloudBuild: map[string]CloudBuildBuilder{
							"builder1": {Project: "project-1"},
							"builder2": {Project: "project-2"},
						},
						Notify: []NotifyConfig{notifyMid},
					},
				},
				Build: []*BuildStep{
					{CopyBuildStep: CopyBuildStep{Copy: "${manifestdir}/manifest_mid.copy"}},
					{CopyBuildStep: CopyBuildStep{Copy: "${contextdir}/context_mid.copy"}},
				},
			})

			write("deeper/leaf.yaml", Manifest{
				Name:       "leaf",
				Extends:    "mid.yaml",
				Dockerfile: "dockerfile",
				ContextDir: "context-dir",
				InputsDir:  "inputs-dir",
				Infra: map[string]Infra{
					"mid": { // partial override
						Registry: "leaf-registry",
						CloudBuild: map[string]CloudBuildBuilder{
							"builder1": {Project: "project-1-override"},
							"builder3": {Project: "project-3"},
						},
					},
				},
				Build: []*BuildStep{
					{CopyBuildStep: CopyBuildStep{Copy: "${manifestdir}/manifest_leaf.copy"}},
					{CopyBuildStep: CopyBuildStep{Copy: "${contextdir}/context_leaf.copy"}},
				},
			})

			m, err := Load(filepath.Join(dir, "deeper", "leaf.yaml"))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, m.Finalize(), should.BeNil)

			// We'll deal with them separately below.
			steps := m.Build
			m.Build = nil

			assert.Loosely(t, m, should.Resemble(&Manifest{
				Name:        "leaf",
				ManifestDir: abs("deeper"),
				Dockerfile:  abs("deeper/dockerfile"),
				ContextDir:  abs("deeper/context-dir"),
				InputsDir:   abs("deeper/inputs-dir"),
				Sources: []string{
					abs("deeper/mid-1"),
					abs("mid-2"),
					abs("base-2"),
					abs("base-1"),
				},
				ImagePins:     abs("pins.yaml"),
				Deterministic: &falseVal,
				Infra: map[string]Infra{
					"base": {
						Storage:    "gs://base-storage",
						Registry:   "base-registry",
						Notify:     []NotifyConfig{notifyBase},
						CloudBuild: map[string]CloudBuildBuilder{},
					},
					"mid": {
						Storage:  "gs://mid-storage",
						Registry: "leaf-registry",
						CloudBuild: map[string]CloudBuildBuilder{
							"builder1": {Project: "project-1-override", Args: []string{}},
							"builder2": {Project: "project-2", Args: []string{}},
							"builder3": {Project: "project-3", Args: []string{}},
						},
						Notify: []NotifyConfig{notifyMid},
					},
				},
			}))

			var copySrc []string
			for _, s := range steps {
				copySrc = append(copySrc, s.Copy)
			}
			assert.Loosely(t, copySrc, should.Resemble([]string{
				abs("manifest_base.copy"),
				abs("deeper/context-dir/context_base.copy"),
				abs("deeper/manifest_mid.copy"),
				abs("deeper/context-dir/context_mid.copy"),
				abs("deeper/manifest_leaf.copy"),
				abs("deeper/context-dir/context_leaf.copy"),
			}))
		})

		t.Run("Recursion", func(t *ftt.Test) {
			write("a.yaml", Manifest{Name: "a", Extends: "b.yaml"})
			write("b.yaml", Manifest{Name: "b", Extends: "a.yaml"})

			_, err := Load(filepath.Join(dir, "a.yaml"))
			assert.Loosely(t, err, should.ErrLike("too much nesting"))
		})

		t.Run("Deep error", func(t *ftt.Test) {
			write("a.yaml", Manifest{Name: "a", Extends: "b.yaml"})
			write("b.yaml", Manifest{
				Name: "b",
				Infra: map[string]Infra{
					"base": {Storage: "bad url"},
				},
			})

			_, err := Load(filepath.Join(dir, "a.yaml"))
			assert.Loosely(t, err, should.ErrLike(`bad storage`))
		})
	})
}

func TestRenderPath(t *testing.T) {
	t.Parallel()

	ftt.Run("Works", t, func(t *ftt.Test) {
		out, err := renderPath("var", "${a}", map[string]string{"a": "zzz"})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, out, should.Equal("zzz"))

		out, err = renderPath("var", "${a}/", map[string]string{"a": "zzz"})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, out, should.Equal("zzz"))

		out, err = renderPath("var", "${a}/.", map[string]string{"a": "zzz"})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, out, should.Equal("zzz"))

		out, err = renderPath("var", "${a}/b/c", map[string]string{"a": "zzz"})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, out, should.Equal(filepath.FromSlash("zzz/b/c")))
	})

	ftt.Run("Errors", t, func(t *ftt.Test) {
		_, err := renderPath("var", ".", map[string]string{"a": "zzz", "b": "yyy"})
		assert.Loosely(t, err, should.ErrLike("must start with ${a} or ${b}"))

		_, err = renderPath("var", "${c}", map[string]string{"a": "zzz", "b": "yyy"})
		assert.Loosely(t, err, should.ErrLike("unknown dir variable ${c}, expecting ${a} or ${b}"))
	})
}
