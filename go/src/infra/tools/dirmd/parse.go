// Copyright 2022 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package dirmd

import (
	"io"
	"os"
	"path/filepath"

	"google.golang.org/protobuf/encoding/prototext"

	"go.chromium.org/luci/common/errors"

	dirmdpb "go.chromium.org/infra/tools/dirmd/proto"
)

// ParseFile parses the metadata file to dirmd.Metadata.
//
// A valid file has a base filename "DIR_METADATA" or "OWNERS".
// The format of its contents correspond to the base name.
func ParseFile(fileName string) (*dirmdpb.Metadata, error) {
	base := filepath.Base(fileName)
	if base != Filename && base != OwnersFilename {
		return nil, errors.Reason("unexpected base filename %q; expected DIR_METADATA, OWNERS", base).Err()
	}

	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	md := &dirmdpb.Metadata{}
	if base == Filename {
		contents, err := io.ReadAll(f)
		if err != nil {
			return nil, err
		}
		return md, prototext.Unmarshal(contents, md)
	}

	md, _, err = ParseOwners(f)
	return md, err
}
