// Copyright (c) 2023 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strings"

	"go.chromium.org/luci/common/data/stringset"
	"go.chromium.org/luci/common/data/text/sequence"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/experimental/crderiveinputs/inputpb"
)

type NaclToolchain struct{}

type naclToolchainStandardPackages struct {
	Packages map[string][]string `json:"packages"`
	Modes    map[string][]string `json:"modes"`
}

type naclArchive struct {
	ExtractDir string `json:"extract_dir"`
	Hash       string `json:"hash"`
	Name       string `json:"name"`
	// log_url not needed
	TarSrcDir string `json:"tar_src_dir"`
	Url       string `json:"url"` // https://storage.googleapis.com/bucket/object...
}

func (n naclArchive) bucketObject() (bucket, object string, err error) {
	const prefix = "https://storage.googleapis.com/"
	if !strings.HasPrefix(n.Url, prefix) {
		err = errors.Reason("archive.Url does not start with %q, got %q", prefix, n.Url).Err()
		return
	}
	tokens := strings.SplitN(n.Url[len(prefix):], "/", 2)
	bucket = tokens[0]
	object = tokens[1]
	return
}

type naclPackageTarget struct {
	Archives []naclArchive `json:"archives"`
	Version  int           `json:"version"`
}

type naclToolchainRevision struct {
	PackageTargets map[string]naclPackageTarget `json:"package_targets"`
	// revision/revision_hash not needed... I think
}

var naclJsonComments = regexp.MustCompile(`(?m)^\s*#.*$`)

func (n NaclToolchain) readJsonWithComments(oracle *Oracle, path string, out any) error {
	raw, err := oracle.ReadFullString(path)
	if err != nil {
		return err
	}
	raw = naclJsonComments.ReplaceAllString(raw, "")
	return json.Unmarshal([]byte(raw), out)
}

func (n NaclToolchain) HandleHook(oracle *Oracle, cwd string, action *GclientHook) (handled bool, err error) {
	pat, err := sequence.NewPattern(
		"src/build/download_nacl_toolchains.py",
		"--mode", "nacl_core_sdk",
		"sync", "--extract", "$")
	if err != nil {
		panic(err)
	}
	if pat.In(action.Action...) {
		handled = true
		LEAKY("download_nacl_toolchains.py --mode nacl_core_sdk sync --extract")

		// NOTE: the current download_nacl_toolchains.py detects the host os/cpu for
		// this. It is most likely broken for cross compilation.
		targetOS := oracle.HostOS
		var targetArch string
		switch oracle.HostCPU {
		case "x64", "x86":
			targetArch = "x86"
		case "arm", "arm64":
			targetArch = "arm"
		default:
			TODO("unable to implement download_nacl_toolchains.py for HostCPU %q", oracle.HostCPU)
			return
		}

		LEAKY("non-relative gclient deps, assuming src/ base for download_nacl_toolchains.py")
		naclBase := path.Join(cwd, "src/native_client")

		// load standard_packages.json
		std := naclToolchainStandardPackages{}
		n.readJsonWithComments(oracle, path.Join(naclBase, "build", "package_version", "standard_packages.json"), &std)

		target := fmt.Sprintf("%s_%s", targetOS, targetArch)
		allPackages, ok := std.Packages[target]
		if !ok {
			TODO("unable to implement download_nacl_toolchains.py for target %q", target)
			return
		}
		allPackagesSet := stringset.NewFromSlice(allPackages...)

		// load toolchain_revisions/<package>.json, pin GCS archives.
		targetBase := path.Join(naclBase, "toolchain", target)
		for _, pkg := range std.Modes["nacl_core_sdk"] {
			if !allPackagesSet.Has(pkg) {
				continue
			}

			// TODO: speed up oracle pinning by adding unpinned resources to the
			// manifest, and then having the oracle lazily pin, with a final pass to
			// ensure everything is fully pinned.

			var revs naclToolchainRevision
			if err = n.readJsonWithComments(oracle, path.Join(naclBase, "toolchain_revisions", pkg+".json"), &revs); err != nil {
				return
			}

			archiveNameList := []string{}

			archives, ok := revs.PackageTargets[target]
			if !ok {
				err = errors.Reason("unable to get archives for package %q - target %q", pkg, target).Err()
				return
			}
			for _, archive := range archives.Archives {
				archiveNameList = append(archiveNameList, archive.Name)

				var bucket, object string
				bucket, object, err = archive.bucketObject()
				if err != nil {
					return
				}

				extractDir := targetBase
				if archive.ExtractDir != "" {
					extractDir = path.Join(extractDir, archive.ExtractDir)
				}
				var format inputpb.GCSArchive_Format
				switch {
				case strings.HasSuffix(object, ".tar.gz") || strings.HasSuffix(object, ".tgz"):
					format = inputpb.GCSArchive_TAR_GZ
				case strings.HasSuffix(object, ".tar.bz2"):
					format = inputpb.GCSArchive_TAR_BZ2
				default:
					err = errors.Reason("do not know archive format for %q", object).Err()
					return
				}
				var sha1hash []byte
				if sha1hash, err = hex.DecodeString(archive.Hash); err != nil {
					return
				}
				hash := &inputpb.GCSBlob_Hash{
					Sha1: sha1hash,
				}
				oracle.PinGCSArchive(extractDir, bucket, object, hash, format, archive.TarSrcDir)
			}

			receipt := map[string]any{}
			receipt["version"] = 1
			receipt["archives"] = archiveNameList
			receiptContent, err := json.MarshalIndent(receipt, "", "  ")
			if err != nil {
				panic(err) // impossible
			}
			oracle.PinRawFile(path.Join(targetBase, pkg, pkg+".json"), string(receiptContent), "download_nacl_toolchains.py hook")
		}

		return
	}
	return
}
