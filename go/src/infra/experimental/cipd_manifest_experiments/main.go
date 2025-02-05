// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:generate cproto

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/luci/cipd/client/cipd/fs"
	"go.chromium.org/luci/cipd/client/cipd/pkg"
	"go.chromium.org/luci/cipd/client/cipd/reader"
	"go.chromium.org/luci/common/sync/parallel"
)

func must[T any](val T, err error) T {
	if err != nil {
		panic(err)
	}
	return val
}

type manifester interface {
	init(packageName string)
	process(file fs.File)
	fixup()
	dump(dumpType string, outPath string)
	getrefs() *HashGroups
}

const InlineThreshold = 256
const LargeFileThreshold = 1e7

var manifesterMap = map[string]func() manifester{
	"pb":       func() manifester { return &ProtoManifest{} },
	"oci":      func() manifester { return &OCI{} },
	"jsonpb":   func() manifester { return &ProtoManifest{} },
	"rbe":      func() manifester { return &remoteCAS{} },
	"rbe+json": func() manifester { return &remoteCAS{} },
}

// Transforms an existing CIPD package on disk into a theoretical OCI Image
// Manifest 1.1 JSON.
//
// Manifest spec: https://github.com/opencontainers/image-spec/blob/main/manifest.md
//
// Some things (like mediaType fields) are made up for this theoretical format.
func main() {
	path := ""
	opts := reader.OpenInstanceOpts{
		VerificationMode: reader.SkipHashVerification,
		// Just some random InstanceID - it doesn't matter for this tool.
		InstanceID: "Ie7jj9gt87p4M9aSPqoogrnGqkR8aNKYTR5m3gUOc3oC",
	}
	var output manifester
	dumpType := "oci"
	switch {
	case len(os.Args) == 3:
		dumpType = os.Args[2]
		if _, ok := manifesterMap[dumpType]; !ok {
			fmt.Fprintf(os.Stderr, "unknown dumpType %q\n", dumpType)
			os.Exit(1)
		}
		fallthrough
	case len(os.Args) == 2:
		path = os.Args[1]
	default:
		fmt.Fprintln(os.Stderr, "cipd_manifest_experiments path/to/pkg [oci|pb|jsonpb]")
		os.Exit(1)
	}
	output = manifesterMap[dumpType]()

	ctx := context.Background()
	input := must(reader.OpenInstance(ctx, must(pkg.NewFileSource(path)), opts))
	defer input.Close(ctx, false)

	P := runtime.NumCPU() * 2

	files := input.Files()
	fcount := len(files)

	output.init(input.Pin().PackageName)

	counter := int64(0)
	err := parallel.WorkPool(P, func(c chan<- func() error) {
		for _, file := range files {
			c <- func() error {
				output.process(file)
				if cval := atomic.AddInt64(&counter, 1); cval%1000 == 0 {
					fmt.Fprintf(os.Stderr, "added %d/%d files\r", cval, fcount)
				}
				return nil
			}
		}
	})
	if err != nil {
		panic(err)
	}

	fmt.Fprintf(os.Stderr, "added %d/%d files\r", counter, fcount)
	fmt.Fprintln(os.Stderr, "\nfixup")
	output.fixup()

	fmt.Fprintln(os.Stderr, "\nserializing")

	outBase := fmt.Sprintf("%s.%s", path, dumpType)
	output.dump(dumpType, outBase)

	if strings.Contains(dumpType, "json") {
		refFile := must(os.Create(outBase + ".refs+json"))
		defer refFile.Close()
		must(refFile.WriteString(protojson.Format(output.getrefs())))
	} else {
		refFile := must(os.Create(outBase + ".refs"))
		defer refFile.Close()
		must(refFile.Write(must(proto.Marshal(output.getrefs()))))
	}
}

func makeHashGroup(raw [][]byte) *HashGroups_Group {
	buf := slices.Clone(raw)
	slices.SortFunc(buf, func(a, b []byte) int {
		return bytes.Compare(a, b)
	})
	// modify buf in-place, only picking unique hashes.
	group := buf[:0]
	for _, hsh := range buf {
		if len(group) == 0 || !bytes.Equal(group[len(group)-1], hsh) {
			group = append(group, hsh)
		}
	}
	return &HashGroups_Group{Hashes: group}
}
