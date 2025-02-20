// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package bundledesc contains definition of a "bundle description".
//
// It is a JSON document that gets embedded into the output bundle. It describes
// important features of the bundle (such as the format version). It may be used
// for future and backward compatibility checks.
package bundledesc

import (
	"encoding/json"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cmd/cloudbuildhelper/fileset"
)

// Path is a path where the description is stored within the bundle.
const Path = ".cloudbuildhelper.json"

// FormatVersion is embedded into the bundle.
//
// It should be bumped whenever there are changes to the bundle structure that
// may require special handling from whatever is consuming the bundle (e.g.
// "gaedeploy" tool). Such consumers can tweak their behavior based on the
// format version they see in the bundle.
const FormatVersion = "2024.0"

// Description gets embedded into the output bundle as a JSON file.
type Description struct {
	// FormatVersion defines the format of the bundle.
	FormatVersion string `json:"format_version"`
	// GoGAEBundles is a list of Go GAE apps copied into the bundle.
	GoGAEBundles []GoGAEBundle `json:"go_gae_bundles,omitempty"`
}

// GoGAEBundle describes a Go GAE app added to the bundle.
type GoGAEBundle struct {
	// AppYAML is a path to the app YAML relative to the bundle root.
	AppYAML string `json:"app_yaml"`
}

// Modify reads the existing description, calls the callback to update it, and
// writes it back.
//
// Creates a new empty one if necessary.
func Modify(f *fileset.Set, cb func(desc *Description) error) error {
	var cur Description
	if f, ok := f.File(Path); ok {
		blob, err := f.ReadAll()
		if err != nil {
			return errors.Annotate(err, "failed to read existing %s", Path).Err()
		}
		if err := json.Unmarshal(blob, &cur); err != nil {
			return errors.Annotate(err, "bad existing %s", Path).Err()
		}
	}

	cur.FormatVersion = FormatVersion
	if err := cb(&cur); err != nil {
		return err
	}

	blob, err := json.MarshalIndent(&cur, "", "  ")
	if err != nil {
		return errors.Annotate(err, "bad updated %s", Path).Err()
	}

	if err := f.AddFromMemory(Path, blob, nil); err != nil {
		return errors.Annotate(err, "storing updated %s", Path).Err()
	}

	return nil
}
