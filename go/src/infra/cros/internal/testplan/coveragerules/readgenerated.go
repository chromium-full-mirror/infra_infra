// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package coveragerules

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"cloud.google.com/go/bigquery"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	testpb "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/bq"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/proto/google/descutil"

	"go.chromium.org/infra/cros/internal/git"
)

// getAllFileDescriptorProtos finds the FileDescriptorProto for descriptor and
// recursively finds the FileDescriptorProtos for all messages that are fields
// in descriptor. Note that there is no effort to dedupe the
// FileDescriptorProtos or prevent infinite recursion (although this would only
// happen if there was an import cycle in the protos).
func getAllFileDescriptorProtos(descriptor protoreflect.MessageDescriptor) []*descriptorpb.FileDescriptorProto {
	fdps := make([]*descriptorpb.FileDescriptorProto, 0)
	fdps = append(fdps, protodesc.ToFileDescriptorProto(descriptor.ParentFile()))

	for i := 0; i < descriptor.Fields().Len(); i += 1 {
		fieldDesc := descriptor.Fields().Get(i)
		if fieldDesc.Message() != nil {
			fdps = append(fdps, getAllFileDescriptorProtos(fieldDesc.Message())...)
		}
	}

	return fdps
}

// GenerateCoverageRuleBqRowSchema generates a BigQuery schema for the
// CoverageRuleBqRow proto.
func GenerateCoverageRuleBqRowSchema() (bigquery.Schema, error) {
	fdset := &descriptorpb.FileDescriptorSet{
		File: getAllFileDescriptorProtos((&testpb.CoverageRuleBqRow{}).ProtoReflect().Descriptor()),
	}

	conv := bq.SchemaConverter{
		Desc:           fdset,
		SourceCodeInfo: make(map[*descriptorpb.FileDescriptorProto]bq.SourceCodeInfoMap, len(fdset.File)),
	}

	var err error
	for _, f := range fdset.File {
		conv.SourceCodeInfo[f], err = descutil.IndexSourceCodeInfo(f)
		if err != nil {
			return nil, fmt.Errorf("failed to index source code info in file %q: %w", f.GetName(), err)
		}
	}

	schema, _, err := conv.Schema("chromiumos.test.api.CoverageRuleBqRow")
	return schema, err
}

// ReadGenerated finds all files under dir, and parses the contents of each into
// a CoverageRuleBqRow. Every file in dir is assumed to be a JSON list of
// CoverageRule jsonprotos (note that there is no proto message representing a
// list of CoverageRules, i.e. the generated files should not actually be valid
// jsonproto, but they should be a JSON list of valid jsonproto). If any file in
// dir isn't a list of CoverageRules, an error is returned.
//
// dir must be in a git repo. This function will determine the host and project
// of the repo and the repo-relative path of each file.
func ReadGenerated(ctx context.Context, dir string) ([]*testpb.CoverageRuleBqRow, error) {
	host, project, err := git.GetRemoteHostAndProject(dir)
	if err != nil {
		return nil, err
	}

	logging.Infof(ctx, "determined dir %q has host %q and project %q", dir, host, project)

	rows := make([]*testpb.CoverageRuleBqRow, 0)
	if err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		logging.Infof(ctx, "parsing CoverageRules from %s", path)

		repoRelativePath, err := git.GetRepoRelativePath(dir, path)
		if err != nil {
			return err
		}

		bytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// First parse the file as a list of raw JSON objects, so we can use
		// protojson to unmarshal each object (which should be a CoverageRule)
		// later.
		unparsedRules := make([]json.RawMessage, 0)
		if err = json.Unmarshal(bytes, &unparsedRules); err != nil {
			return fmt.Errorf("failed to unmarshal JSON in %s: %w", path, err)
		}

		for _, unparsedRule := range unparsedRules {
			rule := &testpb.CoverageRule{}
			if err := protojson.Unmarshal(unparsedRule, rule); err != nil {
				return err
			}

			rows = append(rows, &testpb.CoverageRuleBqRow{
				Host:         host,
				Project:      project,
				Path:         repoRelativePath,
				CoverageRule: rule,
			})
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return rows, nil
}
