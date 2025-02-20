// Copyright (c) 2023 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"google.golang.org/protobuf/encoding/prototext"

	"go.chromium.org/luci/cipd/client/cipd/ensure"
	"go.chromium.org/luci/cipd/client/cipd/template"
	"go.chromium.org/luci/cipd/common/cipderr"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/vpython/api/vpython"

	"go.chromium.org/infra/experimental/crderiveinputs/inputpb"
)

var pyVersionMap = map[string]ensure.PackageDef{
	"2.7": {
		PackageTemplate:   "go.chromium.org/infra/3pp/tools/cpython/${platform}",
		UnresolvedVersion: "version:2@2.7.18.chromium.46",
	},
	"3.8": {
		PackageTemplate:   "go.chromium.org/infra/3pp/tools/cpython3/${platform}",
		UnresolvedVersion: "version:2@3.8.10.chromium.30",
	},
	"3.11": {
		PackageTemplate:   "go.chromium.org/infra/3pp/tools/cpython3/${platform}",
		UnresolvedVersion: "version:2@3.11.6.chromium.30",
	},
}

func (o *Oracle) PinVpythonSpec(specPath string) error {
	LEAKY("Using hard-coded algorithm for vpython_platform expansion.")

	raw, err := o.ReadFullString(specPath)
	if err != nil {
		return err
	}

	specHash := sha256.Sum256([]byte(raw))
	specHashStr := hex.EncodeToString(specHash[:])

	// TODO - this doesn't need to hold a lock on the whole manifest, it just
	// needs to be able to generate a 'promise' for a given specHashStr so that
	// other callers into PinVpythonSpec can block on a single resolution process.

	o.manifestMu.Lock()
	defer o.manifestMu.Unlock()
	curVenv, ok := o.manifest.Virtualenvs[specHashStr]
	if ok {
		return nil
	}
	curVenv = &inputpb.VpythonEnv{ManifestSha2: specHashStr}

	spec := &vpython.Spec{}
	if err := prototext.Unmarshal([]byte(raw), spec); err != nil {
		return err
	}

	pythonVersion := spec.PythonVersion
	if pythonVersion == "" {
		if strings.HasSuffix(specPath, "3") {
			LEAKY("Assuming PythonVersion == 3.8 for %q", specPath)
			pythonVersion = "3.8"
		} else {
			LEAKY("Assuming PythonVersion == 2.7 for %q", specPath)
			pythonVersion = "2.7"
		}
	}

	interpDef, ok := pyVersionMap[pythonVersion]
	if !ok {
		TODO("Pin vpython spec %q with unknown PythonVersion %q", specPath, pythonVersion)
		return nil
	}

	expander := make(template.Expander, len(o.cipdExpander)+1)
	for k, v := range o.cipdExpander {
		expander[k] = v
	}

	pyVersion := fmt.Sprintf("cp%s", strings.ReplaceAll(pythonVersion, ".", ""))
	abi := pyVersion
	if pyVersion == "cp27" {
		LEAKY("Assuming cp27mu ABI.")
		abi += "mu"
	}

	// TODO: support py_platform, py_version, py_tag?
	expander["vpython_platform"] = fmt.Sprintf("%s_%s_%s", expander["platform"], pyVersion, abi)

	LEAKY("Using hard-coded python interpreter version.")
	if curVenv.PythonInterpreter, err = o.pinCipd(interpDef, nil, "", o.cipdExpander); err != nil {
		return err
	}

	curVenv.Wheel = make([]*inputpb.CIPDPackage, 0, len(spec.Wheel))

	for _, wheel := range spec.Wheel {
		missingFatal := true
		if wheel.MatchTag != nil || wheel.NotMatchTag != nil {
			missingFatal = false
		}
		wheelName := wheel.Name
		if badTemplate := "${platform}_${py_python}_${py_abi}"; strings.Contains(wheelName, badTemplate) {
			LEAKY("Replacing unsupported template %q in %q", badTemplate, specPath)
			wheelName = strings.ReplaceAll(wheelName, badTemplate, "${vpython_platform}")
		}
		resolved, err := o.pinCipd(ensure.PackageDef{
			PackageTemplate:   wheelName,
			UnresolvedVersion: wheel.Version,
		}, nil, "", expander)
		if err != nil {
			if !missingFatal && cipderr.ToCode(err) == cipderr.InvalidVersion {
				LEAKY("Assuming match_tag failure for vpython spec %q - %q: ignoring %q", specPath, wheel.Name, err)
				err = nil
			} else {
				return errors.Annotate(err, "processing %q", specPath).Err()
			}
		} else {
			curVenv.Wheel = append(curVenv.Wheel, resolved)
		}
	}

	if o.manifest.Virtualenvs == nil {
		o.manifest.Virtualenvs = map[string]*inputpb.VpythonEnv{}
	}
	o.manifest.Virtualenvs[specHashStr] = curVenv
	return nil
}
