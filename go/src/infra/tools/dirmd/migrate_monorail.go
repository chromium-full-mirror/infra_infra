// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dirmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"go.chromium.org/luci/common/errors"

	dirmdpb "go.chromium.org/infra/tools/dirmd/proto"
)

const MonorailMissingError string = "Monorail component is undefined."

// MigrateMonorailMetadata migrates Monorail Component to a Buganizer Component ID.
// TODO(crbug.com/1505875) - Remove this once migration is complete.
func MigrateMonorailMetadata(dir string, cm map[string]int64, root string) error {
	md, err := ReadMetadata(dir, true)
	if err != nil {
		return err
	}

	if md == nil {
		return nil
	}

	mixinsToProcess, err := HandleMixins(md, cm, root)
	if err != nil {
		return err
	}
	for path, mixin := range mixinsToProcess {
		if err = writeMetadata(path, mixin); err != nil {
			return err
		}
	}

	md, err = HandleMetadata(md, cm, dir)
	if err != nil {
		return err
	}
	if err = writeMetadata(filepath.Join(dir, Filename), md); err != nil {
		return err
	}

	return nil
}

func isMonorailComponentDefined(md *dirmdpb.Metadata) bool {
	return md != nil && md.Monorail != nil && md.Monorail.Component != ""
}

func isBuganizerDefined(md *dirmdpb.Metadata) bool {
	return md != nil && (md.Buganizer != nil || md.BuganizerPublic != nil)
}

func isMonorailProjectDefined(md *dirmdpb.Metadata) bool {
	return md.GetMonorail().GetProject() != ""
}

func isMonorailProjectChromium(md *dirmdpb.Metadata) bool {
	return isMonorailProjectDefined(md) && strings.ToLower(md.Monorail.Project) == "chromium"
}

func canSkipMixin(mixin *dirmdpb.Metadata) bool {
	return isBuganizerDefined(mixin) || !isMonorailComponentDefined(mixin) || !isMonorailProjectChromium(mixin)
}

func HandleMixins(md *dirmdpb.Metadata, cm map[string]int64, root string) (map[string]*dirmdpb.Metadata, error) {
	toProcess := make(map[string]*dirmdpb.Metadata, 0)
	// Handle Mixins before dealing with the current metadata, since Monorail
	// could be inherited from the mixin.
	if md == nil || md.Mixins == nil || len(md.Mixins) <= 0 {
		return toProcess, nil
	}

	for _, mx := range md.GetMixins() {
		// Mixin paths are absolute to the project source. The root is provided as an arg.
		// filepath.Join() will clean up the path.
		// ie/ /Abs/Path/Project and //sub/path/COMMON_METADATA =
		// /Abs/Path/Project/sub/path/COMMON_METADATA
		mfp := filepath.Join(root, mx)

		mixin, err := ReadFile(mfp)
		if err != nil {
			return toProcess, err
		}

		if canSkipMixin(mixin) {
			continue
		}

		err = applyBuganizerID(mixin, cm)
		if err != nil {
			return toProcess, errors.Annotate(err, fmt.Sprintf("Mixin failure at %s", mfp)).Err()
		}

		toProcess[mfp] = mixin
	}

	return toProcess, nil
}

func HandleMetadata(md *dirmdpb.Metadata, cm map[string]int64, dir string) (*dirmdpb.Metadata, error) {
	// Skip if DIR_METADATA already has a Buganizer component defined.
	if isBuganizerDefined(md) {
		return md, nil
	}

	// Skip, but log using error if Monorail/Component metadata undefined.
	if !isMonorailComponentDefined(md) {
		return nil, errors.Reason(fmt.Sprintf("Error at %s: %s", dir, MonorailMissingError)).Err()
	}

	// Skip if Monorail project is defined and not Chromium
	if !isMonorailProjectChromium(md) {
		return md, nil
	}

	err := applyBuganizerID(md, cm)
	if err != nil {
		return nil, errors.Annotate(err, fmt.Sprintf("Failure at %s", dir)).Err()
	}

	return md, nil
}

// applyBuganizerID applies a Buganizer ComponentID to the metadata if the
// Monorail component is mapped to an ID in the provided mapping.
func applyBuganizerID(md *dirmdpb.Metadata, cm map[string]int64) error {
	// Lower case the component
	lower := strings.ToLower(md.Monorail.Component)

	val, ok := cm[lower]
	if !ok {
		// There are a handful of Monorail components listed in DIR_METADATA
		// that are non existent in both Monorail and the actual mapping.
		return errors.Reason(fmt.Sprintf("%s is missing from the provided mapping", md.Monorail.Component)).Err()
	}
	md.Buganizer = &dirmdpb.Buganizer{
		ComponentId: val,
	}

	return nil
}
