// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build !windows
// +build !windows

package main

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	kpb "go.chromium.org/infra/cmd/package_index/kythe/proto"
)

// unitKey is used to identify a unique compilation unit for testing.
type unitKey struct {
	corpus     string
	sourceFile string
}

func TestPackageIndexUnix(t *testing.T) {
	// We can't run those tests in parallel without significant refactoring
	// of the tests as they reuse many structs among test runs.
	// t.Parallel()

	ftt.Run("Package index unix", t, func(t *ftt.Test) {
		// Setup.
		chanSize := 10
		numRoutines := 2
		tmpdir := t.TempDir()

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}

		// Initialize indexPack.
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
		defer cancel()
		testDir := filepath.Join(cwd, "package_index_testdata")
		rootDir := filepath.Join(testDir, "input")
		kzipPath := filepath.Join(rootDir, "src", "out", "Debug", "kzip")
		compDBPath := filepath.Join(rootDir, "src", "out", "Debug", "compile_commands.json")
		gnPath := filepath.Join(rootDir, "src", "out", "Debug", "gn_targets.json")
		outputPath := filepath.Join(tmpdir, "out.kzip")
		if err != nil {
			t.Fatal(err)
		}
		ip := newIndexPack(ctx, outputPath, rootDir, "src/out/Debug", compDBPath, gnPath, kzipPath,
			"chromium-test", "linux", "")

		// Read expected units and place into a map.
		unitMap := make(map[unitKey]string)
		units, err := os.ReadDir(filepath.Join(testDir, "units.expected"))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range units {
			content, err := os.ReadFile(filepath.Join(testDir, "units.expected", f.Name()))
			if err != nil {
				t.Fatal(err)
			}
			indexedCompilation := &kpb.IndexedCompilation{}
			err = prototext.Unmarshal(content, indexedCompilation)
			if err != nil {
				t.Fatal(err)
			}
			unit := indexedCompilation.GetUnit()
			unitMap[unitKey{unit.GetVName().GetCorpus(), unit.GetSourceFile()[0]}] = unit.String()
		}

		// Set new.kzip modified time to after old_duplicate.kzip for testing.
		oldkzip, err := os.Stat(filepath.Join(kzipPath, "old_duplicate.kzip"))
		if err != nil {
			t.Fatal(err)
		}
		newModTime := oldkzip.ModTime().Add(time.Second)
		err = os.Chtimes(filepath.Join(kzipPath, "new.kzip"), newModTime, newModTime)
		if err != nil {
			t.Fatal(err)
		}

		t.Run("Parse and process GN/clang targets", func(t *ftt.Test) {
			// Parse existing kzips.
			existingKzipChannel := make(chan string, chanSize)
			go func() {
				err := ip.mergeExistingKzips(existingKzipChannel)
				if err != nil {
					panic(err)
				}
			}()

			// Process existing kzips.
			var kzipWg sync.WaitGroup
			kzipEntryChannel := make(chan kzipEntry, chanSize)
			kzipSet := NewConcurrentSet(0)
			kzipWg.Add(1)
			go func() {
				err := ip.processExistingKzips(ctx, existingKzipChannel, kzipEntryChannel, kzipSet)
				if err != nil {
					panic(err)
				}
				kzipWg.Done()
			}()

			// Parse and process targets.
			unitProtoChannel := make(chan *kpb.CompilationUnit, chanSize)
			dataFileChannel := make(chan string, chanSize)

			// Parse compdb.
			clangTargets := NewClangTargets(compDBPath)
			clangTargets.DataWg.Add(numRoutines)
			clangTargets.UnitWg.Add(numRoutines)
			clangTargets.KzipDataWg.Add(numRoutines)
			for range numRoutines {
				go func() {
					// Process clang files.
					err := clangTargets.ProcessClangTargets(ip.ctx, ip.rootPath, ip.outDir, ip.corpus,
						ip.buildConfig, "", ip.hashMaps, dataFileChannel, unitProtoChannel)
					if err != nil {
						t.Error(err)
					}
				}()
			}

			// Parse GN targets.
			gnTargets := NewGnTargets(gnPath)
			gnTargets.DataWg.Add(numRoutines)
			gnTargets.KzipDataWg.Add(numRoutines)
			gnTargets.UnitWg.Add(numRoutines)

			// Process GN target data files.
			for range numRoutines {
				go func() {
					// Process GN files.
					err := gnTargets.ProcessGnTargets(ip.ctx, ip.rootPath, ip.outDir, ip.corpus, ip.buildConfig, ip.hashMaps,
						dataFileChannel, unitProtoChannel)
					if err != nil {
						t.Error(err)
					}
				}()
			}

			// Convert data files to kzipEntries.
			for range numRoutines {
				go func() {
					ip.dataFileToKzipEntry(ctx, dataFileChannel, kzipEntryChannel)

					// Signal done for GN compilation unit processing.
					gnTargets.KzipDataWg.Done()
					clangTargets.KzipDataWg.Done()
				}()
			}

			// Convert unit protos to kzipEntries.
			var kzipUnitWg sync.WaitGroup
			kzipUnitWg.Add(numRoutines)
			for range numRoutines {
				go func() {
					ip.unitFileToKzipEntry(ctx, unitProtoChannel, kzipEntryChannel)
					kzipUnitWg.Done()
				}()
			}

			// Close dataFileChannel and unitProtoChannel after all GN targets have been processed and sent.
			go func() {
				gnTargets.DataWg.Wait()
				clangTargets.DataWg.Wait()
				close(dataFileChannel)
			}()
			go func() {
				gnTargets.UnitWg.Wait()
				clangTargets.UnitWg.Wait()
				close(unitProtoChannel)
			}()

			// Close kzipEntryChannel after all entries have been sent.
			go func() {
				kzipUnitWg.Wait()
				kzipWg.Wait()
				close(kzipEntryChannel)
			}()

			// Wait for kzip writes to finish.
			var writeWg sync.WaitGroup
			writeWg.Add(1)
			go func() {
				err := ip.writeToKzip(kzipEntryChannel)
				if err != nil {
					t.Error(err)
				}
				writeWg.Done()
			}()
			writeWg.Wait()

			t.Run("Kzip contains files and units for GN/clang targets and existing kzips", func(t *ftt.Test) {
				// Check kzip exists.
				_, err = os.Stat(outputPath)
				assert.Loosely(t, err, should.BeNil)

				r, err := zip.OpenReader(outputPath)
				if err != nil {
					t.Fatal(err)
				}
				defer r.Close()

				// Get units and files.
				var unitInfo []*zip.File
				var dataInfo []*zip.File
				for _, zipInfo := range r.File {
					assert.Loosely(t, strings.Contains(zipInfo.Name, "\\"), should.BeFalse)
					if strings.Contains(zipInfo.Name, "pbunits") && zipInfo.Name != "root/pbunits/" {
						unitInfo = append(unitInfo, zipInfo)
					} else if strings.Contains(zipInfo.Name, "files") && zipInfo.Name != "root/files/" {
						dataInfo = append(dataInfo, zipInfo)
					}
				}

				// Check generated data files match expected.
				files, _ := os.ReadDir(filepath.Join(testDir, "files.expected"))
				assert.Loosely(t, len(dataInfo), should.Equal(len(files)))

				for _, file := range dataInfo {
					rcData, err := file.Open()
					if err != nil {
						t.Fatal(err)
					}
					defer rcData.Close()
					dataContentOut := make([]byte, file.UncompressedSize64)
					_, err = io.ReadFull(rcData, dataContentOut)
					if err != nil {
						t.Fatal(err)
					}

					_, name := filepath.Split(file.Name)
					dataContentExpected, err := os.ReadFile(
						filepath.Join(testDir, "files.expected", name))
					if err != nil {
						t.Fatal(err)
					}

					assert.Loosely(t, dataContentOut, should.Match(dataContentExpected))
				}

				// Check the expected number of duplicate units generated match expected.
				for lang, stats := range ip.stats {
					if lang == "protobuf" {
						// Both targets
						// - //net/cert:root_store_proto_full_gen
						// - //net/cert:root_store_proto_lite_gen
						// are distinct however evaluate to an identical CU derived from their
						// identical source proto, hence there should be one duplicate.
						assert.That(t, stats.numDuplicates, should.Equal(1))
					} else {
						// Duplicates are unexpected for other languages.
						assert.That(t, stats.numDuplicates, should.Equal(0))
					}
				}

				// Check generated unit protos match expected.
				assert.Loosely(t, len(unitInfo), should.Equal(len(unitMap)))
				for _, file := range unitInfo {
					rcUnit, err := file.Open()
					if err != nil {
						t.Fatal(err)
					}
					defer rcUnit.Close()

					unitContentOut := make([]byte, file.UncompressedSize64)
					_, err = io.ReadFull(rcUnit, unitContentOut)
					if err != nil {
						t.Fatal(err)
					}

					indexedCompilationOut := &kpb.IndexedCompilation{}
					err = proto.Unmarshal(unitContentOut, indexedCompilationOut)
					if err != nil {
						t.Fatal(err)
					}
					unitOut := indexedCompilationOut.GetUnit()

					assert.Loosely(t, unitOut.String(), should.Match(
						unitMap[unitKey{unitOut.GetVName().GetCorpus(), unitOut.GetSourceFile()[0]}]))
				}
			})
		})
	})
}
