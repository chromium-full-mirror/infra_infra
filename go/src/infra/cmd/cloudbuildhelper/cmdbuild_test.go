// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/cmd/cloudbuildhelper/cloudbuild"
	"go.chromium.org/infra/cmd/cloudbuildhelper/fileset"
	"go.chromium.org/infra/cmd/cloudbuildhelper/manifest"
)

const (
	testTargetName   = "test-name"
	testBucketName   = "test-bucket"
	testRegistryName = "fake.example.com/registry"
	testDigest       = "sha256:totally-legit-hash"
	testTagName      = "canonical-tag"
	testLogURL       = "https://example.com/cloud-build-log"

	testImageName = testRegistryName + "/" + testTargetName
)

var _true = true // for *bool

var testBaseOutput = &baseOutput{
	Name:    testTargetName,
	Sources: []string{"a", "b"},
	Notify: []manifest.NotifyConfig{
		{Kind: "git", Repo: "https://a.example.com", Script: "a"},
		{Kind: "git", Repo: "https://b.example.com", Script: "b"},
	},
}

func TestBuild(t *testing.T) {
	t.Parallel()

	ftt.Run("With mocks", t, func(t *ftt.Test) {
		testTime := time.Date(2016, time.February, 3, 4, 5, 6, 0, time.Local)
		ctx, tc := testclock.UseTime(context.Background(), testTime)
		tc.SetTimerCallback(func(d time.Duration, t clock.Timer) {
			if testclock.HasTags(t, "sleep-timer") {
				tc.Add(d)
			}
		})
		ctx, _ = clock.WithTimeout(ctx, 20*time.Minute) // don't hang forever

		store := newStorageImplMock()
		registry := newRegistryImplMock()
		builder := newBuilderImplMock(registry)
		fs, digest := prepFileSet()

		var (
			// Path relative to the storage root.
			testTarballPath = fmt.Sprintf("%s/%s.tar.gz", testTargetName, digest)
			// Where we drops the tarball, excluding "#<generation>" suffix.
			testTarballURL = fmt.Sprintf("gs://%s/%s/%s.tar.gz", testBucketName, testTargetName, digest)
		)

		builder.provenance = func(gs string) string {
			assert.Loosely(t, gs, should.Equal(testTarballURL+"#1")) // used first gen
			return digest                                            // got its digest correctly
		}
		builder.outputDigests = func(img string) string {
			assert.Loosely(t, img, should.HavePrefix(testImageName+":mocked-"))
			return testDigest
		}

		t.Run("Never seen before tarball", func(t *ftt.Test) {
			res, err := runBuild(ctx, buildParams{
				Manifest: &manifest.Manifest{
					Name:          testTargetName,
					Deterministic: &_true,
				},
				Image:        testImageName,
				BuildID:      "b1",
				CanonicalTag: testTagName,
				Tags:         []string{"latest"},
				Stage:        stageFileSet(fs),
				Store:        store,
				Builder:      builder,
				Registry:     registry,
				Output:       testBaseOutput,
			})
			assert.Loosely(t, err, should.BeNil)

			// Uploaded the file.
			obj, err := store.Check(ctx, testTarballPath)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, obj.String(), should.Equal(testTarballURL+"#1")) // uploaded the first gen

			// Used Cloud Build.
			assert.Loosely(t, res, should.Resemble(buildResult{
				baseOutput: testBaseOutput,
				Image: &imageRef{
					Image:        testImageName,
					Digest:       testDigest,
					CanonicalTag: testTagName,
					BuildID:      "b1",
				},
				ViewBuildURL: testLogURL,
			}))

			// Tagged it with canonical tag.
			img, err := registry.GetImage(ctx, fmt.Sprintf("%s:%s", testImageName, testTagName))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, img.Digest, should.Equal(testDigest))

			// And moved "latest" tag.
			img, err = registry.GetImage(ctx, testImageName+":latest")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, img.Digest, should.Equal(testDigest))

			// Now we build this exact tarball again using different canonical tag.
			// We should get back the image we've already built.
			t.Run("Building existing tarball deterministically: reuses the image", func(t *ftt.Test) {
				builder.provenance = func(gs string) string {
					panic("Cloud Build should not be invoked")
				}

				// To avoid clashing on metadata keys that depend on timestamps.
				tc.Add(time.Minute)

				res, err := runBuild(ctx, buildParams{
					Manifest: &manifest.Manifest{
						Name:          testTargetName,
						Deterministic: &_true,
					},
					Image:        testImageName,
					BuildID:      "b2",
					CanonicalTag: "another-tag",
					Tags:         []string{"pushed"},
					Stage:        stageFileSet(fs),
					Store:        store,
					Builder:      builder,
					Registry:     registry,
					Output:       testBaseOutput,
				})
				assert.Loosely(t, err, should.BeNil)

				// Reused the existing image.
				assert.Loosely(t, res, should.Resemble(buildResult{
					baseOutput: testBaseOutput,
					Image: &imageRef{
						Image:        testImageName,
						Digest:       testDigest,
						CanonicalTag: testTagName,
						BuildID:      "b1", // was build there
						Timestamp:    testTime.Add(10 * time.Second),
					},
				}))

				// And moved "pushed" tag, even through no new image was built.
				img, err := registry.GetImage(ctx, testImageName+":pushed")
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, img.Digest, should.Equal(testDigest))

				// Both builds are associated with the tarball via its metadata now.
				tarball, err := store.Check(ctx, testTarballPath)
				assert.Loosely(t, err, should.BeNil)
				md := tarball.Metadata.Values(buildRefMetaKey)
				assert.Loosely(t, md, should.HaveLength(2))
				assert.Loosely(t, md[0].Value, should.Equal(`{"build_id":"b2","tag":"another-tag"}`))
				assert.Loosely(t, md[1].Value, should.Equal(`{"build_id":"b1","tag":"canonical-tag"}`))
			})

			// Now we build this exact tarball again using different canonical tag,
			// but mark the target as non-deterministic. It should ignore the existing
			// image and build a new one.
			t.Run("Building existing tarball non-deterministically: creates new image", func(t *ftt.Test) {
				builder.outputDigests = func(img string) string {
					assert.Loosely(t, img, should.HavePrefix(testImageName+":mocked-"))
					return "sha256:new-totally-legit-hash"
				}

				// To avoid clashing on metadata keys that depend on timestamps.
				tc.Add(time.Minute)

				res, err := runBuild(ctx, buildParams{
					Manifest: &manifest.Manifest{
						Name:          testTargetName,
						Deterministic: nil,
					},
					Image:        testImageName,
					BuildID:      "b2",
					CanonicalTag: "another-tag",
					Tags:         []string{"latest"},
					Stage:        stageFileSet(fs),
					Store:        store,
					Builder:      builder,
					Registry:     registry,
					Output:       testBaseOutput,
				})
				assert.Loosely(t, err, should.BeNil)

				// Built the new image.
				assert.Loosely(t, res, should.Resemble(buildResult{
					baseOutput: testBaseOutput,
					Image: &imageRef{
						Image:        testImageName,
						Digest:       "sha256:new-totally-legit-hash",
						CanonicalTag: "another-tag",
						BuildID:      "b2",
					},
					ViewBuildURL: testLogURL,
				}))

				// And moved "latest" tag.
				img, err = registry.GetImage(ctx, testImageName+":latest")
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, img.Digest, should.Equal("sha256:new-totally-legit-hash"))

				// Both builds are associated with the tarball via its metadata now.
				tarball, err := store.Check(ctx, testTarballPath)
				assert.Loosely(t, err, should.BeNil)
				md := tarball.Metadata.Values(buildRefMetaKey)
				assert.Loosely(t, md, should.HaveLength(2))
				assert.Loosely(t, md[0].Value, should.Equal(`{"build_id":"b2","tag":"another-tag"}`))
				assert.Loosely(t, md[1].Value, should.Equal(`{"build_id":"b1","tag":"canonical-tag"}`))
			})
		})

		t.Run("Building with PushesExplicitly==true builder", func(t *ftt.Test) {
			builder.pushesExplicitly = true

			res, err := runBuild(ctx, buildParams{
				Manifest: &manifest.Manifest{
					Name:          testTargetName,
					Deterministic: &_true,
				},
				Image:        testImageName,
				BuildID:      "b1",
				CanonicalTag: testTagName,
				Tags:         []string{"latest"},
				Stage:        stageFileSet(fs),
				Store:        store,
				Builder:      builder,
				Registry:     registry,
				Output:       testBaseOutput,
			})
			assert.Loosely(t, err, should.BeNil)

			// Uploaded the file.
			obj, err := store.Check(ctx, testTarballPath)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, obj.String(), should.Equal(testTarballURL+"#1")) // uploaded the first gen

			// Used Cloud Build.
			assert.Loosely(t, res, should.Resemble(buildResult{
				baseOutput: testBaseOutput,
				Image: &imageRef{
					Image:        testImageName,
					Digest:       testDigest,
					CanonicalTag: testTagName,
					BuildID:      "b1",
				},
				ViewBuildURL: testLogURL,
			}))

			// Tagged it with canonical tag.
			img, err := registry.GetImage(ctx, fmt.Sprintf("%s:%s", testImageName, testTagName))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, img.Digest, should.Equal(testDigest))

			// And moved "latest" tag.
			img, err = registry.GetImage(ctx, testImageName+":latest")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, img.Digest, should.Equal(testDigest))
		})

		t.Run("Already seen canonical tag", func(t *ftt.Test) {
			registry.put(fmt.Sprintf("%s:%s", testImageName, testTagName), testDigest)

			res, err := runBuild(ctx, buildParams{
				Manifest:     &manifest.Manifest{Name: testTargetName},
				Image:        testImageName,
				CanonicalTag: testTagName,
				Tags:         []string{"latest"},
				Registry:     registry,
				Output:       testBaseOutput,
			})
			assert.Loosely(t, err, should.BeNil)

			// Reused the existing image.
			assert.Loosely(t, res, should.Resemble(buildResult{
				baseOutput: testBaseOutput,
				Image: &imageRef{
					Image:        testImageName,
					Digest:       testDigest,
					CanonicalTag: testTagName,
				},
			}))

			// And moved "latest" tag.
			img, err := registry.GetImage(ctx, testImageName+":latest")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, img.Digest, should.Equal(testDigest))
		})

		t.Run("Using :inputs-hash as canonical tag", func(t *ftt.Test) {
			expectedTag := "cbh-inputs-" + digest[:24]

			params := buildParams{
				Manifest: &manifest.Manifest{
					Name:          testTargetName,
					Deterministic: &_true,
				},
				Image:        testImageName,
				BuildID:      "b1",
				CanonicalTag: inputsHashCanonicalTag,
				Tags:         []string{"latest"},
				Stage:        stageFileSet(fs),
				Store:        store,
				Builder:      builder,
				Registry:     registry,
				Output:       testBaseOutput,
			}
			res, err := runBuild(ctx, params)
			assert.Loosely(t, err, should.BeNil)

			// Uploaded the file.
			obj, err := store.Check(ctx, testTarballPath)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, obj.String(), should.Equal(testTarballURL+"#1")) // uploaded the first gen

			// Used Cloud Build.
			assert.Loosely(t, res, should.Resemble(buildResult{
				baseOutput: testBaseOutput,
				Image: &imageRef{
					Image:        testImageName,
					Digest:       testDigest,
					CanonicalTag: expectedTag,
					BuildID:      "b1",
				},
				ViewBuildURL: testLogURL,
			}))

			// Tagged it with canonical tag.
			img, err := registry.GetImage(ctx, fmt.Sprintf("%s:%s", testImageName, expectedTag))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, img.Digest, should.Equal(testDigest))

			// Repeating the build reuses the existing image since inputs hash didn't
			// change (and thus its canonical tag also didn't change and we already
			// have an image with this canonical tag).
			res, err = runBuild(ctx, params)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.Resemble(buildResult{
				baseOutput: testBaseOutput,
				Image: &imageRef{
					Image:        testImageName,
					Digest:       testDigest,
					CanonicalTag: expectedTag,
				},
				ViewBuildURL: "", // Cloud Build wasn't used
			}))
		})

		t.Run("Cloud Build build failure", func(t *ftt.Test) {
			builder.finalStatus = cloudbuild.StatusFailure
			_, err := runBuild(ctx, buildParams{
				Manifest: &manifest.Manifest{Name: testTargetName},
				Image:    testImageName,
				Stage:    stageFileSet(fs),
				Store:    store,
				Builder:  builder,
				Registry: registry,
			})
			assert.Loosely(t, err, should.ErrLike("build failed, see its logs"))
		})

		t.Run("Cloud Build API errors", func(t *ftt.Test) {
			builder.checkCallback = func(b *runningBuild) error {
				return fmt.Errorf("boom")
			}
			_, err := runBuild(ctx, buildParams{
				Manifest: &manifest.Manifest{Name: testTargetName},
				Image:    testImageName,
				Stage:    stageFileSet(fs),
				Store:    store,
				Builder:  builder,
				Registry: registry,
			})
			assert.Loosely(t, err, should.ErrLike("waiting for the build to finish: too many errors, the last one: boom"))
		})
	})
}

////////////////////////////////////////////////////////////////////////////////

func prepFileSet() (fs *fileset.Set, digest string) {
	fs = &fileset.Set{}

	fs.AddFromMemory("Dockerfile", []byte("boo-boo-boo"), nil)
	fs.AddFromMemory("dir/something", []byte("ba-ba-ba"), nil)

	h := sha256.New()
	if err := fs.ToTarGz(h); err != nil {
		panic(err)
	}
	digest = hex.EncodeToString(h.Sum(nil))
	return
}

func stageFileSet(fs *fileset.Set) stageCallback {
	return func(c context.Context, m *manifest.Manifest, cb func(*fileset.Set) error) error {
		return cb(fs)
	}
}
