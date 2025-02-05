// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"cmp"
	"crypto/sha256"
	"io"
	"os"
	"slices"
	"sync"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/luci/cipd/client/cipd/fs"
)

type ProtoManifest struct {
	m      *Manifest
	hashes [][]byte
	mu     sync.Mutex
}

var _ manifester = (*ProtoManifest)(nil)

func (p *ProtoManifest) init(packageName string) {
	p.m = &Manifest{
		Version:     1,
		PackageName: packageName,
	}
}

func (p *ProtoManifest) makeAttrs(file fs.File) *Attributes {
	ret := &Attributes{
		Executable: file.Executable(),
		Writable:   file.Writable(),
	}
	needed := ret.Executable || ret.Writable
	if mt := file.ModTime(); !mt.IsZero() {
		needed = true
		ret.Modtime = timestamppb.New(file.ModTime())
	}
	if wa := file.WinAttrs(); wa != 0 {
		needed = true
		ret.WinAttrs = &WinAttributes{
			System: (wa | fs.WinAttrSystem) != 0,
			Hidden: (wa | fs.WinAttrHidden) != 0,
		}
	}
	if !needed {
		return nil
	}
	return ret
}

func (p *ProtoManifest) digest(data []byte) *Digest {
	h := sha256.New()
	h.Write(data)
	ret := &Digest{
		Size:   int64(len(data)),
		Sha256: h.Sum(nil),
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.hashes = append(p.hashes, ret.Sha256)
	return ret
}

func (p *ProtoManifest) process(file fs.File) {
	toAdd := &File{
		Path:       file.Name(),
		Attributes: p.makeAttrs(file),
	}

	if file.Symlink() {
		toAdd.Data = &File_Symlink{Symlink: must(file.SymlinkTarget())}
	} else { // directly embed small files
		rc := must(file.Open())
		defer rc.Close()
		// TODO: avoid paging all data into memory
		data := must(io.ReadAll(rc))

		if len(data) < InlineThreshold {
			toAdd.Data = &File_InlineContent{InlineContent: data}
		} else {
			if len(data) > LargeFileThreshold {
				chunks := &Digests{OverallDigest: p.digest(data)}
				for chunk := range slices.Chunk(data, LargeFileThreshold) {
					chunks.Digests = append(chunks.Digests, p.digest(chunk))
				}
				toAdd.Data = &File_Chunked{Chunked: chunks}
			} else {
				toAdd.Data = &File_Chunk{Chunk: p.digest(data)}
			}
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	i, _ := slices.BinarySearchFunc(p.m.Files, toAdd, func(el, target *File) int {
		return cmp.Compare(el.Path, target.Path)
	})
	p.m.Files = slices.Insert(p.m.Files, i, toAdd)
}

func (p *ProtoManifest) fixup() {}

func (p *ProtoManifest) dump(dumpType string, outPath string) {
	ofile := must(os.Create(outPath))
	defer ofile.Close()

	var data []byte
	// TODO: have deterministic serialization
	if dumpType == "jsonpb" {
		data = []byte(protojson.Format(p.m))
	} else {
		data = must(proto.Marshal(p.m))
	}
	must(ofile.Write(data))
}

func (p *ProtoManifest) getrefs() *HashGroups {
	return &HashGroups{
		Groups: []*HashGroups_Group{
			makeHashGroup(p.hashes),
		}}
}
