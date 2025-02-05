// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"cmp"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"sync"

	"github.com/bazelbuild/remote-apis-sdks/go/pkg/digest"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"go.chromium.org/luci/cipd/client/cipd/fs"
)

type remoteCAS struct {
	// map dir -> Directory
	dirs   map[string]*remoteexecution.Directory
	dirMus map[string]*sync.Mutex
	dirsMu sync.Mutex

	dirHashes  [][]byte
	fileHashes [][]byte
	hashMu     sync.Mutex
}

var _ manifester = (*remoteCAS)(nil)

func (r *remoteCAS) digest(data []byte) *remoteexecution.Digest {
	ret := digest.NewFromBlob(data).ToProto()
	hsh := must(hex.DecodeString(ret.Hash))
	r.hashMu.Lock()
	defer r.hashMu.Unlock()
	r.fileHashes = append(r.fileHashes, hsh)
	return ret
}

func (r *remoteCAS) digestDir(dir *remoteexecution.Directory) *remoteexecution.Digest {
	ret := must(digest.NewFromMessage(dir)).ToProto()
	hsh := must(hex.DecodeString(ret.Hash))
	r.hashMu.Lock()
	defer r.hashMu.Unlock()
	r.dirHashes = append(r.dirHashes, hsh)
	return ret
}

func (r *remoteCAS) getDir(dirname string) (*remoteexecution.Directory, func()) {
	var dir *remoteexecution.Directory
	var dirMu *sync.Mutex
	func() {
		r.dirsMu.Lock()
		defer r.dirsMu.Unlock()

		var ok bool
		dir, ok = r.dirs[dirname]
		if !ok {
			dir = &remoteexecution.Directory{}
			r.dirs[dirname] = dir
		}

		dirMu, ok = r.dirMus[dirname]
		if !ok {
			dirMu = &sync.Mutex{}
			r.dirMus[dirname] = dirMu
		}
	}()

	dirMu.Lock()
	return dir, dirMu.Unlock
}

func (r *remoteCAS) addProperty(dir *remoteexecution.Directory, name, value string) {
	if dir.NodeProperties == nil {
		dir.NodeProperties = &remoteexecution.NodeProperties{}
	}
	dir.NodeProperties.Properties = append(dir.NodeProperties.Properties, &remoteexecution.NodeProperty{
		Name: name, Value: value,
	})
}

func (r *remoteCAS) init(packageName string) {
	r.dirs = map[string]*remoteexecution.Directory{}
	r.dirMus = map[string]*sync.Mutex{}

	dir, closer := r.getDir(".")
	defer closer()

	r.addProperty(dir, "packageName", packageName)
}

func (r *remoteCAS) fileToProps(file fs.File) *remoteexecution.NodeProperties {
	ret := &remoteexecution.NodeProperties{}
	needed := false

	var mode uint32
	if file.Executable() {
		mode |= 0111
	}
	if file.Writable() {
		mode |= 0222
	}
	if mode > 0 {
		needed = true
		ret.UnixMode = &wrapperspb.UInt32Value{Value: mode}
	}
	if mt := file.ModTime(); !mt.IsZero() {
		needed = true
		ret.Mtime = timestamppb.New(mt)
	}
	if wa := file.WinAttrs(); wa != 0 {
		needed = true
		// TODO: better encoding
		ret.Properties = append(ret.Properties, &remoteexecution.NodeProperty{
			Name: "winAttrs", Value: wa.String(),
		})
	}
	if !needed {
		return nil
	}
	return ret
}

func (r *remoteCAS) makeChunkedDirEntry(file fs.File) *remoteexecution.Digest {
	rc := must(file.Open())
	defer rc.Close()
	// TODO: avoid paging all data into memory
	data := must(io.ReadAll(rc))

	dir, closer := r.getDir(file.Name())
	defer closer()

	chunkNum := 0
	// TODO - use a smarter chunking algorithm
	// TODO - hash chunks in parallel
	for chunk := range slices.Chunk(data, LargeFileThreshold) {
		dir.Files = append(dir.Files, &remoteexecution.FileNode{
			Name:   fmt.Sprint(chunkNum * LargeFileThreshold),
			Digest: r.digest(chunk),
		})
	}

	dir.NodeProperties = r.fileToProps(file)
	r.addProperty(dir, "purpose", "chunked-file")
	// TODO - record overall hash

	return r.digestDir(dir)
}

func (r *remoteCAS) process(file fs.File) {
	path, name := path.Split(file.Name())
	dir, closer := r.getDir(path)
	defer closer()

	if file.Symlink() {
		dir.Symlinks = append(dir.Symlinks, &remoteexecution.SymlinkNode{
			Name:           file.Name(),
			Target:         must(file.SymlinkTarget()),
			NodeProperties: r.fileToProps(file),
		})
	} else {
		if file.Size() < LargeFileThreshold {
			rc := must(file.Open())
			defer rc.Close()
			// TODO: avoid paging all data into memory
			data := must(io.ReadAll(rc))

			dir.Files = append(dir.Files, &remoteexecution.FileNode{
				Name:           name,
				Digest:         r.digest(data),
				IsExecutable:   file.Executable(),
				NodeProperties: r.fileToProps(file),
			})
		} else {
			dir.Directories = append(dir.Directories, &remoteexecution.DirectoryNode{
				Name:   name,
				Digest: r.makeChunkedDirEntry(file),
			})
		}
	}
}

func (r *remoteCAS) fixup() {
	// walk through all dirs and roll them up
	dirPaths := make([]string, 0, len(r.dirs))
	for k := range r.dirs {
		dirPaths = append(dirPaths, k)
	}
	sort.Strings(dirPaths)

	resolved := map[string]*remoteexecution.Directory{}

	for _, k := range slices.Backward(dirPaths) {
		base := r.dirs[k]
		toMerge := resolved[k]
		base.Files = append(base.Files, toMerge.GetFiles()...)
		slices.SortFunc(base.Files, func(a, b *remoteexecution.FileNode) int {
			return cmp.Compare(a.Name, b.Name)
		})
		base.Directories = append(base.Directories, toMerge.GetDirectories()...)
		slices.SortFunc(base.Directories, func(a, b *remoteexecution.DirectoryNode) int {
			return cmp.Compare(a.Name, b.Name)
		})

		parentPath := path.Dir(path.Clean(k))
		parent := resolved[parentPath]
		if parent == nil {
			parent = &remoteexecution.Directory{}
			resolved[parentPath] = parent
		}
		parent.Directories = append(parent.Directories, &remoteexecution.DirectoryNode{
			Name:   path.Base(k),
			Digest: r.digestDir(base),
		})
	}
}

func (r *remoteCAS) dump(dumpType, outPath string) {
	if err := os.MkdirAll(outPath, 0777); err != nil {
		panic(err)
	}
	for path, dir := range r.dirs {
		digest := r.digestDir(dir)
		if path == "." {
			fmt.Println("ROOT", digest.Hash)
		}
		var data []byte
		if dumpType == "rbe+json" {
			data = []byte(protojson.Format(dir))
		} else {
			data = must(proto.Marshal(dir))
		}

		targetPath := filepath.Join(outPath, digest.Hash)
		os.Remove(targetPath)
		if err := os.WriteFile(targetPath, data, 0666); err != nil {
			panic(err)
		}
	}
}

func (r *remoteCAS) getrefs() *HashGroups {
	return &HashGroups{Groups: []*HashGroups_Group{
		makeHashGroup(r.dirHashes),
		makeHashGroup(r.fileHashes),
	}}
}
