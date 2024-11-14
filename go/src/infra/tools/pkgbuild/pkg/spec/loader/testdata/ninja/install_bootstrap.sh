#!/bin/bash
# Copyright 2018 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

set -e
set -x
set -o pipefail

PREFIX="$1"

if [[ $_3PP_PLATFORM == mac* ]]; then
  XCODE_SDK_PATH=$(xcrun --show-sdk-path)
  # the min version is configured in mac_sdk.gni
  # https://crrev.com/e840c4b48a861be294f206bd694ebce986ddbb88/build/config/mac/mac_sdk.gni#28
  MACOSX_VERSION_MIN="10.13"
  MACOSX_FLAGS="-isysroot${XCODE_SDK_PATH} -mmacosx-version-min=${MACOSX_VERSION_MIN}"
  CFLAGS="${CFLAGS} ${MACOSX_FLAGS}"
  LDFLAGS="${LDFLAGS} ${MACOSX_FLAGS}"
fi

CFLAGS="${CFLAGS}" LDFLAGS="${LDFLAGS}" ./configure.py --bootstrap
./ninja all
./ninja_test
if [[ $_3PP_PLATFORM == windows* ]]; then
  cp ninja.exe "$PREFIX"
else
  cp ninja "$PREFIX"
fi
