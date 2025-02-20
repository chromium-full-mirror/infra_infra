// Copyright (c) 2023 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"go.chromium.org/luci/cipd/client/cipd/ensure"
	"go.chromium.org/luci/cipd/client/cipd/template"
	"go.chromium.org/luci/cipd/common"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/experimental/crderiveinputs/inputpb"
	"go.chromium.org/infra/experimental/crderiveinputs/inputs"
)

func (o *Oracle) PinCipdEnsureFile(root, ensureFile string) error {
	cmRaw, err := o.ReadFullString(ensureFile)
	if err != nil {
		return err
	}
	cm, err := ensure.ParseFile(strings.NewReader(cmRaw))
	if err != nil {
		return err
	}

	var verFilePath string
	var vers *ensure.VersionsFile
	if cm.ResolvedVersions != "" {
		verFilePath = path.Join(path.Dir(ensureFile), cm.ResolvedVersions)
		versRaw, err := o.ReadFullString(verFilePath)
		if err != nil {
			return err
		}
		verFile, err := ensure.ParseVersionsFile(strings.NewReader(versRaw))
		if err != nil {
			return err
		}
		vers = &verFile
	}

	for subdir, pkgs := range cm.PackagesBySubdir {
		subdir, err := o.cipdExpander.Expand(subdir)
		if err != nil {
			return err
		}

		for _, pkg := range pkgs {
			pth := root
			if subdir != "" {
				pth = path.Join(pth, subdir)
			}
			if err := o.PinCipd(pth, pkg, vers, verFilePath); err != nil {
				return err
			}
		}
	}

	return nil
}

func (o *Oracle) pinCipd(pkg ensure.PackageDef, verFile *ensure.VersionsFile, verFilePath string, expander template.Expander) (*inputpb.CIPDPackage, error) {
	Logger.Debugf("oracle.pinCIPD(%q, versFile=%t)", pkg, verFile != nil)

	expandPkg, err := expander.Expand(pkg.PackageTemplate)
	if err == template.ErrSkipTemplate {
		Logger.Infof("Skipping CIPD due to template parameters: %s", pkg.PackageTemplate)
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	pkgSrc := "pin"
	if expandPkg != pkg.PackageTemplate {
		pkgSrc = "crderiveinputs -cipd-{os,arch}"
	}

	var verSrc string
	var pin common.Pin
	if verFile != nil {
		if pin, err = verFile.ResolveVersion(expandPkg, pkg.UnresolvedVersion); err != nil {
			return nil, err
		}
		verSrc = verFilePath
		if pin.InstanceID == pkg.UnresolvedVersion {
			verSrc = "pin"
		}
	} else {
		LEAKY("assuming CIPD ServiceURL %s", o.cipdClient.Options().ServiceURL)
		if pin, err = o.cipdClient.ResolveVersion(o.ctx, expandPkg, pkg.UnresolvedVersion); err != nil {
			return nil, err
		}
		verSrc = "pin"
		if pin.InstanceID != pkg.UnresolvedVersion {
			verSrc = o.cipdClient.Options().ServiceURL
		}
	}

	return &inputpb.CIPDPackage{
		Pkg:     inputs.Resolved(pkg.PackageTemplate, pin.PackageName, pkgSrc),
		Version: inputs.Resolved(pkg.UnresolvedVersion, pin.InstanceID, verSrc),
	}, nil
}

func (o *Oracle) PinCipd(path string, pkg ensure.PackageDef, verFile *ensure.VersionsFile, verFilePath string) error {
	resolved, err := o.pinCipd(pkg, verFile, verFilePath, o.cipdExpander)
	if err != nil {
		return err
	}

	o.withSource(path, func(s *inputpb.Source) {
		inputs.AddCIPD(s, resolved)
	})

	return nil
}

type gitRevisionType int

const (
	Commit gitRevisionType = iota
	ShortCommit
	UnknownRevision
)

func parseGitRevision(rev string) gitRevisionType {
	_, err := hex.DecodeString(rev)
	if err == nil {
		if len(rev) == 40 {
			return Commit
		}
		return ShortCommit
	}
	return UnknownRevision
}

func (o *Oracle) PinGit(path, URL, rev string) error {
	Logger.Debugf("oracle.PinGit(%q, %q, %q)", path, URL, rev)

	var resolved string
	var source string
	switch parseGitRevision(rev) {
	case Commit:
		resolved = rev
		source = "pin"
	case ShortCommit:
		return fmt.Errorf("truncated hashes not supported")
	default:
		Logger.Infof("Resolving %s %s", URL, rev)
		args := []string{"ls-remote"}
		if strings.HasPrefix(rev, "refs/heads/") {
			args = append(args, "-h")
		} else if strings.HasPrefix(rev, "refs/tags/") {
			args = append(args, "-t")
		} else {
			Logger.Warningf("... SLOW!!! Revision doesn't start with refs/heads/ or refs/tags/ - scanning all remote refs")
		}
		args = append(args, URL, rev)

		commit_refs, err := o.gitRepo(URL).output(args...)
		if err != nil {
			return err
		}

		lines := strings.Split(strings.TrimSpace(commit_refs), "\n")
		if len(lines) == 0 {
			return errors.Reason("No refs match %q", rev).Err()
		}
		if len(lines) > 1 {
			return errors.Reason("Too many refs match %q", rev).Err()
		}

		resolved = strings.Split(lines[0], "\t")[0]
		source = "git remote"
		if resolved == "" {
			return errors.Reason("unable to resolve %q %q", URL, rev).Err()
		}
		Logger.Infof("... Got %q", resolved)
	}

	o.withSource(path, func(s *inputpb.Source) {
		inputs.AddGitSource(s, URL, inputs.Resolved(rev, resolved, source))
	})
	return nil
}

func (o *Oracle) pinGcsBlob(path string, blob *inputpb.GCSBlob) error {
	gsurl := fmt.Sprintf("gs://%s/%s", blob.Bucket, blob.Object)

	if h := blob.Hash; h == nil || h.Size == nil || h.Sha256 == nil {
		if h == nil {
			h = &inputpb.GCSBlob_Hash{}
			blob.Hash = h
		}

		if h.Sha1 != nil {
			PIN("GCS Object %q (%s) not fully pinned with size+sha2.", path, gsurl)
		} else {
			Logger.Errorf("PIN - GCS Object %q (%s) has NO HASH PIN AT ALL!", path, gsurl)
		}
		reader, err := o.gcsClient.Bucket(blob.Bucket).Object(blob.Object).NewReader(o.ctx)
		if err != nil {
			return err
		}
		defer reader.Close()
		h.Generation = &wrapperspb.Int64Value{Value: reader.Attrs.Generation}

		cachePath := filepath.Join(o.gcsCachePath, blob.Bucket, blob.Object, fmt.Sprint(h.Generation.Value))
		cacheData, err := os.ReadFile(cachePath)
		if os.IsNotExist(err) {
			err = nil
		} else if err != nil {
			Logger.Warningf("error reading GCS cache data %q - %s", cachePath, err)
			if err = os.Remove(cachePath); err != nil {
				Logger.Warningf("... and error clearing this cache entry - %s", err)
			}
			err = nil
		} else {
			decodedHash := &inputpb.GCSBlob_Hash{}
			if err = protojson.Unmarshal(cacheData, decodedHash); err != nil {
				return err
			}
			if fullyPopulated(decodedHash) {
				blob.Hash = decodedHash
				return nil
			} else {
				// otherwise treat this as missing cache entry
				Logger.Warningf("GCS cache data %q - not fully populated", cachePath, err)
				if err = os.Remove(cachePath); err != nil {
					Logger.Warningf("... and error clearing this cache entry - %s", err)
				}
			}
		}

		Logger.Warningf("SLOW!!! Fetching %s", gsurl)

		actualSize := reader.Size()
		if h.Size != nil {
			if h.Size.Value != actualSize {
				return errors.Reason("mismatched GCS blob size - %s - expected %d, got %d", gsurl, h.Size.Value, actualSize).Err()
			}
		} else {
			h.Size = &wrapperspb.Int64Value{Value: actualSize}
		}

		sha1hash := sha1.New()
		sha2hash := sha256.New()
		tReader := io.TeeReader(reader, io.MultiWriter(sha1hash, sha2hash))

		// drain the file with a 512KB buffer. This will write to one (or both) sha
		// verifiers.
		var buf [512 * 1024]byte
		for err == nil {
			_, err = tReader.Read(buf[:])
		}
		if err == io.EOF {
			err = nil
		} else {
			return err
		}

		actualSha2 := sha2hash.Sum(nil)
		if h.Sha256 != nil {
			if !bytes.Equal(actualSha2, h.Sha256) {
				return errors.Reason("mismatched GCS sha2 hash - %s - expected %x, got %x", gsurl, h.Sha256, actualSha2).Err()
			}
		}
		h.Sha256 = actualSha2

		actualSha1 := sha1hash.Sum(nil)
		if h.Sha1 != nil {
			if !bytes.Equal(actualSha1, h.Sha1) {
				return errors.Reason("mismatched GCS sha1 hash - %s - expected %x, got %x", gsurl, h.Sha1, actualSha1).Err()
			}
		}
		h.Sha1 = actualSha1

		if cacheData, err = protojson.Marshal(h); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(cachePath), 0700); err != nil {
			return err
		}
		if err := os.WriteFile(cachePath, cacheData, 0400); err != nil {
			return err
		}
	}

	return nil
}

func (o *Oracle) PinGCSFile(path, bucket, object string, hash *inputpb.GCSBlob_Hash) error {
	blob := &inputpb.GCSBlob{
		Bucket: bucket,
		Object: object,
		Hash:   hash,
	}

	if err := o.pinGcsBlob(path, blob); err != nil {
		return err
	}
	o.withSource(path, func(s *inputpb.Source) {
		inputs.AddGCSFile(s, blob)
	})
	return nil
}

func (o *Oracle) PinGCSArchive(path, bucket, object string, hash *inputpb.GCSBlob_Hash, format inputpb.GCSArchive_Format, extractSubdir string) error {
	blob := &inputpb.GCSBlob{
		Bucket: bucket,
		Object: object,
		Hash:   hash,
	}

	if err := o.pinGcsBlob(path, blob); err != nil {
		return err
	}
	o.withSource(path, func(s *inputpb.Source) {
		inputs.AddGCSArchive(s, blob, extractSubdir, format)
	})
	return nil
}

func (o *Oracle) PinRawFile(path, contents, source string) {
	o.withSource(path, func(s *inputpb.Source) {
		inputs.AddRawFile(s, contents, source)
	})
}
