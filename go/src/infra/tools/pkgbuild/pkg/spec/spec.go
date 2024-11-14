// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package spec provides 3pp spec definition and helper functions for merging
// Create field.
package spec

import (
	"errors"
	"regexp"
)

var (
	ErrPackageNotAvailable = errors.New("package not available on the target platform")
)

// MergeCreates merges create field in 3pp spec based on the host platform.
// If the package isn't supported on the host, ErrPackageNotAvailable will be
// returned.
func MergeCreates(host string, creates []*Spec_Create) (*Spec_Create, error) {
	var ret *Spec_Create

	for _, c := range creates {
		if c.GetPlatformRe() != "" {
			matched, err := regexp.MatchString(c.GetPlatformRe(), host)
			if err != nil {
				return nil, err
			}
			if !matched {
				continue
			}
		}

		if c.GetUnsupported() {
			return nil, ErrPackageNotAvailable
		}

		if ret == nil {
			ret = &Spec_Create{}
		}
		ProtoMerge(ret, c)
	}

	if ret == nil {
		return nil, ErrPackageNotAvailable
	}

	// To make this create rule self-consistent instead of just having the last
	// platform_re to be applied.
	ret.PlatformRe = ""

	return ret, nil
}
