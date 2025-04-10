// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build windows
// +build windows

package main

import (
	"archive/zip"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "github.com/golang/protobuf/proto"
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

func TestPackageIndexWindows(t *testing.T) {
	// We can't run those tests in parallel without significant refactoring
	// of the tests as they reuse many structs among test runs.
	// t.Parallel()

	ftt.Run("Package index windows", t, func(t *ftt.Test) {
		// Setup.
		chanSize := 10
		numRoutines := 2
		tmpdir, err := ioutil.TempDir("", "")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpdir)

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
		gnPath := filepath.Join(rootDir, "src", "out", "Debug", "gn_targets.json")
		outputPath := filepath.Join(tmpdir, "out.kzip")
		if err != nil {
			t.Fatal(err)
		}

		// Since most development is done in linux, windows tests are runnable
		// in both linux and windows. However, win32 treats \ on the command line
		// very differently, so fix the commands file so it can be tested in windows.
		// * windows treats \'s differently than shlex.  (see
		//   https://docs.microsoft.com/en-us/windows/win32/api/shellapi/
		//   nf-shellapi-commandlinetoargvw#remarks) so, we need to halve the
		//   \'s for windows runs (assuming no in/out of quotes)
		// * Json needs \ escaping, so replacing 2 \'s with 1 \ requires replacing
		//   4 \'s with 2 \'s.
		origCompDB, err := ioutil.ReadFile(filepath.Join(rootDir, "src", "out", "Debug", "compile_commands_win.json"))
		if err != nil {
			t.Fatal(err)
		}
		modCompDBContents := regexp.MustCompile(`(?m)\\\\\\\\`).ReplaceAll(origCompDB, []byte("\\\\"))
		modCompDBPath := filepath.Join(tmpdir, "compile_commands_win_mod.json")
		err = ioutil.WriteFile(modCompDBPath, modCompDBContents, 0444)
		if err != nil {
			t.Fatal(err)
		}

		ip := newIndexPack(ctx, outputPath, rootDir, "src/out/Debug", modCompDBPath, gnPath, kzipPath,
			"chromium-test", "win", "")

		// Read expected units and place into a map.
		unitMap := make(map[unitKey]string)
		units, err := ioutil.ReadDir(filepath.Join(testDir, "units_win.expected"))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range units {
			content, err := ioutil.ReadFile(filepath.Join(testDir, "units_win.expected", f.Name()))
			if err != nil {
				t.Fatal(err)
			}
			indexedCompilation := &kpb.IndexedCompilation{}
			err = v1.UnmarshalText(string(content), indexedCompilation)
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
			clangTargets := NewClangTargets(modCompDBPath)
			clangTargets.DataWg.Add(numRoutines)
			clangTargets.UnitWg.Add(numRoutines)
			clangTargets.KzipDataWg.Add(numRoutines)
			for i := 0; i < numRoutines; i++ {
				go func() {
					// Process clang files.
					err := clangTargets.ProcessClangTargets(ip.ctx, ip.rootPath, ip.outDir, ip.corpus,
						ip.buildConfig, ip.clangTargetArch, ip.hashMaps, dataFileChannel, unitProtoChannel)
					if err != nil {
						// See b:227367175 for context.
						panic(err.Error())
					}
				}()
			}

			// Parse GN targets.
			gnTargets := NewGnTargets(gnPath)
			gnTargets.DataWg.Add(numRoutines)
			gnTargets.KzipDataWg.Add(numRoutines)
			gnTargets.UnitWg.Add(numRoutines)

			// Process GN target data files.
			for i := 0; i < numRoutines; i++ {
				go func() {
					// Process GN files.
					err := gnTargets.ProcessGnTargets(ip.ctx, ip.rootPath, ip.outDir, ip.corpus, ip.buildConfig, ip.hashMaps,
						dataFileChannel, unitProtoChannel)
					if err != nil {
						// See b:227367175 for context.
						panic(err.Error())
					}
				}()
			}

			// Convert data files to kzipEntries.
			for i := 0; i < numRoutines; i++ {
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
			for i := 0; i < numRoutines; i++ {
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
					// See b:227367175 for context.
					panic(err.Error())
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
				files, _ := ioutil.ReadDir(filepath.Join(testDir, "files.expected"))
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
					dataContentExpected, err := ioutil.ReadFile(
						filepath.Join(testDir, "files.expected", name))
					if err != nil {
						t.Fatal(err)
					}

					assert.Loosely(t, dataContentOut, should.Resemble(dataContentExpected))
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

					assert.Loosely(t, unitOut.String(), should.Resemble(
						unitMap[unitKey{unitOut.GetVName().GetCorpus(), unitOut.GetSourceFile()[0]}]))
				}
			})
		})
	})
}
