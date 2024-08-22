// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package vpythoncommon has constants which are used across all the different
// vpython cmd packages.
package vpythoncommon

// Virtualenv38Version is the version of the CIPD package for the 'virtualenv'
// wheel when used with python3.8. This CIPD package is "infra/3pp/tools/virtualenv".
const Virtualenv38Version = "version:2@16.7.12.chromium.7"

// Virtualenv311Version is the version of the CIPD package for the 'virtualenv'
// wheel when used with python3.11. This CIPD package is "infra/3pp/tools/virtualenv".
const Virtualenv311Version = "version:2@20.25.1.chromium.8"
