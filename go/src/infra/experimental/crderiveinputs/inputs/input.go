// Copyright 2023 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package inputs

import (
	"fmt"

	"go.chromium.org/infra/experimental/crderiveinputs/inputpb"
)

func Unresolved(requested string) *inputpb.ResolvableString {
	return &inputpb.ResolvableString{
		Requested: requested,
	}
}

func Resolved(requested, resolved, source string) *inputpb.ResolvableString {
	return &inputpb.ResolvableString{
		Requested:        requested,
		Resolved:         resolved,
		ResolutionSource: source,
	}
}

func AddGitSource(s *inputpb.Source, URL string, version *inputpb.ResolvableString) {
	if s.Content != nil {
		panic(fmt.Sprintf("adding git repo to path with existing content: %q", s.Content))
	}

	s.Content = &inputpb.Source_Git{
		Git: &inputpb.GitCheckout{
			Repo:    URL,
			Version: version,
		},
	}
}

func AddCIPD(s *inputpb.Source, pkg *inputpb.CIPDPackage) {
	if s.Content == nil {
		s.Content = &inputpb.Source_Cipd{Cipd: &inputpb.CIPDPackages{}}
	}

	cipd, ok := s.Content.(*inputpb.Source_Cipd)
	if !ok {
		panic(fmt.Sprintf("adding cipd package to path with existing content: %q", s.Content))
	}

	cipd.Cipd.Packges = append(cipd.Cipd.Packges, pkg)
}

func AddGCSFile(s *inputpb.Source, blob *inputpb.GCSBlob) {
	if s.Content != nil {
		panic(fmt.Sprintf("adding gcs file to path with existing content: %q", s.Content))
	}

	s.Content = &inputpb.Source_GcsFile{
		GcsFile: blob,
	}
}

func AddGCSArchive(s *inputpb.Source, blob *inputpb.GCSBlob, extractSubdir string, format inputpb.GCSArchive_Format) {
	if s.Content == nil {
		s.Content = &inputpb.Source_GcsArchives{GcsArchives: &inputpb.GCSArchives{}}
	}

	archives, ok := s.Content.(*inputpb.Source_GcsArchives)
	if !ok {
		panic(fmt.Sprintf("adding gcs archive to path with existing content: %q", s.Content))
	}

	archives.GcsArchives.Archives = append(archives.GcsArchives.Archives, &inputpb.GCSArchive{
		Archive:       blob,
		ExtractSubdir: extractSubdir,
		Format:        format,
	})
}

func AddRawFile(s *inputpb.Source, contents, source string) {
	if s.Content != nil {
		panic(fmt.Sprintf("adding raw file contents to path with existing content: %q", s.Content))
	}

	s.Content = &inputpb.Source_RawFile{
		RawFile: &inputpb.RawFileContent{
			RawContent: contents,
			Source:     source,
		},
	}
}
