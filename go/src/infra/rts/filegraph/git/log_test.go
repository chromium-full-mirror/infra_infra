// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package git

import (
	"bufio"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"strings"
	"testing"
)

func TestLogReader(t *testing.T) {
	ftt.Run(`LogReader`, t, func(t *ftt.Test) {

		read := func(log string) []commit {
			r := &logReader{r: bufio.NewReader(strings.NewReader(log))}
			r.sep = '|'
			var commits []commit
			err := r.ReadCommits(func(c commit) error {
				commits = append(commits, c)
				return nil
			})
			assert.Loosely(t, err, should.BeNil)
			return commits
		}

		t.Run(`one commit`, func(t *ftt.Test) {
			parseOneCommit := func(log string) commit {
				commits := read(log)
				assert.Loosely(t, commits, should.HaveLength(1))
				return commits[0]
			}

			t.Run(`M`, func(t *ftt.Test) {
				actual := parseOneCommit(`a3dcd10d73c46ea826785d03b7aa35e294d0f12a 91b7cf4d4e8b259f7657b6149e3393b166a7aaee
:100644 100644 8150c0c9b 8f04adce2 M|path/to/file||`)
				assert.Loosely(t, actual, should.Resemble(commit{
					Hash:         "a3dcd10d73c46ea826785d03b7aa35e294d0f12a",
					ParentHashes: []string{"91b7cf4d4e8b259f7657b6149e3393b166a7aaee"},
					Files: []fileChange{
						{
							Status: 'M',
							Path:   "path/to/file",
						},
					},
				}))
			})

			t.Run(`M, M`, func(t *ftt.Test) {
				actual := parseOneCommit(`a3dcd10d73c46ea826785d03b7aa35e294d0f12a 91b7cf4d4e8b259f7657b6149e3393b166a7aaee
:100644 100644 8150c0c9b 8f04adce2 M|path/to/file|:100644 100644 8150c0c9b 8f04adce2 M|path/to/file2||`)

				assert.Loosely(t, actual, should.Resemble(commit{
					Hash:         "a3dcd10d73c46ea826785d03b7aa35e294d0f12a",
					ParentHashes: []string{"91b7cf4d4e8b259f7657b6149e3393b166a7aaee"},
					Files: []fileChange{
						{
							Status: 'M',
							Path:   "path/to/file",
						},
						{
							Status: 'M',
							Path:   "path/to/file2",
						},
					},
				}))
			})

			t.Run(`C`, func(t *ftt.Test) {
				actual := parseOneCommit(`a3dcd10d73c46ea826785d03b7aa35e294d0f12a 91b7cf4d4e8b259f7657b6149e3393b166a7aaee
:100644 100644 8150c0c9b 8f04adce2 C|path/to/file|path/to/file2||`)
				assert.Loosely(t, actual, should.Resemble(commit{
					Hash:         "a3dcd10d73c46ea826785d03b7aa35e294d0f12a",
					ParentHashes: []string{"91b7cf4d4e8b259f7657b6149e3393b166a7aaee"},
					Files: []fileChange{
						{
							Status: 'C',
							Path:   "path/to/file",
							Path2:  "path/to/file2",
						},
					},
				}))
			})

			t.Run(`R50`, func(t *ftt.Test) {
				actual := parseOneCommit(`a3dcd10d73c46ea826785d03b7aa35e294d0f12a 91b7cf4d4e8b259f7657b6149e3393b166a7aaee
:100644 100644 8150c0c9b 8f04adce2 R50|path/to/file|path/to/file2||`)
				assert.Loosely(t, actual, should.Resemble(commit{
					Hash:         "a3dcd10d73c46ea826785d03b7aa35e294d0f12a",
					ParentHashes: []string{"91b7cf4d4e8b259f7657b6149e3393b166a7aaee"},
					Files: []fileChange{
						{
							Status: 'R',
							Path:   "path/to/file",
							Path2:  "path/to/file2",
						},
					},
				}))
			})

			t.Run(`two parents`, func(t *ftt.Test) {
				actual := parseOneCommit(`a3dcd10d73c46ea826785d03b7aa35e294d0f12a 91b7cf4d4e8b259f7657b6149e3393b166a7aaee 3a89a841f9d213cc273af75f085f52c2597a63d2
:100644 100644 8150c0c9b 8f04adce2 M|path/to/file||`)
				assert.Loosely(t, actual, should.Resemble(commit{
					Hash:         "a3dcd10d73c46ea826785d03b7aa35e294d0f12a",
					ParentHashes: []string{"91b7cf4d4e8b259f7657b6149e3393b166a7aaee", "3a89a841f9d213cc273af75f085f52c2597a63d2"},
					Files: []fileChange{
						{
							Status: 'M',
							Path:   "path/to/file",
						},
					},
				}))
			})

		})

		t.Run(`two commits`, func(t *ftt.Test) {
			t.Run(`M; M`, func(t *ftt.Test) {
				actual := read(`a3dcd10d73c46ea826785d03b7aa35e294d0f12a 91b7cf4d4e8b259f7657b6149e3393b166a7aaee
:100644 100644 8150c0c9b 8f04adce2 M|path/to/file||3a89a841f9d213cc273af75f085f52c2597a63d2 5376d1941a55c6481fc568004d4836ffc2c332b9
:100644 100644 8150c0c9b 8f04adce2 M|path/to/file2||`)
				assert.Loosely(t, actual, should.Resemble([]commit{
					{
						Hash:         "a3dcd10d73c46ea826785d03b7aa35e294d0f12a",
						ParentHashes: []string{"91b7cf4d4e8b259f7657b6149e3393b166a7aaee"},
						Files: []fileChange{
							{
								Status: 'M',
								Path:   "path/to/file",
							},
						},
					},
					{
						Hash:         "3a89a841f9d213cc273af75f085f52c2597a63d2",
						ParentHashes: []string{"5376d1941a55c6481fc568004d4836ffc2c332b9"},
						Files: []fileChange{
							{
								Status: 'M',
								Path:   "path/to/file2",
							},
						},
					},
				}))
			})
		})

		t.Run(`empty commit`, func(t *ftt.Test) {
			actual := read(`f71d633d037422e101edb573038b8e281f283d16 |2f1870f85324ed520077653a2c8a881e9052d96c f71d633d037422e101edb573038b8e281f283d16
:100644 100644 8150c0c9b 8f04adce2 M|path/to/file2||`)
			assert.Loosely(t, actual, should.Resemble([]commit{
				{
					Hash: "f71d633d037422e101edb573038b8e281f283d16",
				},
				{
					Hash:         "2f1870f85324ed520077653a2c8a881e9052d96c",
					ParentHashes: []string{"f71d633d037422e101edb573038b8e281f283d16"},
					Files: []fileChange{
						{
							Status: 'M',
							Path:   "path/to/file2",
						},
					},
				},
			}))
		})

		t.Run(`orphan commit`, func(t *ftt.Test) {
			// This string is split into two because some editors trim trailing
			// whitespace, even if it is a part of a string literal.
			actual := read("f71d633d037422e101edb573038b8e281f283d16 " + `
:100644 100644 8150c0c9b 8f04adce2 M|path/to/file||2f1870f85324ed520077653a2c8a881e9052d96c f71d633d037422e101edb573038b8e281f283d16
:100644 100644 8150c0c9b 8f04adce2 M|path/to/file2||`)
			assert.Loosely(t, actual, should.Resemble([]commit{
				{
					Hash: "f71d633d037422e101edb573038b8e281f283d16",
					Files: []fileChange{
						{
							Status: 'M',
							Path:   "path/to/file",
						},
					},
				},
				{
					Hash:         "2f1870f85324ed520077653a2c8a881e9052d96c",
					ParentHashes: []string{"f71d633d037422e101edb573038b8e281f283d16"},
					Files: []fileChange{
						{
							Status: 'M',
							Path:   "path/to/file2",
						},
					},
				},
			}))
		})

	})
}
