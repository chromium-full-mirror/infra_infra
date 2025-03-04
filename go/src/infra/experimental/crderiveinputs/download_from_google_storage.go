// Copyright (c) 2023 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"encoding/hex"
	"flag"
	"path"
	"strings"

	"golang.org/x/sync/errgroup"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/experimental/crderiveinputs/inputpb"
)

func shortLongStringVar(fs *flag.FlagSet, target *string, short, long string) {
	// target nil == ignore
	if target == nil {
		target = new(string)
	}
	fs.StringVar(target, short, "", "")
	fs.StringVar(target, long, "", "")
}

func shortLongBoolVar(fs *flag.FlagSet, target *bool, short, long string) {
	// target nil == ignore
	if target == nil {
		target = new(bool)
	}
	fs.BoolVar(target, short, false, "")
	fs.BoolVar(target, long, false, "")
}

type DownloadFromGCS struct{}

type downloadFromGCSArgs struct {
	output       string
	bucket       string
	recursive    bool
	directory    bool
	sha1File     string
	platform     string
	autoPlatform bool
	extract      bool

	targets []string
}

func (d DownloadFromGCS) parseCLI(args []string) (match bool, ret downloadFromGCSArgs, err error) {
	idx := 0
	prog := args[idx]
	if knownPythonNames.Has(prog) {
		idx = 1
		prog = args[idx]
	}
	if prog != "download_from_google_storage" && !strings.HasSuffix(prog, "download_from_google_storage.py") {
		// no match
		return
	}
	LEAKY("download_from_google_storage.py CLI argument parser")
	match = true

	fs := flag.NewFlagSet("download_from_google_storage", flag.ContinueOnError)

	shortLongStringVar(fs, &ret.output, "o", "output")
	shortLongStringVar(fs, &ret.bucket, "b", "bucket")
	shortLongStringVar(fs, &ret.sha1File, "s", "sha1_file")
	shortLongStringVar(fs, &ret.platform, "p", "platform")
	shortLongBoolVar(fs, &ret.directory, "d", "directory")

	shortLongBoolVar(fs, &ret.recursive, "r", "recursive")
	shortLongBoolVar(fs, &ret.autoPlatform, "a", "auto_platform")
	shortLongBoolVar(fs, &ret.extract, "u", "extract")

	// ignored options
	shortLongStringVar(fs, nil, "e", "boto")    // Configure where the boto file is.
	shortLongBoolVar(fs, nil, "c", "no_resume") // Should downloads be resumable.
	shortLongBoolVar(fs, nil, "f", "force")
	shortLongBoolVar(fs, nil, "i", "ignore_errors")
	shortLongStringVar(fs, nil, "t", "num_threads")
	shortLongBoolVar(fs, nil, "g", "config")
	shortLongBoolVar(fs, nil, "n", "no_auth")
	shortLongBoolVar(fs, nil, "v", "verbose")
	shortLongBoolVar(fs, nil, "q", "quiet")

	err = fs.Parse(args[idx+1:])
	if err != nil {
		err = errors.Annotate(err, "download_from_google_storage").Err()
		return
	}

	ret.targets = fs.Args()

	return
}

func (d DownloadFromGCS) HandleHook(oracle *Oracle, cwd string, hook *GclientHook) (handled bool, err error) {
	var args downloadFromGCSArgs
	handled, args, err = d.parseCLI(hook.Action)
	if err != nil || !handled {
		return
	}

	if args.directory {
		if args.extract {
			TODO("download_from_google_storage with --directory --extract")
			return true, nil
		}
		eg := errgroup.Group{}
		for _, dir := range args.targets {
			listing, err := oracle.WalkDirectory(path.Join(cwd, dir), "*.sha1", "**/*.sha1")
			if err != nil {
				return true, err
			}
			for _, targetSha := range listing {
				eg.Go(func() error {
					shaContents, err := oracle.ReadFullString(targetSha)
					if err != nil {
						return err
					}
					shaBytes, err := hex.DecodeString(shaContents)
					if err != nil {
						return err
					}
					return oracle.PinGCSFile(targetSha[:len(targetSha)-len(".sha1")], args.bucket, shaContents, &inputpb.GCSBlob_Hash{
						Sha1: shaBytes,
					})
				})
			}
		}
		return true, eg.Wait()
	} else if args.sha1File != "" {
		targetSha := path.Join(cwd, args.sha1File)

		shaContents, err := oracle.ReadFullString(targetSha)
		if err != nil {
			return true, err
		}
		shaContents = strings.TrimSpace(shaContents)
		shaBytes, err := hex.DecodeString(shaContents)
		if err != nil {
			return true, err
		}

		output := args.output
		if args.output == "" {
			if strings.HasSuffix(targetSha, ".sha1") {
				output = targetSha[:len(targetSha)-len(".sha1")]
			} else {
				return true, errors.New("don't know how to handle non-sha1 extension")
			}
		}

		if args.extract {
			if !strings.HasSuffix(output, ".tar.gz") {
				return true, errors.Reason("download_from_google_storage with `-s --extract` but file does not end with .tar.gz: %q", output).Err()
			}

			return true, oracle.PinGCSArchive(output, args.bucket, shaContents, &inputpb.GCSBlob_Hash{
				Sha1: shaBytes,
			}, inputpb.GCSArchive_TAR_GZ, "")
		} else {
			return true, oracle.PinGCSFile(output, args.bucket, shaContents, &inputpb.GCSBlob_Hash{
				Sha1: shaBytes,
			})
		}
	} else {
		TODO("download_from_google_storage with unknown args %+v", args)
	}

	return
}
