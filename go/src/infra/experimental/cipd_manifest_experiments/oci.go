// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"sync"

	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	specsV1 "github.com/opencontainers/image-spec/specs-go/v1"

	"go.chromium.org/luci/cipd/client/cipd/fs"
)

type OCI struct {
	schema *specsV1.Manifest
	hashes [][]byte
	mu     sync.Mutex
}

var _ manifester = (*OCI)(nil)

func (o *OCI) init(packageName string) {
	o.schema = &specsV1.Manifest{
		Versioned: specs.Versioned{SchemaVersion: 2},
		// I think this mediaType may be incorrect, because we're going to have
		// out-of-spec layer descriptors?
		MediaType:    "application/vnd.oci.image.manifest.v1+json",
		ArtifactType: "application/vnd.google.archive.v1+json",
		// Config? We need package name in here somewhere.
		Layers: []specsV1.Descriptor{},
	}
}

func (o *OCI) makeAttrs(desc *specsV1.Descriptor, file fs.File) {
	// TODO: model these attributes in a better way
	if file.Executable() {
		desc.Annotations["exec"] = "1"
	}
	if file.Writable() {
		desc.Annotations["writable"] = "1"
	}
	if mt := file.ModTime(); !mt.IsZero() {
		desc.Annotations["modtime"] = mt.String()
	}
	if wa := file.WinAttrs(); wa != 0 {
		desc.Annotations["win_attrs"] = wa.String()
	}
}

func (o *OCI) handleSymlink(file fs.File) []specsV1.Descriptor {
	data := []byte(must(file.SymlinkTarget()))

	desc := specsV1.Descriptor{
		Annotations: map[string]string{
			"path": file.Name(),
		},
		MediaType: "application/vnd.google.archive.symlink.v1",
		Data:      data,
		Size:      int64(len(data)),
		Digest:    o.digest(data),
	}
	return []specsV1.Descriptor{desc}
}

func (o *OCI) digest(data []byte, skipRecord ...bool) digest.Digest {
	h := sha256.New()
	h.Write(data)
	hsh := h.Sum(nil)
	if len(skipRecord) > 0 && skipRecord[0] {
		o.mu.Lock()
		o.hashes = append(o.hashes, hsh)
		o.mu.Unlock()
	}
	return digest.Digest("sha256:" + hex.EncodeToString(hsh))
}

func (o *OCI) handleFile(file fs.File) []specsV1.Descriptor {
	rc := must(file.Open())
	defer rc.Close()
	// TODO: avoid paging all data into memory
	data := must(io.ReadAll(rc))
	// don't record digest here - we don't actually have this blob in CAS
	overallHash := o.digest(data, true)

	var rets []specsV1.Descriptor

	if file.Size() > LargeFileThreshold {
		chunkNum := 0
		for chunk := range slices.Chunk(data, LargeFileThreshold) {
			rets = append(rets, specsV1.Descriptor{
				MediaType: "application/vnd.google.archive.filechunk.v1",
				Annotations: map[string]string{
					"path":   file.Name(),
					"offset": fmt.Sprint(chunkNum * LargeFileThreshold),
				},
				Size:   int64(len(chunk)),
				Digest: o.digest(chunk),
			})
			chunkNum++
		}
		rets[len(rets)-1].Annotations["overallSize"] = fmt.Sprint(len(data))
		rets[len(rets)-1].Annotations["overallDigest"] = string(overallHash)
	} else {
		desc := specsV1.Descriptor{
			MediaType: "application/octet-stream",
			Digest:    overallHash,
			Annotations: map[string]string{
				"path": file.Name(),
			},
		}
		if file.Size() < InlineThreshold {
			desc.Data = data
			desc.Size = int64(len(data))
		}

		rets = []specsV1.Descriptor{desc}
	}

	return rets
}

func (o *OCI) process(file fs.File) {
	var descs []specsV1.Descriptor
	if file.Symlink() {
		descs = o.handleSymlink(file)
	} else {
		descs = o.handleFile(file)
	}

	o.makeAttrs(&descs[len(descs)-1], file)

	o.mu.Lock()
	defer o.mu.Unlock()
	i, _ := slices.BinarySearchFunc(o.schema.Layers, descs[0], func(el, target specsV1.Descriptor) int {
		return cmp.Compare(el.Annotations["path"], target.Annotations["path"])
	})
	o.schema.Layers = slices.Insert(o.schema.Layers, i, descs...)
}

func (o *OCI) fixup() {}

func (o *OCI) dump(dumpType string, outPath string) {
	ofile := must(os.Create(outPath))
	defer ofile.Close()

	// TODO: Deterministically serialize manifest
	enc := json.NewEncoder(ofile)
	enc.SetIndent("", "  ")
	if err := enc.Encode(o.schema); err != nil {
		panic(err)
	}
}

func (o *OCI) getrefs() *HashGroups {
	return &HashGroups{Groups: []*HashGroups_Group{
		makeHashGroup(o.hashes),
	}}
}
