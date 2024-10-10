// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/golang/protobuf/ptypes"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	kpb "infra/cmd/package_index/kythe/proto"
)

func TestRemoveFilepathsFiles(t *testing.T) {
	t.Parallel()
	ftt.Run("Remove filepaths files", t, func(t *ftt.Test) {
		ctx := context.Background()
		tmpdir := t.TempDir()

		t.Run("Remove filepaths with files", func(t *ftt.Test) {
			// File setup
			f, err := os.Create(filepath.Join(tmpdir, "foo.filepaths"))
			assert.Loosely(t, err, should.BeNil)
			f.Close()
			fpath, err := filepath.Abs(f.Name())
			if err != nil {
				t.Fatal(err)
			}

			nested, err := os.MkdirTemp(tmpdir, "")
			assert.Loosely(t, err, should.BeNil)

			b, err := os.Create(filepath.Join(nested, "bar.filepaths"))
			assert.Loosely(t, err, should.BeNil)
			b.Close()
			bpath, err := filepath.Abs(b.Name())
			assert.Loosely(t, err, should.BeNil)

			removeFilepathsFiles(ctx, tmpdir)

			t.Run("tmpdir should be empty while the nested dir should remain unchanged", func(t *ftt.Test) {
				_, err = os.Stat(fpath)
				assert.Loosely(t, os.IsNotExist(err), should.Equal(true))

				_, err = os.Stat(bpath)
				assert.Loosely(t, err, should.BeNil)
			})
		})

		t.Run("Remove filepaths without filepaths files", func(t *ftt.Test) {
			h, err := os.Create(filepath.Join(tmpdir, "hello.txt"))
			if err != nil {
				t.Fatal(err)
			}
			h.Close()
			hpath, err := filepath.Abs(h.Name())
			if err != nil {
				t.Fatal(err)
			}

			removeFilepathsFiles(ctx, tmpdir)

			t.Run("Nothing should have changed", func(t *ftt.Test) {
				_, err = os.Stat(hpath)
				assert.Loosely(t, err, should.BeNil)
			})
		})

	})
}

func TestConvertGnPath(t *testing.T) {
	t.Parallel()
	ftt.Run("Convert GN Path", t, func(t *ftt.Test) {
		ctx := context.Background()
		gnPath := "//path/tests/thing.txt"

		t.Run("Bad outDir", func(t *ftt.Test) {
			assert.Loosely(t, func() {
				convertGnPath(ctx, gnPath, "/bad/dir/")
			}, should.Panic)
		})

		t.Run("GN path", func(t *ftt.Test) {
			t.Run("GN path should be relative to outDir", func(t *ftt.Test) {
				r, err := convertGnPath(ctx, gnPath, "src/path/to/out")
				if err != nil {
					t.Fatal(err)
				}
				assert.Loosely(t, r, should.Equal(filepath.Join("..", "..", "tests", "thing.txt")))
			})
		})
	})
}

func TestNormalizePath(t *testing.T) {
	t.Parallel()
	ftt.Run("Normalize path", t, func(t *ftt.Test) {
		dir := "test/dir/"
		p := "/path/to/file.txt"

		t.Run("Joining path", func(t *ftt.Test) {
			t.Run("The path should be cleaned and joined", func(t *ftt.Test) {
				assert.Loosely(t, normalizePath(dir, p), should.Equal(filepath.Join("test", "dir", "path", "to", "file.txt")))
			})
		})
	})
}

func TestInjectDetails(t *testing.T) {
	t.Parallel()
	ftt.Run("Inject unit details", t, func(t *ftt.Test) {
		ctx := context.Background()
		buildConfig := "testconfig"

		t.Run("BuildDetails not found in unitProto", func(t *ftt.Test) {
			unitProto := &kpb.CompilationUnit{}
			injectUnitBuildDetails(ctx, unitProto, buildConfig)

			t.Run("New details should be added to unitProto", func(t *ftt.Test) {
				assert.Loosely(t, len(unitProto.GetDetails()), should.Equal(1))

				any := unitProto.GetDetails()[0]
				assert.Loosely(t, any.GetTypeUrl(), should.Equal("kythe.io/proto/kythe.proto.BuildDetails"))

				buildDetails := &kpb.BuildDetails{}
				proto.Unmarshal(any.GetValue(), buildDetails)
				assert.Loosely(t, buildDetails.GetBuildConfig(), should.Equal(buildConfig))
			})
		})

		t.Run("BuildDetails found in unitProto", func(t *ftt.Test) {
			// unitProto details setup
			unitProto := &kpb.CompilationUnit{}
			details := &kpb.BuildDetails{}
			details.BuildConfig = ""
			anyDetails, err := ptypes.MarshalAny(details)
			if err != nil {
				t.Fatal(err)
			}

			anyDetails.TypeUrl = "kythe.io/proto/kythe.proto.BuildDetails"
			unitProto.Details = append(unitProto.Details, anyDetails)
			injectUnitBuildDetails(ctx, unitProto, buildConfig)

			t.Run("Any details should modified in unitProto", func(t *ftt.Test) {
				assert.Loosely(t, len(unitProto.GetDetails()), should.Equal(1))

				any := unitProto.GetDetails()[0]
				assert.Loosely(t, any.GetTypeUrl(), should.Equal("kythe.io/proto/kythe.proto.BuildDetails"))

				buildDetails := &kpb.BuildDetails{}
				proto.Unmarshal(any.GetValue(), buildDetails)
				assert.Loosely(t, buildDetails.GetBuildConfig(), should.Equal(buildConfig))
			})
		})
	})
}

func TestFindImports(t *testing.T) {
	t.Parallel()
	ftt.Run("Find imports", t, func(t *ftt.Test) {
		ctx := context.Background()
		re := regexp.MustCompile(`(?m)^\s*import\s*(?:weak|public)?\s*"([^"]*)\s*";`)

		// File setup
		tmpdir := t.TempDir()
		importPaths := []string{tmpdir}

		f, err := os.Create(tmpdir + "/test.proto")
		assert.Loosely(t, err, should.BeNil)
		defer f.Close()

		f.WriteString("import \"foo.proto\";\nimport weak \"bar.proto\";\n")
		fpath, err := filepath.Abs(f.Name())
		assert.Loosely(t, err, should.BeNil)

		t.Run("File doesn't exist", func(t *ftt.Test) {
			p := "/path/to/no/file"
			t.Run("Imports should be empty", func(t *ftt.Test) {
				assert.Loosely(t, len(findImports(ctx, re, p, importPaths)), should.BeZero)
			})
		})

		t.Run("Import file doesn't exist", func(t *ftt.Test) {
			t.Run("Imports should be empty", func(t *ftt.Test) {
				assert.Loosely(t, len(findImports(ctx, re, fpath, importPaths)), should.BeZero)
			})
		})

		t.Run("Import file exists", func(t *ftt.Test) {
			// Create import files
			fooPath := filepath.Join(tmpdir, "foo.proto")
			foo, err := os.Create(fooPath)
			if err != nil {
				t.Fatal(err)
			}
			defer foo.Close()

			barPath := filepath.Join(tmpdir, "bar.proto")
			bar, err := os.Create(barPath)
			if err != nil {
				t.Fatal(err)
			}
			defer bar.Close()

			t.Run("Should return two imports", func(t *ftt.Test) {
				r := findImports(ctx, re, fpath, importPaths)
				assert.Loosely(t, len(r), should.Equal(2))
				assert.Loosely(t, r, should.Contain(fooPath))
				assert.Loosely(t, r, should.Contain(barPath))
			})
		})
	})
}

func TestSetVname(t *testing.T) {
	t.Parallel()
	ftt.Run("Inject unit details", t, func(t *ftt.Test) {
		var vnameProto kpb.VName
		vnameProtoRoot := "root"
		vnameProto.Root = vnameProtoRoot

		defaultCorpus := "corpus"

		t.Run("Bad filepath", func(t *ftt.Test) {
			p := "\\bad\\path"

			assert.Loosely(t, func() {
				setVnameForFile(&vnameProto, p, defaultCorpus)
			}, should.Panic)
		})

		t.Cleanup(func() {
			*projectFlag = "chromium"
		})

		t.Run("With Chromium", func(t *ftt.Test) {
			*projectFlag = "chromium"

			t.Run("Filepath has special corpus", func(t *ftt.Test) {
				p := "third_party/depot_tools/win_toolchain/rest/of/path"
				setVnameForFile(&vnameProto, p, defaultCorpus)

				t.Run("Should modify vnameProto with special/external settings", func(t *ftt.Test) {
					assert.Loosely(t, vnameProto.Path, should.Equal("rest/of/path"))
					assert.Loosely(t, vnameProto.Root, should.Equal("third_party/depot_tools/win_toolchain"))
				})
			})

			t.Run("Filepath has no special corpus with src prefix", func(t *ftt.Test) {
				p := "src/build/rest/of/path"
				setVnameForFile(&vnameProto, p, defaultCorpus)

				t.Run("Should strip src prefix", func(t *ftt.Test) {
					assert.Loosely(t, vnameProto.Path, should.Equal("build/rest/of/path"))
					assert.Loosely(t, vnameProto.Root, should.Equal(vnameProtoRoot))
					assert.Loosely(t, vnameProto.Corpus, should.Equal(defaultCorpus))
				})
			})

			t.Run("Filepath has no special corpus without src prefix", func(t *ftt.Test) {
				p := "foo/build/rest/of/path"
				setVnameForFile(&vnameProto, p, defaultCorpus)

				t.Run("Should not modify path", func(t *ftt.Test) {
					assert.Loosely(t, vnameProto.Path, should.Equal(p))
					assert.Loosely(t, vnameProto.Root, should.Equal(vnameProtoRoot))
					assert.Loosely(t, vnameProto.Corpus, should.Equal(defaultCorpus))
				})
			})
			t.Run("Root is set for out/ generated files", func(t *ftt.Test) {
				p := "out/Debug/rest/of/path"
				setVnameForFile(&vnameProto, p, defaultCorpus)

				t.Run("root should be out", func(t *ftt.Test) {
					assert.Loosely(t, vnameProto.Path, should.Equal("Debug/rest/of/path"))
					assert.Loosely(t, vnameProto.Root, should.Equal("out"))
					assert.Loosely(t, vnameProto.Corpus, should.Equal(defaultCorpus))
				})
			})

		})

		t.Run("With Chrome", func(t *ftt.Test) {
			*projectFlag = "chrome"

			t.Run("Filepath has special corpus", func(t *ftt.Test) {
				p := "third_party/depot_tools/win_toolchain/rest/of/path"
				setVnameForFile(&vnameProto, p, defaultCorpus)

				t.Run("Should modify vnameProto with special/external settings", func(t *ftt.Test) {
					assert.Loosely(t, vnameProto.Path, should.Equal("rest/of/path"))
					assert.Loosely(t, vnameProto.Root, should.Equal("third_party/depot_tools/win_toolchain"))
				})
			})

			t.Run("Filepath has no special corpus with src prefix", func(t *ftt.Test) {
				p := "src/build/rest/of/path"
				setVnameForFile(&vnameProto, p, defaultCorpus)

				t.Run("Should strip src prefix", func(t *ftt.Test) {
					assert.Loosely(t, vnameProto.Path, should.Equal("build/rest/of/path"))
					assert.Loosely(t, vnameProto.Root, should.Equal(vnameProtoRoot))
					assert.Loosely(t, vnameProto.Corpus, should.Equal(defaultCorpus))
				})
			})

			t.Run("Filepath has no special corpus without src prefix", func(t *ftt.Test) {
				p := "foo/build/rest/of/path"
				setVnameForFile(&vnameProto, p, defaultCorpus)

				t.Run("Should not modify path", func(t *ftt.Test) {
					assert.Loosely(t, vnameProto.Path, should.Equal(p))
					assert.Loosely(t, vnameProto.Root, should.Equal(vnameProtoRoot))
					assert.Loosely(t, vnameProto.Corpus, should.Equal(defaultCorpus))
				})
			})

			t.Run("Root is set for out/ generated files", func(t *ftt.Test) {
				p := "out/Debug/rest/of/path"
				setVnameForFile(&vnameProto, p, defaultCorpus)

				t.Run("root should be out", func(t *ftt.Test) {
					assert.Loosely(t, vnameProto.Path, should.Equal("Debug/rest/of/path"))
					assert.Loosely(t, vnameProto.Root, should.Equal("out"))
					assert.Loosely(t, vnameProto.Corpus, should.Equal(defaultCorpus))
				})
			})
		})

		for _, proj := range []string{"chromiumos", "chromeos"} {
			*projectFlag = proj
			t.Run(fmt.Sprintf("With %s", proj), func(t *ftt.Test) {
				t.Run("Filepath has chroot prefix", func(t *ftt.Test) {
					p := "../../../../cache/cros_chroot/chroot/build/amd64-generic/rest/of/path"
					setVnameForFile(&vnameProto, p, defaultCorpus)

					t.Run("Should prefix vnameProto with path to gen files", func(t *ftt.Test) {
						assert.Loosely(t, vnameProto.Path, should.Equal("gen/amd64-generic/chroot/build/amd64-generic/rest/of/path"))
					})
				})

				t.Run("Filepath has out dir prefix", func(t *ftt.Test) {
					p := "src/out/amd64-generic/rest/of/path"
					setVnameForFile(&vnameProto, p, defaultCorpus)

					t.Run("Should prefix vnameProto with path to gen files", func(t *ftt.Test) {
						assert.Loosely(t, vnameProto.Path, should.Equal("gen/amd64-generic/src/out/amd64-generic/rest/of/path"))
					})
				})

				t.Run("Filepath has no special corpus with src prefix", func(t *ftt.Test) {
					p := "src/build/rest/of/path"
					setVnameForFile(&vnameProto, p, defaultCorpus)

					t.Run("Should not modify path", func(t *ftt.Test) {
						assert.Loosely(t, vnameProto.Path, should.Equal(p))
						assert.Loosely(t, vnameProto.Root, should.Equal(vnameProtoRoot))
						assert.Loosely(t, vnameProto.Corpus, should.Equal(defaultCorpus))
					})
				})

				t.Run("Filepath has no special corpus without src prefix", func(t *ftt.Test) {
					p := "foo/build/rest/of/path"
					setVnameForFile(&vnameProto, p, defaultCorpus)

					t.Run("Should not modify path", func(t *ftt.Test) {
						assert.Loosely(t, vnameProto.Path, should.Equal(p))
						assert.Loosely(t, vnameProto.Root, should.Equal(vnameProtoRoot))
						assert.Loosely(t, vnameProto.Corpus, should.Equal(defaultCorpus))
					})
				})
			})

		}
	})
}

func TestUnwantedWinArg(t *testing.T) {
	t.Parallel()
	ftt.Run("Win args", t, func(t *ftt.Test) {

		t.Run("Wanted arg", func(t *ftt.Test) {
			arg := "-O"

			t.Run("Arg should be included", func(t *ftt.Test) {
				assert.Loosely(t, isUnwantedWinArg(arg), should.Equal(false))
			})
		})

		t.Run("Unwanted arg", func(t *ftt.Test) {
			arg := "-DSK_USER_CONFIG_HEADER"

			t.Run("Arg shouldn't be included", func(t *ftt.Test) {
				assert.Loosely(t, isUnwantedWinArg(arg), should.Equal(true))
			})
		})
	})
}
