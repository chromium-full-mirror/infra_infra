// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"go.chromium.org/luci/common/data/stringset"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/gologger"

	kpb "infra/cmd/package_index/kythe/proto"
)

// Size of buffered channels.
var chanSize = 1000

// Number of goroutines to use in parallel processing.
var numRoutines = 32

var (
	projectFlag         = flag.String("project", "chromium", "Project this kzip should be generated for. This should be set to 'chromium', 'chrome', or 'chromiumos'. Defaults to 'chromium'.")
	outputFlag          = flag.String("path_to_archive_output", "", "Path to index pack archive to be generated.")
	compDBFlag          = flag.String("path_to_compdb", "", "Path to the compilation database.")
	gnFlag              = flag.String("path_to_gn_targets", "", "Path to the gn targets json file.")
	corpusFlag          = flag.String("corpus", "", "Kythe corpus to use for the vname.")
	existingKzipsFlag   = flag.String("path_to_java_kzips", "", "Path to already generated java kzips which will be included in the final index pack.")
	buildFlag           = flag.String("build_config", "", "Build config to use in the unit file.")
	clangTargetArchFlag = flag.String("clang_target_arch", "", "Target architecture to provide via -target in any clang commands.")
	checkoutFlag        = flag.String("checkout_dir", "", "Root of the repository.")
	outDirFlag          = flag.String("out_dir", "src/out/Debug", "Output directory from which compilation is run.")
	filepathsFlag       = flag.Bool("keep_filepaths_files", false, "Keep the .filepaths files used for index pack generation.")
	verboseFlag         = flag.Bool("verbose", false, "Print the details of every file being written to the index pack.")

	allowedProjects = stringset.NewFromSlice(
		"chromium",
		"chromiumos",
		"chrome",
		"chromeos")
)

// validateFlags checks that the required flags are present.
func validateFlags(ctx context.Context) {
	flagErr := false

	if !allowedProjects.Has(*projectFlag) {
		logging.Errorf(ctx, "project not supported. [%s] are the only supported projects.", strings.Join(allowedProjects.ToSortedSlice(), ", "))
		flagErr = true
	}

	if *outputFlag == "" {
		logging.Errorf(ctx, "path_to_archive_output flag required.")
		flagErr = true
	}

	if *compDBFlag == "" {
		logging.Errorf(ctx, "path_to_compdb flag required.")
		flagErr = true
	}

	if *gnFlag == "" {
		logging.Errorf(ctx, "path_to_gn_targets flag required.")
		flagErr = true
	}

	if *corpusFlag == "" {
		logging.Errorf(ctx, "corpus flag required.")
		flagErr = true
	}

	if *checkoutFlag == "" {
		logging.Errorf(ctx, "checkout_dir flag required.")
		flagErr = true
	}

	if flagErr {
		panic("Flags are missing or not supported")
	}
}

func main() {
	ctx := gologger.StdConfig.Use(context.Background())
	flag.Parse()
	validateFlags(ctx)

	// Remove the old zip archive (if it exists). This avoids the new index
	// pack being added to the old zip archive.
	if _, err := os.Stat(*outputFlag); err == nil { // Old kzip exists.
		err = os.Remove(*outputFlag)
		if err != nil {
			panic(err)
		}
	}

	// Setup.
	if *verboseFlag {
		ctx = logging.SetLevel(ctx, logging.Debug)
	}
	logging.Infof(ctx, "%s: Index generation...", time.Now().Format("15:04:05"))
	rootPath, err := filepath.Abs(filepath.Join(*checkoutFlag, ".."))
	if err != nil {
		panic(err)
	}
	ip := newIndexPack(ctx, *outputFlag, rootPath, *outDirFlag, *compDBFlag,
		*gnFlag, *existingKzipsFlag, *corpusFlag, *buildFlag, *clangTargetArchFlag)

	// Process existing kzips.
	existingKzipChannel := make(chan string, chanSize)
	go func() {
		if ip.existingJavaKzipsPath != "" {
			err := ip.mergeExistingKzips(existingKzipChannel)
			if err != nil {
				panic(err)
			}
		} else {
			close(existingKzipChannel)
		}
	}()

	var kzipEntryWg sync.WaitGroup
	kzipEntryChannel := make(chan kzipEntry, 100) // Channel size is reduced for chromiumos builder.
	kzipSet := NewConcurrentSet(0)
	kzipEntryWg.Add(1)
	go func() {
		err := ip.processExistingKzips(ctx, existingKzipChannel, kzipEntryChannel, kzipSet)
		if err != nil {
			panic(err)
		}
		kzipEntryWg.Done()
	}()

	// Process targets.
	unitProtoChannel := make(chan *kpb.CompilationUnit, chanSize)
	dataFileChannel := make(chan string, chanSize)

	// Process clang targets.
	clangTargets := NewClangTargets(ip.compDBPath)
	clangTargets.DataWg.Add(numRoutines)
	clangTargets.KzipDataWg.Add(numRoutines)
	clangTargets.UnitWg.Add(numRoutines)
	for i := 0; i < numRoutines; i++ {
		go func() {
			err := clangTargets.ProcessClangTargets(ip.ctx, ip.rootPath, ip.outDir, ip.corpus,
				ip.buildConfig, ip.clangTargetArch, ip.hashMaps, dataFileChannel, unitProtoChannel)
			if err != nil {
				panic(err)
			}
		}()
	}

	// Process GN targets.
	gnTargets := NewGnTargets(ip.gnTargetsPath)
	gnTargets.DataWg.Add(numRoutines)
	gnTargets.KzipDataWg.Add(numRoutines)
	gnTargets.UnitWg.Add(numRoutines)
	for i := 0; i < numRoutines; i++ {
		go func() {
			err := gnTargets.ProcessGnTargets(ip.ctx, ip.rootPath, ip.outDir, ip.corpus, ip.buildConfig, ip.hashMaps,
				dataFileChannel, unitProtoChannel)
			if err != nil {
				panic(err)
			}
		}()
	}

	// Convert data files to kzipEntries.
	kzipEntryWg.Add(numRoutines)
	for i := 0; i < numRoutines; i++ {
		go func() {
			ip.dataFileToKzipEntry(ctx, dataFileChannel, kzipEntryChannel)
			kzipEntryWg.Done()

			// Signal targets to start unit proto processing.
			gnTargets.KzipDataWg.Done()
			clangTargets.KzipDataWg.Done()
		}()
	}

	// Convert unit protos to kzipEntries.
	kzipEntryWg.Add(numRoutines)
	for i := 0; i < numRoutines; i++ {
		go func() {
			ip.unitFileToKzipEntry(ctx, unitProtoChannel, kzipEntryChannel)
			kzipEntryWg.Done()
		}()
	}

	// Close dataFileChannel and unitProtoChannel after all targets have been processed and sent.
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

	// Close kzipEntryChannel after all kzip entries have been sent.
	go func() {
		kzipEntryWg.Wait()
		close(kzipEntryChannel)
	}()

	// Write all data file and unit proto entries to kzip.
	err = ip.writeToKzip(kzipEntryChannel)

	// Print stats regardless of success.
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, '-', tabwriter.AlignRight|tabwriter.Debug)
	fmt.Fprintln(tw, "Language\tCompilationUnits\tRequiredInputs\tSourceFiles")
	numUnits := 0
	numRequiredInputs := 0
	numSourceFiles := 0
	for l, st := range ip.stats {
		nri := len(st.requiredInputs)
		nsf := len(st.sourceFiles)
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\n", l, st.numCompilationUnits, nri, nsf)
		numUnits += st.numCompilationUnits
		numRequiredInputs += nri
		numSourceFiles += nsf
	}
	fmt.Fprintf(tw, "Total\t%d\t%d\t%d\n", numUnits, numRequiredInputs, numSourceFiles)
	if tw.Flush() != nil {
		logging.Warningf(ctx, "failed to flush stats to stdout")
	}
	if err != nil {
		panic(err)
	}

	// Clean up.
	if !*filepathsFlag {
		// Remove all *.filepaths files.
		removeFilepathsFiles(ip.ctx, filepath.Join(rootPath, "src"))
	}
	logging.Infof(ctx, "%s: Done.", time.Now().Format("15:04:05"))
	os.Exit(0)
}
