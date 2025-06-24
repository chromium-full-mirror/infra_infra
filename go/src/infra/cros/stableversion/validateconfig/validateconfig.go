// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package validateconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/golang/protobuf/jsonpb"

	"go.chromium.org/chromiumos/infra/proto/go/lab_platform"

	"go.chromium.org/infra/cros/stableversion"
)

var unmarshaller = jsonpb.Unmarshaler{AllowUnknownFields: false}

// InspectFile takes a path and determines what, if anything, is wrong with a stable_versions.cfg file.
func InspectFile(path string) (*lab_platform.StableVersions, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("file cannot be read (%s)", err.Error())
	}
	return InspectBuffer(buf)
}

// InspectBuffer takes file contents and determines what, if anything, is wrong with a stable_versions.cfg file.
func InspectBuffer(contents []byte) (*lab_platform.StableVersions, error) {
	if len(contents) == 0 {
		return nil, errors.New("file unexpectedly has length zero")
	}
	if !utf8.ValidString(string(contents)) {
		return nil, errors.New("file is not valid UTF-8")
	}
	if !isValidJSON(contents) {
		return nil, errors.New("file is not valid JSON")
	}
	sv, err := ParseStableVersions(contents)
	if err != nil {
		return nil, err
	}
	if sv == nil {
		return nil, errors.New("file is null JSON literal")
	}
	if len(sv.GetVersions()) == 0 {
		return nil, errors.New("file has no 'versions' entries")
	}
	if err := validateVersions(sv.GetVersions()); err != nil {
		return nil, err
	}
	return sv, nil
}

// isValidJSON determines whether a byte array contains valid JSON or not.
// This is used to give informative error messages if a non-JSON file is
// passed to ./stable_version2 validate-config .
func isValidJSON(contents []byte) bool {
	var sink json.RawMessage
	if err := json.Unmarshal(contents, &sink); err != nil {
		return false
	}
	return true
}

// ParseStableVersions takes a byte array and attempts to parse a stable version
// proto file out of it.
func ParseStableVersions(contents []byte) (*lab_platform.StableVersions, error) {
	var allSV lab_platform.StableVersions
	if err := unmarshaller.Unmarshal(bytes.NewReader(contents), &allSV); err != nil {
		return nil, fmt.Errorf("JSON does not conform to schema: %s", err.Error())
	}
	return &allSV, nil
}

const (
	fileShallowlyMalformedEntry = "file has bad %s position (%d): key=%s: error: %s"
	fileShallowlyDuplicateEntry = "file has duplicate version entry position (%d): key=%s"
)

func validateVersions(versions []*lab_platform.StableVersion) error {
	resMap := make(map[string]*lab_platform.StableVersion, len(versions))
	for index, v := range versions {
		key := stableversion.TargetToKey(v)
		if err := validateTarget(v.GetTarget()); err != nil {
			return fmt.Errorf(fileShallowlyMalformedEntry, "target", index, key.String(), err)
		}
		if err := validateVersion(v); err != nil {
			return fmt.Errorf(fileShallowlyMalformedEntry, "version", index, key.String(), err)
		}
		if _, ok := resMap[key.String()]; ok {
			return fmt.Errorf(fileShallowlyDuplicateEntry, index, key.String())
		}
		resMap[key.String()] = v
	}
	return nil
}

func validateTarget(t *lab_platform.StableVersionTarget) error {
	if t.GetBoard() == "" || t.GetModel() == "" {
		return fmt.Errorf("validate target: board/model is empty")
	}
	if !isLowercase(t.GetBoard()) {
		return fmt.Errorf("validate target: board %q is not lowercase", t.GetBoard())
	}
	if !isLowercase(t.GetModel()) {
		return fmt.Errorf("validate target: model %q is not lowercase", t.GetModel())
	}
	return nil
}

const (
	errorBadOSVersion           = "bad OS version: %s"
	errorBadOSPath              = "bad OS path version: %q as does not contain version %q"
	errorBadFirmwareVersion     = "bad firmware version: %s"
	errorBadFirmwarePath        = "bad firmware path: %s"
	errorUnexpectedFirmwarePath = "unexpected firmware path as version is not specified: %s"
	errorMismatchFirmwarePath   = "firmware path mismatch sub-version: %q and %q"
)

func validateVersion(v *lab_platform.StableVersion) error {
	if v.GetOsVersion() == "" || v.GetOsImagePath() == "" {
		return fmt.Errorf("validate version: os data is empty")
	}
	if stableversion.DeviceTypeForValidations[v.GetTarget().GetDeviceType()] {
		if err := stableversion.ValidateCrOSVersion(v.GetOsVersion()); err != nil {
			return fmt.Errorf(errorBadOSVersion, err.Error())
		}
		if v.GetOsImagePath() != "" {
			if !strings.Contains(v.GetOsImagePath(), v.GetOsVersion()) {
				return fmt.Errorf(errorBadOSPath, v.GetOsImagePath(), v.GetOsVersion())
			}
		}
		if v.GetFirmwareRoVersion() != "" {
			subVersion, err := stableversion.ParseFirmwareVersion(v.GetFirmwareRoVersion())
			if err != nil {
				return fmt.Errorf(errorBadFirmwareVersion, err.Error())
			} else if subVersion == "" {
				return fmt.Errorf(errorBadFirmwareVersion, "not able extract sub-version")
			}
			if path := v.GetFirmwareRoImagePath(); path != "" {
				// Custom `*/pinned_firmware.tar.bz2` path can mismatches with version.
				if !strings.HasSuffix(path, "pinned_firmware.tar.bz2") && !strings.Contains(path, subVersion) {
					return fmt.Errorf(errorMismatchFirmwarePath, subVersion, path)
				}
				if _, err := stableversion.ParseFirmwarePath(path); err != nil {
					return fmt.Errorf(errorBadFirmwarePath, err.Error())
				}
			}
		} else if v.GetFirmwareRoImagePath() != "" {
			return fmt.Errorf(errorUnexpectedFirmwarePath, v.GetFirmwareRoImagePath())
		}
	}
	return nil
}

// check if string has entirely lowercase letters
func isLowercase(s string) bool {
	for _, ch := range s {
		if unicode.IsLetter(ch) {
			if unicode.IsUpper(ch) {
				return false
			}
		}
	}
	return true
}
