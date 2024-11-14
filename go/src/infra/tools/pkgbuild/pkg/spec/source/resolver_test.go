// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package source

import (
	"context"
	"os"
	"testing"

	"go.chromium.org/luci/common/exec/execmock"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/tools/pkgbuild/pkg/spec"
)

func TestResolveSourceSpec(t *testing.T) {
	ftt.Run("resolve", t, func(t *ftt.Test) {
		ctx := execmock.Init(context.Background())
		r := NewResolver()

		t.Run("git", func(t *ftt.Test) {
			execmock.Simple.Mock(ctx, execmock.SimpleInput{
				Stdout: `{"version": "1.5.6", "git": {"url": "https://github.com/facebook/zstd", "commit": "35016bc1c0b9a2f7121b7ecc312100aad7d9f2ad"}}`,
			})
			info, err := r.Resolve(ctx, "linux-amd64", "", &spec.Spec_Create_Source{
				Method: &spec.Spec_Create_Source_Git{
					Git: &spec.GitSource{
						Repo:       "https://github.com/facebook/zstd",
						TagPattern: "v%s",
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, info, should.Match(&SourceInfo{
				Version: "1.5.6",
				Source: &SourceInfo_Git_{
					Git: &SourceInfo_Git{
						Url:    "https://github.com/facebook/zstd",
						Commit: "35016bc1c0b9a2f7121b7ecc312100aad7d9f2ad",
					},
				},
			}))
		})
		t.Run("script", func(t *ftt.Test) {
			f, err := os.CreateTemp(t.TempDir(), "*.py")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, f.Close(), should.BeNil)

			execmock.Simple.WithArgs("=latest").Mock(ctx, execmock.SimpleInput{
				Stdout: `3.11.9`,
			})
			execmock.Simple.WithArgs("=get_url").Mock(ctx, execmock.SimpleInput{
				Stdout: `{"url": ["https://www.python.org/ftp/python/3.11.9/amd64/core.msi", "https://www.python.org/ftp/python/3.11.9/amd64/core_d.msi", "https://www.python.org/ftp/python/3.11.9/amd64/core_pdb.msi"], "name": ["core.msi", "core_d.msi", "core_pdb.msi"], "ext": ".msi"}`,
			})

			src := &spec.Spec_Create_Source{
				Method: &spec.Spec_Create_Source_Script{
					Script: &spec.ScriptSource{
						Name: []string{f.Name()},
					},
				},
			}

			t.Run("ok", func(t *ftt.Test) {
				info, err := r.Resolve(ctx, "windows-amd64", "", src)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, info, should.Match(&SourceInfo{
					Version: "3.11.9",
					Source: &SourceInfo_Http{
						Http: &SourceInfo_HTTP{
							Url: []string{
								"https://www.python.org/ftp/python/3.11.9/amd64/core.msi",
								"https://www.python.org/ftp/python/3.11.9/amd64/core_d.msi",
								"https://www.python.org/ftp/python/3.11.9/amd64/core_pdb.msi",
							},
							Name: []string{"core.msi", "core_d.msi", "core_pdb.msi"},
							Ext:  ".msi",
						},
					},
				}))
			})

			t.Run("fetch checkout", func(t *ftt.Test) {
				src.GetScript().UseFetchCheckoutWorkflow = true
				info, err := r.Resolve(ctx, "windows-amd64", "", src)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, info, should.Match(&SourceInfo{
					Version: "3.11.9",
				}))
			})
		})
		t.Run("url", func(t *ftt.Test) {
			info, err := r.Resolve(ctx, "linux-amd64", "", &spec.Spec_Create_Source{
				Method: &spec.Spec_Create_Source_Url{
					Url: &spec.UrlSource{
						DownloadUrl: "https://zlib.net/fossils/zlib-1.2.12.tar.gz",
						Version:     "1.2.12",
					},
				},
				UnpackArchive:  true,
				CpeBaseAddress: "cpe:/a:zlib:zlib",
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, info, should.Match(&SourceInfo{
				Version: "1.2.12",
				Source: &SourceInfo_Http{
					Http: &SourceInfo_HTTP{
						Url: []string{"https://zlib.net/fossils/zlib-1.2.12.tar.gz"},
						Ext: ".tar.gz",
					},
				},
			}))
		})
		t.Run("cipd", func(t *ftt.Test) {
			_, err := r.Resolve(ctx, "linux-amd64", "", &spec.Spec_Create_Source{
				Method: &spec.Spec_Create_Source_Cipd{
					Cipd: &spec.CipdSource{},
				},
				UnpackArchive:  true,
				CpeBaseAddress: "cpe:/a:zlib:zlib",
			})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("not implemented"))
		})
	})
}
