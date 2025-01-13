#!/bin/bash
# Copyright 2025 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

set -e
set -x
set -o pipefail

PREFIX="$1"

if [[ $_3PP_PLATFORM = "windows-386"  ]]; then
  cp raw_source_0.i386 $PREFIX/wimboot.i386
elif [[ $_3PP_PLATFORM = "windows-amd64"  ]]; then
  cp raw_source_0 $PREFIX/wimboot
elif [[ $_3PP_PLATFORM = "windows-arm64" ]]; then
  cp raw_source_0.arm64 $PREFIX/wimboot.arm64
else
  echo "Unsupported architecture: $_3PP_PLATFORM"
  exit 1
fi