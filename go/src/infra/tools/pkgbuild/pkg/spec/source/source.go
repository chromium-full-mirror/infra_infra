// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package source provides LoadSource reads, writes and updates source
// information.
package source

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/infra/tools/pkgbuild/pkg/spec"
)

// Source represents the source for the package.
// Version can be resolved or loaded from lock file.
type Source struct {
	dir     string
	creates []*spec.Spec_Create

	locked *SourceLockFile
}

func LoadSource(dir string, creates []*spec.Spec_Create) *Source {
	ret := &Source{
		dir:     dir,
		creates: creates,
		locked: &SourceLockFile{
			Infos: make(map[string]*SourceInfo),
		},
	}

	b, err := os.ReadFile(filepath.Join(dir, "src.lock"))
	if err != nil {
		return ret
	}
	if err := protojson.Unmarshal(b, ret.locked); err != nil {
		return ret
	}

	return ret
}

func (s *Source) UpdateInfo(ctx context.Context, cipdHostPlatforms []string, r Resolver) error {
	var errs []error
	for _, p := range cipdHostPlatforms {
		c, err := spec.MergeCreates(p, s.creates)
		if err != nil {
			if errors.Is(err, spec.ErrPackageNotAvailable) {
				continue
			}
			errs = append(errs, err)
			continue
		}
		info, err := r.Resolve(ctx, p, s.dir, c.Source)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		s.locked.Infos[p] = info
	}
	s.locked.LastUpdated = timestamppb.Now()

	return errors.Join(errs...)
}

func (s *Source) GetInfo(cipdHostPlatform string) *SourceInfo {
	if s == nil {
		return nil
	}
	return s.locked.Infos[cipdHostPlatform]
}

func (s *Source) PersistInfo() error {
	b, err := protojson.Marshal(s.locked)
	if err != nil {
		return err
	}

	// Json encoded by protojson is intentionally unstable.
	// Indent it for reproducibility and better readability.
	buf := bytes.NewBuffer(nil)
	if err := json.Indent(buf, b, "", "  "); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(s.dir, "src.lock"), buf.Bytes(), 0666); err != nil {
		return err
	}

	return nil
}
